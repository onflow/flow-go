package registry

// registry provides a (function name, function body) on-memory key-value store with adding and invoking msgTypes functionalities
// In the current version of the gossip, RegistryManager is solely utilized internally by the gossip layer for its internal affairs,
// and is not meant for the application layer.
import (
	"context"
	"fmt"
	"sync"
)

const (
	DefaultTypes = 3
)

// RegistryManager is used internally to wrap Registries and provide an invocation interface
type RegistryManager struct {
	msgTypes map[MessageType]HandleFunc
	mu       sync.RWMutex
}

// NewRegistryManager initializes a registry manager which manges a given registry
func NewRegistryManager(registry Registry) *RegistryManager {
	if registry == nil {
		return &RegistryManager{
			msgTypes: make(map[MessageType]HandleFunc),
		}
	}

	return &RegistryManager{
		msgTypes: registry.MessageTypes(),
	}
}

// revive:disable:unexported-return

// Invoke passes input parameters to given msgType handler in the registry
func (r *RegistryManager) Invoke(ctx context.Context, msgType MessageType, payloadBytes []byte) (*invokeResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.msgTypes[msgType]
	if !ok {
		return nil, fmt.Errorf("could not run msgType %v: msgType does not exist", msgType)
	}
	resp, err := handler(ctx, payloadBytes)

	return &invokeResponse{Resp: resp, Err: err}, nil
}

// revive:enable

// AddDefaultType adds defaults handlers to registry
func (r *RegistryManager) AddDefaultTypes(msgType []MessageType, f []HandleFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < len(msgType); i++ {
		r.msgTypes[msgType[i]] = f[i]
	}

	return nil
}

// AddMessageType adds a msgType and its handler to the RegistryManager
func (r *RegistryManager) AddMessageType(mType MessageType, f HandleFunc, forcedRegister bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.msgTypes[mType]; ok && !forcedRegister {
		return fmt.Errorf("Could not override message type: %v", mType)
	}

	r.msgTypes[mType] = f

	return nil
}

// invokeResponse encapsulates results from invoked function
type invokeResponse struct {
	Resp []byte
	Err  error
}
