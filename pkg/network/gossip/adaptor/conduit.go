// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package adaptor

import (
	"context"
)

// SendFunc serves as function for the sender to submit messages to the wrapper
// which will send them through GRPC.
type sendFunc func(uint8, interface{}, ...string) error

// RecvFunc is a function that submits a received message to the wrapper to
// forward it to the receiver.
type recvFunc func(uint8, []byte) error

// Conduit will send messages while injecting the bound engine code.
type Conduit struct {
	code uint8
	send sendFunc
	recv recvFunc
}

// Send will function as sending function for this engine.
func (c *Conduit) Send(event interface{}, recipients ...string) error {
	return c.send(c.code, event, recipients...)
}

// Handle functions as GRPC callback to handle payloads for this engine.
func (c *Conduit) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	return nil, c.recv(c.code, payload)
}
