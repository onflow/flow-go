package registry

import "context"

// MessageType is a type representing mapping of registry
type MessageType uint64

// HandleFunc is the function signature expected from all registered functions
type HandleFunc func(context.Context, []byte) ([]byte, error)

// Registry supplies the msgTypes to be called by Gossip Messages
// We assume each registry to enclose the set of functions of a single type of node e.g., execution node
type Registry interface {

	// MessageTypes returns the mapping of the registry as a map data structure
	// which takes an enum type MessageType that identifies a certain message
	// and the returns a function that is invoked as a response to a message
	MessageTypes() map[MessageType]HandleFunc
}
