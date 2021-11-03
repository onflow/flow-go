package util

import (
	"context"
	"errors"
	"os"
	"time"
)

var ErrChannelClosed = errors.New("channel closed")
var ErrSignalReceived = errors.New("signal received")

// WithDone wraps a signal channel with a context, and cancels the context when the channel is closed.
// When the context is Done, the ctx.Err() will either be ErrChannelClosed if the channel closed first,
// or the error from the underlying context (Canceled, DeadlineExceeded, etc).
func WithDone(parent context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &doneCtx{cancelCtx: ctx}
	go func() {
		select {
		case <-done:
			c.err = ErrChannelClosed
		case <-ctx.Done():
		}
		cancel()
	}()
	return ctx, cancel
}

type doneCtx struct {
	cancelCtx context.Context
	err       error
}

func (d *doneCtx) Done() <-chan struct{} {
	return d.cancelCtx.Done()
}

func (d *doneCtx) Err() error {
	if d.err != nil {
		return d.err
	}
	return d.cancelCtx.Err()
}

func (d *doneCtx) Value(key interface{}) interface{} {
	return d.cancelCtx.Value(key)
}

func (d *doneCtx) Deadline() (deadline time.Time, ok bool) {
	return d.cancelCtx.Deadline()
}

// WithSignal wraps an os.Signal channel with a context, and calls the cancel a signal is received.
// When the context is Done, the ctx.Err() will either be ErrSignalReceived if a signal was received,
// or the error from the underlying context (Canceled, DeadlineExceeded, etc).
//
// Note that the context can only be cancelled once and this can only communicate a single signal,
// even though the channel can receive multiple. This is consistent with context paradigm of closing
// the done channel on cancel. If you need to check for multiple signals, simply create a new context
// using WithSignal and the same signal channel, and the new context will handle the next signal on the
// channel or received later.
// The specific signal received is available from the Signal() method.
func WithSignal(parent context.Context, signalChan <-chan os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &signalCtx{cancelCtx: ctx}
	go func() {
		select {
		case sig := <-signalChan:
			c.err = ErrSignalReceived
			c.signal = sig
		case <-ctx.Done():
		}
		cancel()
	}()
	return c, cancel
}

type signalCtx struct {
	cancelCtx context.Context
	err       error
	signal    os.Signal
}

func (s *signalCtx) Done() <-chan struct{} {
	return s.cancelCtx.Done()
}

func (s *signalCtx) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.cancelCtx.Err()
}

func (s *signalCtx) Signal() os.Signal {
	return s.signal
}

func (s *signalCtx) Value(key interface{}) interface{} {
	return s.cancelCtx.Value(key)
}

func (s *signalCtx) Deadline() (deadline time.Time, ok bool) {
	return s.cancelCtx.Deadline()
}
