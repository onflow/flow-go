package p2p

import (
	"bufio"
	"context"
	"fmt"
	"time"

	ggio "github.com/gogo/protobuf/io"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/message"
)

// the Flow Ping protocol prefix
const FlowLibP2PPingPrefix = "/flow/ping/"

const maxPingMessageSize = 5 * kb

const pingTimeout = time.Second * 60

// PingService handles the outbound and inbound ping requests and response
type PingService struct {
	host           host.Host
	pingProtocolID protocol.ID
	version        string
	logger         zerolog.Logger
}

func NewPingService(h host.Host, pingProtocolID protocol.ID, version string, logger zerolog.Logger) *PingService {
	ps := &PingService{host: h, pingProtocolID: pingProtocolID, version: version, logger: logger}
	h.SetStreamHandler(pingProtocolID, ps.PingHandler)
	return ps
}

// PingHandler receives the inbound stream for Flow ping protocol and respond back with the PingResponse message
func (ps *PingService) PingHandler(s network.Stream) {

	errCh := make(chan error, 1)
	defer close(errCh)
	timer := time.NewTimer(pingTimeout)
	defer timer.Stop()

	go func() {
		log := ps.streamLogger(s)
		select {
		case <-timer.C:
			// if read or write took longer than configured timeout, then reset the stream
			log.Error().Msg("ping timeout")
			err := s.Reset()
			if err != nil {
				log.Error().Err(err).Msg("failed to reset stream")
			}
		case err, ok := <-errCh:
			// reset the stream if an error occur while responding to a ping request
			if ok {
				log.Error().Err(err).Msg("ping response failed")
				err := s.Reset()
				if err != nil {
					log.Error().Err(err).Msg("failed to reset stream")
				}
				return
			}
			// if no error, then just close the stream
			err = s.Close()
			if err != nil {
				log.Error().Err(err).Msg("failed to close stream")
			}
		}
	}()

	pingRequest := &message.PingRequest{}
	// create the reader
	reader := ggio.NewDelimitedReader(s, maxPingMessageSize)

	// read the request
	err := reader.ReadMsg(pingRequest)
	if err != nil {
		errCh <- err
		return
	}

	// create the writer
	bufw := bufio.NewWriter(s)
	writer := ggio.NewDelimitedWriter(bufw)

	// create a PingResponse
	pingResponse := &message.PingResponse{
		Version: ps.version, // the semantic version of the build
		// TODO populate BlockHeight:
	}

	// send the PingResponse
	err = writer.WriteMsg(pingResponse)
	if err != nil {
		errCh <- err
		return
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		errCh <- err
		return
	}
}

// Ping sends a Ping request to the remote node and returns the response, rtt and error if any.
func (ps *PingService) Ping(ctx context.Context, p peer.ID) (message.PingResponse, time.Duration, error) {

	// create a done channel to indicate Ping request-response is done or an error has occurred
	done := make(chan error, 1)
	defer close(done)

	timer := time.NewTimer(PingTimeoutSecs)
	defer timer.Stop()

	// create a new stream to the remote node
	s, err := ps.host.NewStream(ctx, p, ps.pingProtocolID)
	if err != nil {
		return message.PingResponse{}, -1, fmt.Errorf("failed to create stream: %w", err)
	}

	// if ping succeeded, close the stream else reset the stream
	go func() {
		log := ps.streamLogger(s)
		select {
		case <-timer.C:
			// time expired without a response, log an error and reset the stream
			log.Error().Msg("context timed out on ping to remote node")
			// reset the stream (to cause an error on the remote side as well)
			err := s.Reset()
			if err != nil {
				log.Err(err).Msg("failed to reset stream")
			}
		case <-done:
			// close the stream
			err := s.Close()
			if err != nil {
				log.Err(err).Msg("failed to close stream")
			}
		}
	}()

	// create the writer
	bufw := bufio.NewWriter(s)
	writer := ggio.NewDelimitedWriter(bufw)

	// create a ping request
	pingRequest := &message.PingRequest{}

	// record start of ping
	before := time.Now()

	// send the request
	err = writer.WriteMsg(pingRequest)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	pingResponse := &message.PingResponse{}

	// create the reader
	reader := ggio.NewDelimitedReader(s, maxPingMessageSize)

	// read the ping response
	err = reader.ReadMsg(pingResponse)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	rtt := time.Since(before)
	// No error, record the RTT.
	ps.host.Peerstore().RecordLatency(p, rtt)

	return *pingResponse, rtt, nil
}

func (ps *PingService) streamLogger(s network.Stream) zerolog.Logger {
	return ps.logger.With().
		Str("remote", s.Conn().RemoteMultiaddr().String()).
		Str("protocol", string(s.Protocol())).
		Logger()
}
