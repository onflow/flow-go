package util

import (
	"context"
	"reflect"
	"sync"

	"github.com/onflow/flow-go/module"
)

// AllReady calls Ready on all input components and returns a channel that is
// closed when all input components are ready.
func AllReady(components ...module.ReadyDoneAware) <-chan struct{} {
	readyChans := make([]<-chan struct{}, len(components))

	for i, c := range components {
		readyChans[i] = c.Ready()
	}

	return AllClosed(readyChans...)
}

// AllDone calls Done on all input components and returns a channel that is
// closed when all input components are done.
func AllDone(components ...module.ReadyDoneAware) <-chan struct{} {
	doneChans := make([]<-chan struct{}, len(components))

	for i, c := range components {
		doneChans[i] = c.Done()
	}

	return AllClosed(doneChans...)
}

// AllClosed returns a channel that is closed when all input channels are closed.
func AllClosed(channels ...<-chan struct{}) <-chan struct{} {
	done := make(chan struct{})
	var wg sync.WaitGroup

	for _, ch := range channels {
		wg.Add(1)
		go func(ch <-chan struct{}) {
			<-ch
			wg.Done()
		}(ch)
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
// This handles the corner case where the context is cancelled at the same time that components
// were marked ready, and the Done case was selected.
// This is intended for situations where ignoring a signal can cause safety issues.
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

// MergeChannels merges a list of channels into a single channel
func MergeChannels(channels interface{}) interface{} {
	sliceType := reflect.TypeOf(channels)
	if sliceType.Kind() != reflect.Slice && sliceType.Kind() != reflect.Array {
		panic("argument must be an array or slice")
	}
	chanType := sliceType.Elem()
	if chanType.ChanDir() == reflect.SendDir {
		panic("channels cannot be send-only")
	}
	c := reflect.ValueOf(channels)
	var cases []reflect.SelectCase
	for i := 0; i < c.Len(); i++ {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: c.Index(i),
		})
	}
	elemType := chanType.Elem()
	out := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, elemType), 0)
	go func() {
		for len(cases) > 0 {
			i, v, ok := reflect.Select(cases)
			if !ok {
				lastIndex := len(cases) - 1
				cases[i], cases[lastIndex] = cases[lastIndex], cases[i]
				cases = cases[:lastIndex]
				continue
			}
			out.Send(v)
		}
		out.Close()
	}()
	return out.Convert(reflect.ChanOf(reflect.RecvDir, elemType)).Interface()
}

// WaitError waits for either an error on the error channel, the provided context to be cancelled
// Returns an error if one is received on the error channel, otherwise it returns nil
//
// This handles a race condition where the done channel could have been closed as a result of an
// irrecoverable error being thrown, so that when the scheduler yields control back to this
// goroutine, both channels are available to read from. If the Done case happens to be chosen
// at random to proceed instead of the error case, then we would return without error which could
// result in unsafe continuation.
func WaitError(ctx context.Context, errChan <-chan error) error {
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		select {
		case err := <-errChan:
			return err
		default:
		}
		return nil
	}
}
