// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"io"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/pkg/codec"
)

// Connection represents a direct connection to another peer on the flow
// network.
type Connection struct {
	log      zerolog.Logger
	enc      codec.Encoder
	dec      codec.Decoder
	inbound  chan interface{}
	outbound chan interface{}
	done     chan struct{}
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewConnection(log zerolog.Logger, enc codec.Encoder, dec codec.Decoder) *Connection {

	c := Connection{
		log:      log,
		enc:      enc,
		dec:      dec,
		inbound:  make(chan interface{}),
		outbound: make(chan interface{}),
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
func (c *Connection) Process() {
	go c.recv()
	go c.send()
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
		if errors.Cause(err) == io.EOF {
			c.log.Debug().Msg("connection closed remotely, stopping reads")
			close(c.done)
			continue
		}
		if err != nil {
			c.log.Error().Err(err).Msg("could not read data, stopping reads")
			close(c.done)
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
			if err != nil {
				c.log.Error().Err(err).Msg("could not send message")
				close(c.done)
			}
		}
	}

	// close and drain outbound channel
	close(c.outbound)
	for range c.outbound {
	}
}
