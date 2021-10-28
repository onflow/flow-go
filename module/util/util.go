package util

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/onflow/flow-go/module"
)

// AllReady calls Ready on all input components and returns a channel that is
// closed when all input components are ready.
func AllReady(components ...module.ReadyDoneAware) <-chan struct{} {
	ready := make(chan struct{})
	var wg sync.WaitGroup

	for _, component := range components {
		wg.Add(1)
		go func(c module.ReadyDoneAware) {
			<-c.Ready()
			wg.Done()
		}(component)
	}

	go func() {
		wg.Wait()
		close(ready)
	}()

	return ready
}

// AllDone calls Done on all input components and returns a channel that is
// closed when all input components are done.
func AllDone(components ...module.ReadyDoneAware) <-chan struct{} {
	done := make(chan struct{})
	var wg sync.WaitGroup

	for _, component := range components {
		wg.Add(1)
		go func(c module.ReadyDoneAware) {
			<-c.Done()
			wg.Done()
		}(component)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	return done
}

// WaitReady waits for either a signal/close on the ready channel or for the context to be cancelled
// Returns nil if the channel was signalled/closed before returning, otherwise, it returns the context
// error.
//
// This handles the corner case where the context is cancelled at the exact same time that components
// were marked ready, and reduces boilerplate code.
// This is intended for situations where ignoring a ready signal can cause safety issues.
func WaitReady(ctx context.Context, ready <-chan struct{}) error {
	select {
	case <-ctx.Done():
		select {
		case <-ready:
			return nil
		default:
		}
		return ctx.Err()
	case <-ready:
		return nil
	}
}

// CheckClosed checks if the provided channel has a signal or was closed.
// Returns true if the channel was signaled/closed, otherwise, returns false.
//
// This is intended to reduce boilerplate code when multiple channel checks are required because
// missed signals could cause safety issues.
func CheckClosed(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// WaitError waits for an error on the error channel, the provided context to be cancelled, or
// the provided channel to be closed.
// Returns nil if the done channel is close, otherwise, it returns an error.
//
// This handles a race condition between the error signaller and component's done channel described
// in detail below
func WaitError(ctx context.Context, errChan <-chan error) error {
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		// Without this additional select, there is a race condition here where the done channel
		// could have been closed as a result of an irrecoverable error being thrown, so that when
		// the scheduler yields control back to this goroutine, both channels are available to read
		// from. If this last case happens to be chosen at random to proceed instead of the one
		// above, then we would return as if the component shutdown gracefully, when in fact it
		// encountered an irrecoverable error.
		select {
		case err := <-errChan:
			return err
		default:
		}
		return nil
	}
}

var ErrChannelClosed = errors.New("channel closed")
var ErrSignalReceived = errors.New("signal received")

// WithDone wraps a done channel with a context, and calls the cancel when the done channel is closed.
func WithDone(parent context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &doneCtx{cancelCtx: ctx}
	go func() {
		select {
		case <-done:
			cancel()
			c.err = ErrChannelClosed
		case <-ctx.Done():
			c.err = ctx.Err()
		}
	}()
	return ctx, cancel
}

// TODO: we need to be able to receive multiple signals
// WithSignal wraps an os.Signal channel with a context, and calls the cancel a signal is received.
// TODO: should this go somewhere else? maybe under module
func WithSignals(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)

	ctx, cancel := context.WithCancel(parent)
	c := &signalCtx{cancelCtx: ctx}
	go func() {
		select {
		case <-signalChan:
			cancel()
			c.err = ErrSignalReceived
		case <-ctx.Done():
			c.err = ctx.Err()
		}

		// cleanup since we can't use this channel again
		signal.Stop(signalChan)
		close(signalChan)
	}()

	return c, cancel
}

type signalCtx struct {
	cancelCtx context.Context
	err       error
}

func (s *signalCtx) Done() <-chan struct{} {
	return s.cancelCtx.Done()
}

func (s *signalCtx) Err() error {
	return s.err
}

func (s *signalCtx) Value(key interface{}) interface{} {
	return s.cancelCtx.Value(key)
}

func (s *signalCtx) Deadline() (deadline time.Time, ok bool) {
	return s.cancelCtx.Deadline()
}

type doneCtx struct {
	cancelCtx context.Context
	err       error
}

func (s *doneCtx) Done() <-chan struct{} {
	return s.cancelCtx.Done()
}

func (s *doneCtx) Err() error {
	return s.err
}

func (s *doneCtx) Value(key interface{}) interface{} {
	return s.cancelCtx.Value(key)
}

func (s *doneCtx) Deadline() (deadline time.Time, ok bool) {
	return s.cancelCtx.Deadline()
}
