package util

import (
	"context"
	"errors"
)

// ErrChannelClosed is returned from Err() when a context returned from WithDone is closed after
// the provided channel is closed.
var ErrChannelClosed = errors.New("channel closed")

// WithDone wraps a signal channel with a context, and cancels the context when the channel is closed.
// When the context is Done, the ctx.Err() will either be ErrChannelClosed if the channel closed first,
// or the error from the underlying context (Canceled, DeadlineExceeded, etc).
func WithDone(parent context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &doneCtx{Context: ctx}
	go func() {
		select {
		case <-done:
			cancel()
			c.err = ErrChannelClosed
		case <-ctx.Done():
		}
	}()
	return c, cancel
}

type doneCtx struct {
	context.Context
	err error
}

func (c *doneCtx) Err() error {
	if c.err != nil {
		return c.err
	}
	return c.Context.Err()
}
