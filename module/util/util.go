package util

import (
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
