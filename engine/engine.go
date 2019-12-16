package engine

import "github.com/dapperlabs/flow-go/network"

// Engine defines the interface for engines. This extends `network.Engine`
// with methods used only within the engine's process.
type Engine interface {
	network.Engine

	// Submit submits a local event to the engine. Any errors should be
	// logged internally.
	Submit(event interface{})

	// Ready returns a channel that is closed when the engine has fully
	// started up.
	Ready() <-chan struct{}

	// Done returns a channel that is closed when the engine has fully stopped.
	// Calling Done has the effect of starting the shut-down process.
	Done() <-chan struct{}
}
