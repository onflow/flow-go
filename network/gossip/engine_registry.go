package gossip

import (
	"github.com/dapperlabs/flow-go/network"
	"github.com/pkg/errors"
)

// EngineRegistry provies an engine store for gossip nodes
type EngineRegistry struct {
	store map[uint8]network.Engine
}

// NewEngineRegistry returns a new empty engine registry
func NewEngineRegistry() (*EngineRegistry, error) {
	return &EngineRegistry{
		store: make(map[uint8]network.Engine),
	}, nil
}

// Add adds an engine to the store of the engine registry
func (er *EngineRegistry) Add(engineID uint8, engine network.Engine) error {
	if _, ok := er.store[engineID]; ok {
		return errors.Errorf("engine with ID (%v) already registered", engineID)
	}

	er.store[engineID] = engine

	return nil
}

// Process runs the given event on the specified engineID
func (er *EngineRegistry) Process(engineID uint8, originID string, event interface{}) error {
	engine, ok := er.store[engineID]
	if !ok {
		return errors.Errorf("could not process request. Engine with ID (%v) is not registered", engineID)
	}

	return engine.Process(originID, event)
}
