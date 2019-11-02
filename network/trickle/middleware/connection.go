// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"net"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network"
)

// Connection represents a direct connection to another peer on the flow
// network.
type Connection struct {
	log      zerolog.Logger
	conn     net.Conn
	enc      network.Encoder
	dec      network.Decoder
	inbound  chan interface{}
	outbound chan interface{}
	once     *sync.Once
	done     chan struct{}
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewConnection(log zerolog.Logger, codec network.Codec, conn net.Conn) *Connection {

	log = log.With().
		Str("local_addr", conn.LocalAddr().String()).
		Str("remote_addr", conn.RemoteAddr().String()).
		Logger()

	enc := codec.NewEncoder(conn)
	dec := codec.NewDecoder(conn)

	c := Connection{
		log:      log,
		conn:     conn,
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

	// encode the message onto the connection stream
	err := c.enc.Encode(msg)
	if err != nil {
		return errors.Wrap(err, "could not encode message")
	}

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

// Process will start one background routine to handle receiving messages and
// onde background routine to handle sending messages, so that the actual
// middleware layer can function without blocking.
func (c *Connection) Process(connID string) {
	c.log = c.log.With().Str("conn_id", connID).Logger()
	go c.recv()
	go c.send()
}

// stop will stop by closing the done channel and closing the connection.
func (c *Connection) stop() {
	c.once.Do(func() {
		close(c.done)
		c.conn.Close()
	})
}

// recv must be run in a goroutine and takes care of continuously receiving
// messages from the peer connection until the connection fails.
func (c *Connection) recv() {

RecvLoop:
	for {

		// check if we should stop
		select {
		case <-c.done:
			c.log.Debug().Msg("exiting receive routine")
			break RecvLoop
		default:
		}

		// if we have a message on the connection, receive it
		msg, err := c.Receive()
		if isClosedErr(err) {
			c.log.Debug().Msg("connection closed, stopping reads")
			c.stop()
			continue
		}
		if err != nil {
			c.log.Error().Err(err).Msg("could not read data, stopping reads")
			c.stop()
			continue
		}

		// stash the received message into the inbound queue for handling
		c.inbound <- msg
	}

	// close and drain the inbound channel
	close(c.inbound)
	for range c.inbound {
	}
}

// send must be run in a goroutine and takes care of continuously sending
// messages to the peer until the message queue is closed.
func (c *Connection) send() {

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
	for range c.outbound {
	}
}
