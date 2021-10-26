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

// WaitError waits for either an error on the error channel or for the channel to be signalled/closed
// Returns an error from the error channel if one was received, otherwise it returns nil
//
// Without the additional select, there is a race condition here where the done channel
// could be closed right after an irrecoverable error is thrown, so that when the scheduler
// yields control back to this goroutine, both channels are available to read from. If this
// second case happens to be chosen at random to proceed, then we would return and silently
// ignore the error.
func WaitError(errChan <-chan error, done <-chan struct{}) error {
	select {
	case err := <-errChan:
		return err
	case <-done:
		select {
		case err := <-errChan:
			return err
		default:
		}
	}
	return nil
}

// WrapSignal wraps a os.Signal channel with a struct{} channel, and closes the channel when a
// signal is received.
//
// This is intended to make signals compatible with the other util channel methods.
func WrapSignal(signals ...os.Signal) <-chan struct{} {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, signals...)

	done := make(chan struct{})
	go func() {
		<-sig
		close(done)

		// cleanup since we can't use this channel again
		signal.Stop(sig)
		close(sig)
	}()
	return done
}
