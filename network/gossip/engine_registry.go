package gossip

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
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
func (er *EngineRegistry) Add(channelID uint8, engine network.Engine) error {
	if _, ok := er.store[channelID]; ok {
		return errors.Errorf("engine with ID (%v) already registered", channelID)
	}

	er.store[channelID] = engine

	return nil
}

// Process runs the given event on the specified channelID
func (er *EngineRegistry) Process(channelID uint8, originID flow.Identifier, event interface{}) error {
	engine, ok := er.store[channelID]
	if !ok {
		return errors.Errorf("could not process request. Engine with ID (%v) is not registered", channelID)
	}

	return engine.Process(originID, event)
}
