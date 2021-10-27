package util

import (
	"context"
	"os"
	"os/signal"
	"sync"

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
// the provided channel to be signalled/closed.
// Returns nil if the done channel is signalled/close, otherwise, it returns an error.
//
// This handles a race condition between the error signaller and component's done channel described
// in detail below
func WaitError(ctx context.Context, errChan <-chan error, done <-chan struct{}) error {
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		// capture errors that were thrown concurrently with the context being cancelled
		select {
		case err := <-errChan:
			return err
		default:
		}
		return ctx.Err()
	case <-done:
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

		// Similarly, the done channel could have closed as a result of the context being canceled.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// WithSignal wraps a os.Signal channel with a context, and calls the cancel a signal is received.
// TODO: should this go somewhere else? maybe under module
func WithSignal(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, signals...)

	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-sig:
		}
		cancel()

		// cleanup since we can't use this channel again
		signal.Stop(sig)
		close(sig)
	}()
	return ctx, cancel
}
