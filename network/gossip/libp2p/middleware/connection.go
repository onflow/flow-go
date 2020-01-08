// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"io/ioutil"
	"sync"

	"github.com/dapperlabs/flow-go/network"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
)

// Connection represents a direct connection to another peer on the flow
// network.
type Connection struct {
	log      zerolog.Logger
	stream   libp2pnetwork.Stream
	enc      network.Encoder
	dec      network.Decoder
	inbound  chan interface{}
	outbound chan interface{}
	once     *sync.Once
	done     chan struct{}
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewConnection(log zerolog.Logger, codec network.Codec, stream libp2pnetwork.Stream) *Connection {

	log = log.With().
		Str("local_addr", stream.Conn().LocalPeer().String()).
		Str("remote_addr", stream.Conn().RemotePeer().String()).
		Logger()

	enc := codec.NewEncoder(stream)
	dec := codec.NewDecoder(stream)

	c := Connection{
		log:      log,
		stream:   stream,
		enc:      enc,
		dec:      dec,
		inbound:  make(chan interface{}),
		outbound: make(chan interface{}),
		once:     &sync.Once{},
		done:     make(chan struct{}),
	}

	return &c
}

// Send will encode the given opaque message using the injected encoder and
// write it to this peer's network connection.
func (c *Connection) Send(msg interface{}) error {
	// Send the message using the stream
	b := msg.([]byte)
	_, err := c.stream.Write(b)
	if err != nil {
		return err
	}
	// Debug log the message length
	c.log.Debug().Str("peer", c.stream.Conn().RemotePeer().String()).
		Str("message", string(b)).Int("length", len(b)).
		Msg("sent message")

	c.stream.Close()
	return nil
}

// Receive will use the injected decoder to read the next opaque message from
// this peer's network connection.
func (c *Connection) Receive() (interface{}, error) {

	// decode the message from the connection stream
	msg, err := c.dec.Decode()
	if err != nil {
		return nil, errors.Wrap(err, "could not decode message")
	}

	return msg, nil
}


// stop will stop by closing the done channel and closing the connection.
func (c *Connection) stop() {
	c.once.Do(func() {
		close(c.done)
		c.stream.Close()
	})
}

// recv must be run in a goroutine and takes care of continuously receiving
// messages from the peer connection until the connection fails.
func (c *Connection) ReceiveLoop() {
fmt.Println("starting recv")
RecvLoop:
	for {
		// check if we should stop
		select {
		case <-c.done:
			c.log.Debug().Msg("exiting receive routine")
			break RecvLoop
		default:
		}

			// TODO: implement length-prefix framing to delineate protobuf message if exchanging more than one message (Issue#1969)
			// (protobuf has no inherent delimiter)
			// Read incoming data into a buffer
			fmt.Println("calling readall")
			buff, err := ioutil.ReadAll(c.stream)
			if err != nil {
				c.log.Error().Str("peer", c.stream.Conn().RemotePeer().String()).Err(err)
				c.stream.Close()
				return
			}
		     fmt.Printf(" Got buff %d\n", len(buff))
			// ioutil.ReadAll continues to read even after an EOF is encountered.
			// Close connection and return in that case (This is not an error)
			if len(buff) <= 0 {
				c.stream.Close()
				return
			}
			c.log.Debug().Str("peer", c.stream.Conn().RemotePeer().
				String()).Bytes("message", buff).Int("length", len(buff)).
				Msg("received message")

			fmt.Printf(" pushing to channel ")


		// stash the received message into the inbound queue for handling
		c.inbound <- buff
	}

	// close and drain the inbound channel
	close(c.inbound)
}

// send must be run in a goroutine and takes care of continuously sending
// messages to the peer until the message queue is closed.
func (c *Connection) SendLoop() {

SendLoop:
	for {
		select {

		// check if we should stop
		case <-c.done:
			c.log.Debug().Msg("exiting send routine")
			break SendLoop

			// if we have a message in the outbound queue, write it to the connection
		case msg := <-c.outbound:
			err := c.Send(msg)
			if isClosedErr(err) {
				c.log.Debug().Msg("connection closed, stopping writes")
				c.stop()
				continue
			}
			if err != nil {
				c.log.Error().Err(err).Msg("could not send data, stopping writes")
				c.stop()
				continue
			}
		}
	}

	// close and drain outbound channel
	close(c.outbound)
}
