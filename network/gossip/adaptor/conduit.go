// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package adaptor

import (
	"context"
)

// sendFunc serves as function for the sender to submit messages to the wrapper
// which will send them through GRPC.
type submitFunc func(uint8, interface{}, ...string) error

// recvFunc is a function that submits a received message to the wrapper to
// forward it to the receiver.
type handleFunc func(uint8, []byte) error

// Conduit will send messages while injecting the bound engine code.
type Conduit struct {
	code   uint8
	submit submitFunc
	handle handleFunc
}

// Submit will function as sending function for this engine.
func (c *Conduit) Submit(event interface{}, recipients ...string) error {
	return c.submit(c.code, event, recipients...)
}

// Handle functions as GRPC callback to handle payloads for this engine.
func (c *Conduit) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	return nil, c.handle(c.code, payload)
}
