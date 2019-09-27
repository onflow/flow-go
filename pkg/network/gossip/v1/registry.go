package gnode

import "fmt"

// HandleFunc is the function signature expected from all registered functions
// maybe also pass the context as a parameter?
type HandleFunc func([]byte) ([]byte, error)

// Registry supplies the methods to be called by Gossip Messages
type Registry interface {
	Methods() map[string]HandleFunc
}

// MultiRegistry provides a way to combine multiple registries into one
type MultiRegistry struct {
	methods map[string]HandleFunc
}

func (mr *MultiRegistry) Methods() map[string]HandleFunc {
	return mr.methods
}

// NewMultiRegistry returns a new Registry which is a combination of all given
// registries.
// Note: If there are registries containing the same method name, then one of
// them will be overwritten.
func NewMultiRegistry(registries ...Registry) *MultiRegistry {

	mr := MultiRegistry{methods: make(map[string]HandleFunc)}

	for _, reg := range registries {
		for name, method := range reg.Methods() {
			mr.methods[name] = method
		}
	}

	return &mr
}

// registryRunner is used internally to wrap Registries and provide an invocation
// interface
type registryManager struct {
	methods map[string]HandleFunc
}

func newRegistryManager(registry Registry) *registryManager {
	return &registryManager{methods: registry.Methods()}
}

func newEmptyRegistryManager() *registryManager {
	return &registryManager{methods: make(map[string]HandleFunc)}
}

func (r *registryManager) Invoke(method string, payloadBytes []byte) (*invokeResponse, error) {
	if _, ok := r.methods[method]; !ok {
		return nil, fmt.Errorf("could not run method %v: method does not exist", method)
	}

	resp, err := r.methods[method](payloadBytes)

	return &invokeResponse{Resp: resp, Err: err}, nil
}

func (r *registryManager) AddMethod(method string, f HandleFunc) error {
	if _, ok := r.methods[method]; ok {
		return fmt.Errorf("could not add method %v: method with the same name already exists", method)
	}

	r.methods[method] = f

	return nil
}

type invokeResponse struct {
	Resp []byte
	Err  error
}
