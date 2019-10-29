package mock

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/trickle"
	"github.com/dapperlabs/flow-go/network/trickle/state"
)

// Network is a mocked network layer made for testing Engine's behavior.
// When an Engine is installed on a Network, the mocked network will deliver
// all Engine's events synchronously in memory to another Engine, so that tests can run
// fast and easy to assert errors.
type Network struct {
	hub     *Hub
	com     module.Committee
	engines map[uint8]network.Engine
	state   trickle.State
}

// NewNetwork create a mocked network.
// The committee has the identity of the node already, so only `committee` is needed
// in order for a mock hub to find each other.
func NewNetwork(com module.Committee, hub *Hub) (*Network, error) {
	state, err := state.New()
	if err != nil {
		return nil, err
	}

	o := &Network{
		com:     com,
		hub:     hub,
		engines: make(map[uint8]network.Engine),
		state:   state,
	}
	// Plug the network to a hub so that networks can find each other.
	hub.Plug(o)
	return o, nil
}

// submit is called when an Engine is sending an event to an Engine on another node or nodes.
func (mn *Network) submit(engineID uint8, event interface{}, targetIDs ...string) error {
	var err error
	engine, ok := mn.engines[engineID]
	if !ok {
		return errors.Errorf("missing engine for engineID %v", engineID)
	}
	eventID, err := engine.Identify(event)
	if err != nil {
		return err
	}

	for _, nodeID := range targetIDs {
		// Find the network of the targeted node
		receiverNetwork, ok := mn.hub.Networks[nodeID]
		if !ok || receiverNetwork == nil {
			return errors.Errorf("Network can not find a node by ID %v", nodeID)
		}

		receiverEngine, ok := receiverNetwork.engines[engineID]
		if !ok {
			return errors.Errorf("Network can not find engine ID: %v for node: %v", engineID, nodeID)
		}

		// Make sure the peer is up
		receiverNetwork.state.Up(nodeID)

		if receiverNetwork.state.PeerHaveSeen(nodeID, eventID) {
			continue
		}

		// mark the peer has seen the event
		receiverNetwork.state.Seen(nodeID, eventID)

		// Find the engine of the targeted network
		// Call `Process` to let receiver engine receive the event directly.
		err = receiverEngine.Process(mn.GetID(), event)

		if err != nil {
			return errors.Wrapf(err, "senderEngine failed to process event: %v", event)
		}
	}
	return nil
}

// GetID returns the identity of the Engine
func (mn *Network) GetID() string {
	me := mn.com.Me()
	return me.NodeID
}

// Register implements pkg/module/Network's interface
func (mn *Network) Register(engineID uint8, engine network.Engine) (network.Conduit, error) {
	_, ok := mn.engines[engineID]
	if ok {
		return nil, errors.Errorf("engine code already taken (%d)", engineID)
	}
	conduit := &Conduit{
		engineID: engineID,
		submit:   mn.submit,
	}
	mn.engines[engineID] = engine
	return conduit, nil
}
