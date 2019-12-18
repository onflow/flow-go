package engine

// Engine defines the internal interface for engines. This is the interface
// used by other components in the same node as the engine.
type Engine interface {
	// Submit submits a local event to the engine. It is identical to
	// `network#Engine.Process` but sets the origin ID to the node ID.
	Submit(event interface{}) error
}
