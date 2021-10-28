package module

import (
	"errors"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// WARNING: The semantics of this interface will be changing in the near future, with
// startup / shutdown capabilities being delegated to the Startable interface instead.
// For more details, see https://github.com/onflow/flow-go/pull/1167
//
// ReadyDoneAware provides an easy interface to wait for module startup and shutdown.
// Modules that implement this interface only support a single start-stop cycle, and
// will not restart if Ready() is called again after shutdown has already commenced.
type ReadyDoneAware interface {
	// Ready commences startup of the module, and returns a ready channel that is closed once
	// startup has completed. Note that the ready channel may never close if errors are
	// encountered during startup.
	// If shutdown has already commenced before this method is called for the first time,
	// startup will not be performed and the returned channel will also never close.
	// This should be an idempotent method.
	Ready() <-chan struct{}

	// Done commences shutdown of the module, and returns a done channel that is closed once
	// shutdown has completed. Note that the done channel should be closed even if errors are
	// encountered during shutdown.
	// This should be an idempotent method.
	Done() <-chan struct{}
}

// NoopReadyDoneAware is a ReadyDoneAware implementation whose ready/done channels close
// immediately
type NoopReadyDoneAware struct{}

func (n *NoopReadyDoneAware) Ready() <-chan struct{} {
	ready := make(chan struct{})
	defer close(ready)
	return ready
}

func (n *NoopReadyDoneAware) Done() <-chan struct{} {
	done := make(chan struct{})
	defer close(done)
	return done
}

// CustomReadyDoneAware is a ReadyDoneAware implementation that allows the instantiator
// to provide the ready/done channels. This is useful for building aggregate interfaces
// e.g. a collection of ReadyDoneAware objects, or a WaitGroup based approach.
type CustomReadyDoneAware struct {
	ready <-chan struct{}
	done  <-chan struct{}
}

func NewCustomReadyDoneAware(ready, done <-chan struct{}) *CustomReadyDoneAware {
	return &CustomReadyDoneAware{ready, done}
}

func (c *CustomReadyDoneAware) Ready() <-chan struct{} {
	return c.ready
}

func (c *CustomReadyDoneAware) Done() <-chan struct{} {
	return c.done
}

var ErrMultipleStartup = errors.New("component may only be started once")

// Startable provides an interface to start a component. Once started, the component
// can be stopped by cancelling the given context.
type Startable interface {
	// Start starts the component. Any irrecoverable errors encountered while the component is running
	// should be thrown with the given SignalerContext.
	// This method should only be called once, and subsequent calls should panic with ErrMultipleStartup.
	Start(irrecoverable.SignalerContext)
}
