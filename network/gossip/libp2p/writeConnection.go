// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"bufio"
	"fmt"
	"sync"

	ggio "github.com/gogo/protobuf/io"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
)

// Connection represents a direct connection to another peer on the flow
// network.
type WriteConnection struct {
	*Connection
	*sync.Once                       // used to close write connection once
	outbound   chan *message.Message // the channel to consumed by this writer
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewWriteConnection(log zerolog.Logger, stream libp2pnetwork.Stream) *WriteConnection {

	c := NewConnection(log, stream)
	wc := WriteConnection{
		Connection: c,
		outbound:   make(chan *message.Message),
		Once:       &sync.Once{},
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
			// connection stops
			wc.log.Debug().Msg("exiting send routine: connection stops")
			break SendLoop

			// if we have a message in the outbound queue, write it to the connection
		case msg, ok := <-wc.outbound:
			if !ok {
				// oubound channel has been closed
				return
			}
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
				wc.stop()
				continue
			}
			if err != nil {
				wc.log.Error().Err(err).Msg("could not send data, stopping writes")
				wc.stop()
				continue
			}
		}
	}
}

func (wc *WriteConnection) Stop() {
	wc.Do(func() {
		close(wc.outbound)
		wc.Connection.stop()
	})
}
