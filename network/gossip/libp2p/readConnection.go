package libp2p

import (
	ggio "github.com/gogo/protobuf/io"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
)

type ReadConnection struct {
	*Connection
	inbound chan *message.Message
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewReadConnection(log zerolog.Logger, stream libp2pnetwork.Stream) *ReadConnection {
	connection := NewConnection(log, stream)
	c := ReadConnection{
		Connection: connection,
		inbound:    make(chan *message.Message),
	}
	return &c
}

// recv must be run in a goroutine and takes care of continuously receiving
// messages from the peer connection until the connection fails.
func (rc *ReadConnection) ReceiveLoop() {
	// close and drain the inbound channel
	defer close(rc.inbound)
	r := ggio.NewDelimitedReader(rc.stream, 1<<20)
RecvLoop:
	for {
		// check if we should stop
		select {
		case <-rc.done:
			rc.log.Debug().Msg("exiting receive routine")
			break RecvLoop
		default:
		}

		var msg message.Message
		err := r.ReadMsg(&msg)
		if err != nil {
			rc.log.Error().Str("peer", rc.stream.Conn().RemotePeer().String()).Err(err)
			rc.stop()
			return
		}

		rc.log.Debug().Str("peer", rc.stream.Conn().RemotePeer().String()).
			Hex("sender", msg.OriginID).
			Hex("eventID", msg.EventID).
			Msg("received message")

		// stash the received message into the inbound queue for handling
		rc.inbound <- &msg
	}
}
