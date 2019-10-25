package mocks

import (
	"github.com/dapperlabs/flow-go/pkg/module"
	"github.com/dapperlabs/flow-go/pkg/network"
	"github.com/pkg/errors"
)

// MockNetwork is a mocked network layer made for testing Engine's behavior.
// When an Engine is installed on a MockNetwork, the mocked network will deliver
// all Engine's events synchronously in memory to another Engine, so that tests can run
// fast and easy to assert errors.
type MockNetwork struct {
	hub     *MockHub
	com     module.Committee
	engines map[uint8]network.Engine
}

func NewNetwork(com module.Committee, hub *MockHub) (*MockNetwork, error) {
	o := &MockNetwork{
		com:     com,
		hub:     hub,
		engines: make(map[uint8]network.Engine),
	}
	// Plug the network to a hub so that networks can find each other.
	hub.Plug(o)
	return o, nil
}

// submit is called when an Engine is sending an event to an Engine on another node or nodes.
func (mn *MockNetwork) submit(engineID uint8, event interface{}, targetIDs ...string) error {
	var err error
	for _, nodeID := range targetIDs {
		// Find the network of the targeted node
		receiverNetwork, ok := mn.hub.Networks[nodeID]
		if !ok || receiverNetwork == nil {
			return errors.Errorf("MockNetwork can not find a node by ID %v", nodeID)
		}
		// Find the engine of the targeted network
		receiverEngine, ok := receiverNetwork.engines[engineID]
		if !ok {
			return errors.Errorf("MockNetwork can not find engine ID: %v for node: %v", engineID, nodeID)
		}
		// Call `Process` to let receiver engine receive the event directly.
		err = receiverEngine.Process(mn.GetID(), event)
		if err != nil {
			return errors.Wrapf(err, "senderEngine failed to process event: %v", event)
		}
	}
	return nil
}

// GetID returns the identity of the Engine
func (mn *MockNetwork) GetID() string {
	me := mn.com.Me()
	return me.ID
}

func (mn *MockNetwork) Register(engineID uint8, engine network.Engine) (network.Conduit, error) {
	_, ok := mn.engines[engineID]
	if ok {
		return nil, errors.Errorf("engine code already taken (%d)", engineID)
	}
	conduit := &MockConduit{
		engineID: engineID,
		send:     mn.submit,
	}
	mn.engines[engineID] = engine
	return conduit, nil
}
