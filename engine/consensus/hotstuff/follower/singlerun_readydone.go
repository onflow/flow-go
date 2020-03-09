package follower

import (
	"sync"
)

type state int

// SingleRunner is a support struct for implementing module.ReadyDoneAware
type SingleRunner struct {
	stateTransition sync.Mutex // lock for preventing concurrent state transitions

	startupSignaled  bool          // indicates whether Start(...) invoked
	startupCompleted chan struct{} // channel is closed when startup was completed and the component is properly running

	shutdownSignaled bool          // indicates whether Stop() invoked
	shutdownSignal   chan struct{} // used to signal that shutdown signal has been given

	shutdownCompleted chan struct{} // used to signal that shutdown was completed and the component is done
}

func NewSingleRunner() SingleRunner {
	return SingleRunner{
		stateTransition:   sync.Mutex{},
		startupSignaled:   false,
		startupCompleted:  make(chan struct{}),
		shutdownSignaled:  false,
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
	if s.startupSignaled || s.shutdownSignaled {
		s.stateTransition.Unlock()
		return s.startupCompleted
	}

	s.startupSignaled = true
	go func() {
		close(s.startupCompleted)
		s.stateTransition.Unlock()
		f()
		close(s.shutdownCompleted)
	}()

	return s.startupCompleted
}

func (s *SingleRunner) Stop() <-chan struct{} {
	s.stateTransition.Lock()
	if !s.shutdownSignaled {
		close(s.shutdownSignal)
	}
	s.stateTransition.Unlock()
	return s.shutdownCompleted
}
