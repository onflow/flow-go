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

const FlowLibP2PPingPrefix = "/flow/ping/"

const pingTimeout = time.Second * 60

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

func (ps *PingService) PingHandler(s network.Stream) {

	errCh := make(chan error, 1)
	defer close(errCh)
	timer := time.NewTimer(pingTimeout)
	defer timer.Stop()

	log := ps.logger.With().Str("remote", s.Conn().RemoteMultiaddr().String()).Str("protocol", string(s.Protocol())).Logger()

	go func() {
		select {
		case <-timer.C:
			log.Error().Msg("ping timeout")
			err := s.Reset()
			if err != nil {
				log.Error().Err(err).Msg("failed to reset stream")
			}
		case err, ok := <-errCh:
			if ok {
				log.Error().Err(err)
				err := s.Reset()
				if err != nil {
					log.Error().Err(err).Msg("failed to reset stream")
				}
				return
			}
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

	requestBytes, err := reader.ReadMsg()
	if err != nil {
		errCh <- err
		return
	}
	pingRequest := &message.PingRequest{}
	err = proto.Unmarshal(requestBytes, pingRequest)
	if err != nil {
		errCh <- err
		return
	}

	pingResponse := &message.PingResponse{
		Version: build.Semver(), // the semantic version of the build
		// TODO populate BlockHeight:
	}

	responseBytes, err := pingResponse.Marshal()
	if err != nil {
		errCh <- err
		return
	}

	err = writer.WriteMsg(responseBytes)
	if err != nil {
		errCh <- err
		return
	}
}

func (ps *PingService) Ping(ctx context.Context, p peer.ID) (message.PingResponse, time.Duration, error) {
	s, err := ps.host.NewStream(ctx, p, ps.pingProtocolID)
	if err != nil {
		return message.PingResponse{}, -1, err
	}
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

	pingRequest := &message.PingRequest{}
	requestBytes, err := proto.Marshal(pingRequest)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

	before := time.Now()
	_, err = writer.Write(requestBytes)
	if err != nil {
		return message.PingResponse{}, -1, err
	}

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
