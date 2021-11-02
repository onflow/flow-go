package lifecycle

import (
	"sync"
)

// LifecycleManager is a support struct for implementing module.ReadyDoneAware
type LifecycleManager struct {
	stateTransition   sync.Mutex    // lock for preventing concurrent state transitions
	started           chan struct{} // used to signal that startup has completed
	stopped           chan struct{} // used to signal that shutdown has completed
	startupCommenced  bool          // indicates whether OnStart() has been invoked
	shutdownCommenced bool          // indicates whether OnStop() has been invoked
	shutdownSignal    chan struct{} // used to signal that shutdown has commenced
}

func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		stateTransition:   sync.Mutex{},
		startupCommenced:  false,
		started:           make(chan struct{}),
		shutdownCommenced: false,
		stopped:           make(chan struct{}),
		shutdownSignal:    make(chan struct{}),
	}
}

// OnStart will commence startup of the LifecycleManager. If OnStop has already been called
// before the first call to OnStart, startup will not be performed. After the first call,
// subsequent calls to OnStart do nothing.
func (lm *LifecycleManager) OnStart(startupFns ...func()) {
	lm.stateTransition.Lock()
	if lm.shutdownCommenced || lm.startupCommenced {
		lm.stateTransition.Unlock()
		return
	}
	lm.startupCommenced = true
	lm.stateTransition.Unlock()

	go func() {
		for _, fn := range startupFns {
			fn()
		}
		close(lm.started)
	}()
}

// OnStop will commence shutdown of the LifecycleManager. If the LifecycleManager is still
// starting up, we will wait for startup to complete before shutting down. After the first
// call, subsequent calls to OnStop do nothing.
func (lm *LifecycleManager) OnStop(shutdownFns ...func()) {
	lm.stateTransition.Lock()
	if lm.shutdownCommenced {
		lm.stateTransition.Unlock()
		return
	}
	lm.shutdownCommenced = true
	lm.stateTransition.Unlock()

	close(lm.shutdownSignal)
	go func() {
		if lm.startupCommenced {
			<-lm.started
			for _, fn := range shutdownFns {
				fn()
			}
		}
		close(lm.stopped)
	}()
}

// ShutdownSignal returns a channel that is closed when shutdown has commenced.
func (lm *LifecycleManager) ShutdownSignal() <-chan struct{} {
	return lm.shutdownSignal
}

// Started returns a channel that is closed when startup has completed.
// If the LifecycleManager is stopped before OnStart() is ever called,
// the returned channel will never be closed.
func (lm *LifecycleManager) Started() <-chan struct{} {
	return lm.started
}

// Stopped returns a channel that is closed when shutdown has completed
func (lm *LifecycleManager) Stopped() <-chan struct{} {
	return lm.stopped
}
