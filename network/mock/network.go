package mock

import (
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
	cache   map[string]struct{}
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

func (mn *Network) Hub() *Hub {
	return mn.hub
}

// submit is called when an Engine is sending an event to an Engine on another node or nodes.
func (mn *Network) submit(engineID uint8, event interface{}, targetIDs ...string) error {
	mn.hub.Buffer(mn.GetID(), engineID, event, targetIDs)
	return nil
}

// GetID returns the identity of the Node.
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

// return a certain node has seen a certain key
func (mn *Network) haveSeen(key string) bool {
	mn.Lock()
	defer mn.Unlock()
	seen, ok := mn.seenEventIDs[key]
	if !ok {
		return false
	}
	return seen
}

// mark a certain node has seen a certain event for a certain engine
func (mn *Network) seen(key string) {
	mn.Lock()
	defer mn.Unlock()
	mn.seenEventIDs[key] = true
}
