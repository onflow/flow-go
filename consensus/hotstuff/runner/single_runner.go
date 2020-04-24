package runner

import (
	"fmt"
	"sync"
)

// SingleRunner is a support struct for implementing module.ReadyDoneAware
type SingleRunner struct {
	stateTransition sync.Mutex // lock for preventing concurrent state transitions

	startupCommenced bool          // indicates whether Start(...) invoked
	startupCompleted chan struct{} // channel is closed when startup was completed and the component is properly running

	shutdownCommenced bool          // indicates whether Stop() invoked
	shutdownSignal    chan struct{} // used to signal that shutdown has commenced

	shutdownCompleted chan struct{} // used to signal that shutdown was completed and the component is done
}

func NewSingleRunner() SingleRunner {
	return SingleRunner{
		stateTransition:   sync.Mutex{},
		startupCommenced:  false,
		startupCompleted:  make(chan struct{}),
		shutdownCommenced: false,
		shutdownSignal:    make(chan struct{}),
		shutdownCompleted: make(chan struct{}),
	}
}

// ShutdownSignal returns a channel that is closed when the shutdown signal has been given
func (u *SingleRunner) ShutdownSignal() <-chan struct{} {
	return u.shutdownSignal
}

func (s *SingleRunner) Start(f func()) <-chan struct{} {
	s.stateTransition.Lock()
	if s.startupCommenced || s.shutdownCommenced {
		s.stateTransition.Unlock()
		return s.startupCompleted
	}

	s.startupCommenced = true
	fmt.Printf("single runner start\n")
	go func() {
		close(s.startupCompleted)
		s.stateTransition.Unlock()
		fmt.Printf("Start f...\n")
		f()
		// there are two cases f() would exit:
		// (a) f exited on its own without Abort() being called (this is generally an internal error)
		// (b) f exited as a reaction to Abort() being called
		// In either case, we want to abort and close shutdownCompleted
		fmt.Printf("Start Lock...\n")
		s.stateTransition.Lock()
		s.unsafeCommenceShutdown()
		close(s.shutdownCompleted)
		s.stateTransition.Unlock()
	}()

	return s.startupCompleted
}

// Abort() will abort the SingleRunner. Note that the channel returned from Start() will never be
// closed if the SingleRunner is aborted before Start() is called. This mimics the real world case:
//    * consider a runner at a starting position of a race waiting for the start signal
//    * if the runner to told to abort the race _before_ the start signal occurred, the runner will never start
func (s *SingleRunner) Abort() <-chan struct{} {
	defer fmt.Printf("abort done\n")
	s.stateTransition.Lock()
	fmt.Printf("close shutdown signal \n")
	s.unsafeCommenceShutdown()
	s.stateTransition.Unlock()
	return s.shutdownCompleted
}

// Completed() will wait for the SingleRunner to stop running. It will not initialize a shutdown.
func (s *SingleRunner) Completed() <-chan struct{} {
	return s.shutdownCompleted
}

// unsafeCommenceShutdown executes the shutdown logic once but is not concurrency safe
func (s *SingleRunner) unsafeCommenceShutdown() {
	if !s.shutdownCommenced {
		s.shutdownCommenced = true
		close(s.shutdownSignal)
	}
}
