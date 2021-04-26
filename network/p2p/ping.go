package p2p

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-msgio"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/bootstrap/build"
	"github.com/onflow/flow-go/network/message"
)

// the Flow Ping protocol prefix
const FlowLibP2PPingPrefix = "/flow/ping/"

const pingTimeout = time.Second * 60

// PingService handles the outbound and inbound ping requests and response
type PingService struct {
	host           host.Host
	logger         zerolog.Logger
	pingProtocolID protocol.ID
}

func NewPingService(h host.Host, pingProtocolID protocol.ID, logger zerolog.Logger) *PingService {
	ps := &PingService{host: h, logger: logger, pingProtocolID: pingProtocolID}
	h.SetStreamHandler(pingProtocolID, ps.PingHandler)
	return ps
}

// PingHandler receives the inbound stream for Flow ping protocol and respond back with the PingResponse message
func (ps *PingService) PingHandler(s network.Stream) {

	errCh := make(chan error, 1)
	defer close(errCh)
	timer := time.NewTimer(pingTimeout)
	defer timer.Stop()

	log := ps.logger.With().Str("remote", s.Conn().RemoteMultiaddr().String()).Str("protocol", string(s.Protocol())).Logger()

	go func() {
		select {
		case <-timer.C:
			// if read or write took longer than configured timeout, then reset the stream
			log.Error().Msg("ping timeout")
			err := s.Reset()
			if err != nil {
				log.Error().Err(err).Msg("failed to reset stream")
			}
		case err, ok := <-errCh:
			// if and error occur while reposning to a pind request, then reset the stream
			if ok {
				log.Error().Err(err)
				err := s.Reset()
				if err != nil {
					log.Error().Err(err).Msg("failed to reset stream")
				}
				return
			}
			// if no error, then close the stream
			err = s.Close()
			if err != nil {
				log.Error().Err(err).Msg("failed to close stream")
			}
		}
	}()

	// create the reader
	reader := msgio.NewReader(s)
	// create the writer
	writer := msgio.NewWriter(s)

	// read request bytes
	requestBytes, err := reader.ReadMsg()
	if err != nil {
		errCh <- err
		return
	}

	// unmarshal bytes to PingRequest
	pingRequest := &message.PingRequest{}
	err = proto.Unmarshal(requestBytes, pingRequest)
	if err != nil {
		errCh <- err
		return
	}

	// create a PingResponse
	pingResponse := &message.PingResponse{
		Version: build.Semver(), // the semantic version of the build
		// TODO populate BlockHeight:
	}

	// marshal the response to bytes
	responseBytes, err := pingResponse.Marshal()
	if err != nil {
		errCh <- err
		return
	}

	// send the response to the remote node.
	err = writer.WriteMsg(responseBytes)
	if err != nil {
		errCh <- err
		return
	}
}

// Ping sends a Ping request to the remote node and returns the response, rtt and error if any.
func (ps *PingService) Ping(ctx context.Context, p peer.ID) (message.PingResponse, time.Duration, error) {

	// create a new stream to the remote node
	s, err := ps.host.NewStream(ctx, p, ps.pingProtocolID)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	// reset stream on completion
	defer func() {
		err := s.Reset()
		ps.logger.Error().
			Err(err).
			Str("remote", s.Conn().RemoteMultiaddr().String()).
			Str("protocol", string(s.Protocol())).
			Msg("failed to reset stream")
	}()

	// create the reader
	reader := msgio.NewReader(s)
	// create the writer
	writer := msgio.NewWriter(s)

	// create a ping request
	pingRequest := &message.PingRequest{}

	// marshal request to bytes slice
	requestBytes, err := proto.Marshal(pingRequest)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	// record start of ping
	before := time.Now()

	// send ping request bytes
	_, err = writer.Write(requestBytes)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	// read the ping response from the remote node
	responseBytes, err := reader.ReadMsg()
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	pingResponse := &message.PingResponse{}
	err = proto.Unmarshal(responseBytes, pingResponse)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	rtt := time.Since(before)
	// No error, record the RTT.
	ps.host.Peerstore().RecordLatency(p, rtt)

	return *pingResponse, rtt, nil
}
