package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

type hash [32]byte

// Network is a mocked network layer made for testing Engine's behavior.
// When an Engine is installed on a Network, the mocked network will deliver
// all Engine's events synchronously in memory to another Engine, so that tests can run
// fast and easy to assert errors.
type Network struct {
	hub     *Hub
	com     module.Committee
	engines map[uint8]network.Engine
	sync.Mutex
	seenEventIDs map[string]bool
}

// NewNetwork create a mocked network.
// The committee has the identity of the node already, so only `committee` is needed
// in order for a mock hub to find each other.
func NewNetwork(com module.Committee, hub *Hub) *Network {
	o := &Network{
		com:          com,
		hub:          hub,
		engines:      make(map[uint8]network.Engine),
		seenEventIDs: make(map[string]bool),
	}
	// Plug the network to a hub so that networks can find each other.
	hub.Plug(o)
	return o
}

// submit is called when an Engine is sending an event to an Engine on another node or nodes.
func (mn *Network) submit(engineID uint8, event interface{}, targetIDs ...string) error {
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	eventHash := sha256.Sum256(eventBytes)

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

		if receiverNetwork.haveSeen(nodeID, eventHash) {
			continue
		}

		// mark the peer has seen the event
		receiverNetwork.seen(nodeID, eventHash)

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

// Unregister removes the engine and prevent the memleak
func (mn *Network) Unregister(engineID uint8, engine network.Engine) error {
	engine, ok := mn.engines[engineID]
	if !ok {
		return errors.Errorf("engine code does not exist (%d)", engineID)
	}
	delete(mn.engines, engineID)
	return nil
}

// return the cached key for seen event
func eventKey(nodeID string, eventHash hash) string {
	bytes := eventHash[:]
	key := hex.EncodeToString(bytes)
	return nodeID + "-" + key
}

// return a certain node has seen a certain event
func (mn *Network) haveSeen(nodeID string, eventHash hash) bool {
	mn.Lock()
	defer mn.Unlock()
	seen, ok := mn.seenEventIDs[eventKey(nodeID, eventHash)]
	if !ok {
		return false
	}
	return seen
}

// mark a certain node has seen a certain event
func (mn *Network) seen(nodeID string, eventHash hash) {
	mn.Lock()
	defer mn.Unlock()
	mn.seenEventIDs[eventKey(nodeID, eventHash)] = true
}
