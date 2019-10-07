package gnode

// registry provides a (function name, function body) on-memory key-value store with adding and invoking msgTypes functionalities
import (
	"context"
	"fmt"
)

// HandleFunc is the function signature expected from all registered functions
type HandleFunc func(context.Context, []byte) ([]byte, error)

// Registry supplies the msgTypes to be called by Gossip Messages
// We assume each registry to enclose the set of functions of a single type of node e.g., execution node
type Registry interface {
	MessageTypes() map[string]HandleFunc
}

// MultiRegistry supports combining multiple registries into one
// It is suited for scenarios where multiple nodes are running on the same machine and share the
// same gossip layer
type MultiRegistry struct {
	msgTypes map[string]HandleFunc
}

// MessageTypes returns the list of msgTypes to be served
func (mr *MultiRegistry) MessageTypes() map[string]HandleFunc {
	return mr.msgTypes
}

// NewMultiRegistry receives a set of arbitrary number of registers and consolidates them into a MultiRegistry type
// Note: If there are registries containing the same msgType name, then one of
// them will be overwritten.
func NewMultiRegistry(registries ...Registry) *MultiRegistry {

	mr := MultiRegistry{msgTypes: make(map[string]HandleFunc)}

	for _, reg := range registries {
		for name, msgType := range reg.MessageTypes() {
			mr.msgTypes[name] = msgType
		}
	}
	return &mr
}

// registryRunner is used internally to wrap Registries and provide an invocation
// interface
type registryManager struct {
	msgTypes map[string]HandleFunc
}

// newRegistryManager initializes a registry manager which manges a given registry
func newRegistryManager(registry Registry) *registryManager {
	if registry == nil {
		return &registryManager{msgTypes: make(map[string]HandleFunc)}
	}

	return &registryManager{msgTypes: registry.MessageTypes()}
}

// Invoke passes input parameters to given msgType name in the registry
func (r *registryManager) Invoke(ctx context.Context, msgType string, payloadBytes []byte) (*invokeResponse, error) {
	if _, ok := r.msgTypes[msgType]; !ok {
		return nil, fmt.Errorf("could not run msgType %v: msgType does not exist", msgType)
	}

	resp, err := r.msgTypes[msgType](ctx, payloadBytes)

	return &invokeResponse{Resp: resp, Err: err}, nil
}

// AddMessageType allows a registryManager of adding a msgType to registries inside of it
func (r *registryManager) AddMessageType(msgType string, f HandleFunc) error {

	if msgType == "" {
		return fmt.Errorf("msgType name cannot be an empty string")
	}

	if _, ok := r.msgTypes[msgType]; ok {
		return fmt.Errorf("could not add msgType %v: msgType with the same name already exists", msgType)
	}

	r.msgTypes[msgType] = f

	return nil
}

// invokeResponse encapsulates results from invoked function
type invokeResponse struct {
	Resp []byte
	Err  error
}
