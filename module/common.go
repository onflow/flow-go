package module

import "runtime"

// ReadyDoneAware provides an easy interface to wait for module startup and shutdown.
// Modules that implement this interface only support a single start-stop cycle, and
// will not restart if Ready() is called again after shutdown has already commenced.
type ReadyDoneAware interface {
	// Ready commences startup of the module, and returns a ready channel that is closed once
	// startup has completed.
	// If shutdown has already commenced before this method is called for the first time,
	// startup will not be performed and the returned channel will never close.
	// This should be an idempotent method.
	Ready() <-chan struct{}

	// Done commences shutdown of the module, and returns a done channel that is closed once
	// shutdown has completed.
	// This should be an idempotent method.
	Done() <-chan struct{}
}

type NoopReadDoneAware struct{}

func (n *NoopReadDoneAware) Ready() <-chan struct{} {
	ready := make(chan struct{})
	defer close(ready)
	return ready
}

func (n *NoopReadDoneAware) Done() <-chan struct{} {
	done := make(chan struct{})
	defer close(done)
	return done
}

// ErrorAware provides an interface to be notified of errors encountered during
// a component's lifecycle.
type ErrorAware interface {
	Errors() <-chan error
}

// ErrorBase implements the ErrorAware interface, and provides a way for components
// to signal an irrecoverable error.
type ErrorBase struct {
	errors chan error
}

func NewErrorBase() *ErrorBase {
	return &ErrorBase{make(chan error)}
}

func (e *ErrorBase) Errors() <-chan error {
	return e.errors
}

func (e *ErrorBase) ThrowError(err error) {
	e.errors <- err
	runtime.Goexit()
}
