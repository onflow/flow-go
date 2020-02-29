package libp2p

import (
	"io"

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
		// error handling done similar to comm.go in pubsub (as suggested by libp2p folks)
		if err != nil {
			// if the sender closes the connection an EOF is received otherwise an actual error is received
			if err != io.EOF {
				rc.log.Error().Err(err)
				err = rc.stream.Reset()
				if err != nil {
					rc.log.Error().Err(err)
				}
			} else {
				err = rc.stream.Close()
				if err != nil {
					rc.log.Error().Err(err)
				}
			}
			rc.stop()
			return
		}

		// stash the received message into the inbound queue for handling
		rc.inbound <- &msg
	}
}
