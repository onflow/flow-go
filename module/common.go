package module

import (
	"context"
	"runtime"
)

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

// Startable provides an interface to start a component. Once started, the component
// can be stopped by cancelling the given context.
type Startable interface {
	Start(context.Context) error
}

// ErrorAware provides an interface to be notified of errors encountered during
// a component's lifecycle.
type ErrorAware interface {
	Errors() <-chan error
}

// ErrorManager implements the ErrorAware interface, and provides a way for components
// to signal an irrecoverable error.
type ErrorManager struct {
	errors chan error
}

func NewErrorManager() *ErrorManager {
	return &ErrorManager{make(chan error)}
}

func (e *ErrorManager) Errors() <-chan error {
	return e.errors
}

func (e *ErrorManager) ThrowError(err error) {
	e.errors <- err
	runtime.Goexit()
}

type ErrorHandler func(ctx context.Context, errors <-chan error, restart func())

type Component interface {
	Startable
	ReadyDoneAware
	ErrorAware
}

type ComponentFactory func() (Component, error)

func RunComponent(ctx context.Context, componentFactory ComponentFactory, errorHandler ErrorHandler) error {
	restartChan := make(chan struct{})

	start := func() (context.CancelFunc, <-chan struct{}, error) {
		component, err := componentFactory()
		if err != nil {
			// failed to create component
			return nil, nil, err
		}

		// context used to restart the component
		runCtx, cancel := context.WithCancel(ctx)
		if err := component.Start(runCtx); err != nil {
			// failed to start component
			cancel()
			return nil, nil, err
		}

		select {
		case <-ctx.Done():
			runtime.Goexit()
		case <-component.Ready():
		}

		go errorHandler(runCtx, component.Errors(), func() {
			restartChan <- struct{}{}
			runtime.Goexit()
		})

		return cancel, component.Done(), nil
	}

	for {
		cancel, done, err := start()
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-restartChan:
			// shutdown the component
			cancel()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
		}
	}
}
