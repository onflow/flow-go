package stub

import (
	"context"
	"errors"
)

type Connection struct {
	closed        bool
	address       string
	underlay      *Underlay
	closeCallback func()
}

func NewConnection(address string, underlay *Underlay) *Connection {
	return &Connection{
		closed:   false,
		address:  address,
		underlay: underlay,
	}
}

// OnClose stores the callback
func (c *Connection) OnClose(callback func()) error {
	if c.closeCallback != nil {
		return errors.New("OnClose callback can only be set once")
	}
	c.closeCallback = callback
	return nil
}

// Close calls OnClose callback
func (c *Connection) Close() error {
	if c.closed {
		return errors.New("already closed")
	}
	c.closed = true
	c.closeCallback()
	return nil
}

// Send mocks that the peer receives the message
func (c *Connection) Send(ctx context.Context, msg []byte) error {
	if c.closed {
		return errors.New("can not send message through closed connection")
	}
	var received chan interface{}
	go func() {
		defer close(received)
		c.underlay.receive(c.address, msg)
	}()
	select {
	case <-ctx.Done():
	case <-received:
	}
	return nil
}
