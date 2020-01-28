// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"bufio"
	"fmt"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/rs/zerolog"

	ggio "github.com/gogo/protobuf/io"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
)

// Connection represents a direct connection to another peer on the flow
// network.
type WriteConnection struct {
	*Connection
	outbound chan *message.Message
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewWriteConnection(log zerolog.Logger, stream libp2pnetwork.Stream) *WriteConnection {

	c := NewConnection(log, stream)
	wc := WriteConnection{
		Connection: c,
		outbound:   make(chan *message.Message),
	}
	return &wc
}

// send must be run in a goroutine and takes care of continuously sending
// messages to the peer until the message queue is closed.
func (wc *WriteConnection) SendLoop() {

SendLoop:
	for {
		select {

		// check if we should stop
		case <-wc.done:
			wc.log.Debug().Msg("exiting send routine")
			break SendLoop

			// if we have a message in the outbound queue, write it to the connection
		case msg := <-wc.outbound:
			bufw := bufio.NewWriter(wc.stream)
			writer := ggio.NewDelimitedWriter(bufw)

			err := writer.WriteMsg(msg)
			if err != nil {
				fmt.Println(err)
			}

			bufw.Flush()

			wc.log.Debug().
				Bytes("sender", msg.OriginID).
				Hex("eventID", msg.EventID).
				Msg("sent message")

			if isClosedErr(err) {
				wc.log.Error().Err(err).Msg("connection closed, stopping writes")
				wc.Stop()
				continue
			}
			if err != nil {
				wc.log.Error().Err(err).Msg("could not send data, stopping writes")
				wc.Stop()
				continue
			}
		}
	}

	// close and drain outbound channel
	close(wc.outbound)
}
