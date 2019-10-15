// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package naive

// Network represents the overlay network, which allows engines to register in
// order to communicate with the same engine on other nodes.
type Network interface {
	Register(code uint8, engine Engine) (Conduit, error)
}

// Conduit represents the intersection of the network stack with a specific
// engine, allowing the engine to send events to the same process on other
// nodes.
type Conduit interface {
	Send(event interface{}, recipients ...string) error
}

// Engine represents a node process running in the flow network. It provides the
// network stack with a way to submit events from other nodes for processing, as
// well as a way to uniquely identify events and request them by ID.
type Engine interface {
	Receive(sender string, event interface{}) error
	Identify(event interface{}) ([]byte, error)
	Retrieve(id []byte) (interface{}, error)
}
