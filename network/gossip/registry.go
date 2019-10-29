package gossip

// registry provides a (function name, function body) on-memory key-value store with adding and invoking msgTypes functionalities
import (
	"context"
	"fmt"
	"sync"
)

// HandleFunc is the function signature expected from all registered functions
type HandleFunc func(context.Context, []byte) ([]byte, error)

// Registry supplies the msgTypes to be called by Gossip Messages
// We assume each registry to enclose the set of functions of a single type of node e.g., execution node
type Registry interface {
	MessageTypes() map[uint64]HandleFunc
	NameMapping() map[string]uint64
}

// registryRunner is used internally to wrap Registries and provide an invocation
// interface
type registryManager struct {
	msgTypes  map[uint64]HandleFunc
	msgTypeID map[string]uint64
	idMsgType map[uint64]string
	mu        sync.RWMutex
}

// MsgTypeToID returns the numerical value mapped to the given message type
func (r *registryManager) MsgTypeToID(msgType string) (uint64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	val, ok := r.msgTypeID[msgType]
	if !ok {
		return 0, fmt.Errorf("msgType %v was not found", msgType)
	}

	return val, nil
}

// IDtoMsgType takes a message id and returns the message corresponding to it.
func (r *registryManager) IDtoMsgType(ID uint64) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	val, ok := r.idMsgType[ID]

	if !ok {
		return "", fmt.Errorf("message ID %v cannot be resolved, not type is found", ID)
	}
	return val, nil
}

// newRegistryManager initializes a registry manager which manges a given registry
func newRegistryManager(registry Registry) *registryManager {
	if registry == nil {
		return &registryManager{
			msgTypes:  make(map[uint64]HandleFunc),
			msgTypeID: make(map[string]uint64),
			idMsgType: make(map[uint64]string),
		}
	}

	return &registryManager{
		msgTypes:  registry.MessageTypes(),
		msgTypeID: registry.NameMapping(),
		idMsgType: getIDMappings(registry.NameMapping()),
	}
}

// getIDMappings takes a mapping from string (message types) to uint64 (message ids) and returns the inverse of this mapping,
// i.e., from uint64 to string.
func getIDMappings(msgTypeMap map[string]uint64) map[uint64]string {

	mp := make(map[uint64]string)

	for k, v := range msgTypeMap {
		mp[v] = k
	}

	return mp
}

// Invoke passes input parameters to given msgType name in the registry
func (r *registryManager) Invoke(ctx context.Context, msgType uint64, payloadBytes []byte) (*invokeResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.msgTypes[msgType]; !ok {
		return nil, fmt.Errorf("could not run msgType %v: msgType does not exist", msgType)
	}

	resp, err := r.msgTypes[msgType](ctx, payloadBytes)

	return &invokeResponse{Resp: resp, Err: err}, nil
}

// AddMessageType allows a registryManager of adding a msgType to registries inside of it
func (r *registryManager) AddMessageType(msgType string, f HandleFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if msgType == "" {
		return fmt.Errorf("msgType name cannot be an empty string")
	}

	if _, ok := r.msgTypeID[msgType]; ok {
		return fmt.Errorf("could not add msgType %v: msgType with the same name already exists", msgType)
	}

	// Predicate: There is no message in the map with a uint that is greater than
	// the size of the map. If there is return an error

	// record the current number Of MsgTypes in order to add a new non-conflicting
	// one
	n := len(r.msgTypes)

	msgID := uint64(n)

	if _, ok := r.msgTypes[msgID]; ok {
		return fmt.Errorf("could not add msgType: registry does not comply with the expected protocol")
	}

	r.msgTypeID[msgType] = msgID
	r.msgTypes[msgID] = f
	r.idMsgType[msgID] = msgType
	return nil
}

// invokeResponse encapsulates results from invoked function
type invokeResponse struct {
	Resp []byte
	Err  error
}
