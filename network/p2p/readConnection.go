package p2p

import (
	"context"
	"io"
	"sync"

	ggio "github.com/gogo/protobuf/io"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/message"
)

// readConnection reads the incoming stream and calls the callback until the remote closes the stream or the context is
// cancelled
type readConnection struct {
	ctx        context.Context
	stream     libp2pnetwork.Stream
	log        zerolog.Logger
	metrics    module.NetworkMetrics
	maxMsgSize int
	callback   func(msg *message.Message)
}

// newReadConnection creates a new readConnection
func newReadConnection(ctx context.Context,
	stream libp2pnetwork.Stream,
	callback func(msg *message.Message),
	log zerolog.Logger,
	metrics module.NetworkMetrics,
	maxMsgSize int) *readConnection {

	if maxMsgSize <= 0 {
		maxMsgSize = DefaultMaxUnicastMsgSize
	}

	streamLogger := streamLogger(log, stream)

	c := readConnection{
		ctx:        ctx,
		stream:     stream,
		callback:   callback,
		log:        streamLogger,
		metrics:    metrics,
		maxMsgSize: maxMsgSize,
	}
	return &c
}

// receiveLoop must be run in a goroutine and it continuously reads messages from the peer until
// either the remote closes the stream or the context is cancelled
func (rc *readConnection) receiveLoop(wg *sync.WaitGroup) {

	defer wg.Done()
	defer rc.log.Trace().Msg("exiting receive routine")

	// create the reader
	r := ggio.NewDelimitedReader(rc.stream, rc.maxMsgSize)

	for {
		// check if we should stop
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		var msg message.Message
		// read the next message (blocking call)
		err := r.ReadMsg(&msg)

		// error handling done similar to comm.go in pubsub (as suggested by libp2p folks)
		if err != nil {
			// if the sender closes the connection, an EOF is received
			if err == io.EOF {
				rc.closeStream()
				return
			}
			rc.log.Error().Err(err)
			rc.resetStream()
			return
		}

		// check message size
		maxSize := unicastMaxMsgSize(&msg)
		if msg.Size() > maxSize {
			// if message size exceeded, reset stream and log error
			rc.resetStream()
			rc.log.Error().
				Hex("sender", msg.OriginID).
				Hex("event_id", msg.EventID).
				Str("event_type", msg.Type).
				Str("channel", msg.ChannelID).
				Int("maxSize", maxSize).
				Msg("received message exceeded permissible message maxSize")
			return
		}

		// log metrics with the channel name as OneToOne
		rc.metrics.NetworkMessageReceived(msg.Size(), metrics.ChannelOneToOne, msg.Type)

		// call the callback
		rc.callback(&msg)
	}
}

func (rc *readConnection) closeStream() {
	err := rc.stream.Close()
	if err != nil {
		rc.log.Error().Err(err).Msg("failed to close stream")
	}
}

func (rc *readConnection) resetStream() {
	err := rc.stream.Reset()
	if err != nil {
		rc.log.Error().Err(err).Msg("failed to reset stream")
	}
}

func streamLogger(log zerolog.Logger, stream libp2pnetwork.Stream) zerolog.Logger {
	logger := log.With().
		Str("remote_peer", stream.Conn().RemotePeer().String()).
		Str("remote_address", stream.Conn().RemoteMultiaddr().String()).
		Str("local_peer", stream.Conn().LocalPeer().String()).
		Str("local_address", stream.Conn().LocalMultiaddr().String()).Logger()
	return logger
}
