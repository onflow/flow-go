package tests

import (
	"os"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/module/committee"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network/mock"
)

// mockPropagationNode is a mocked node instance for testing propagation engine.
type mockPropagationNode struct {
	// the real engine to be tested
	engine *propagation.Engine
	// a mocked network layer in order for the Hub to route events in memory to a targeted node
	net *mock.Network
	// the state of the engine, exposed in order for tests to assert
	pool *mempool.Mempool
}

// newMockPropagationNode creates a mocked node with a real engine in it, and "plug" the node into a mocked hub.
func newMockPropagationNode(hub *mock.Hub, allNodes []string, nodeIndex int) (*mockPropagationNode, error) {
	if nodeIndex >= len(allNodes) {
		return nil, errors.Errorf("nodeIndex is out of range: %v", nodeIndex)
	}

	nodeEntry := allNodes[nodeIndex]

	nodeID, err := committee.EntryToID(nodeEntry)
	if err != nil {
		return nil, err
	}

	// only log error logs
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	pool, err := mempool.New()
	if err != nil {
		return nil, err
	}

	com, err := committee.New(allNodes, nodeID)
	if err != nil {
		return nil, err
	}

	net := mock.NewNetwork(com, hub)

	engine, err := propagation.New(log, net, com, pool)
	if err != nil {
		return nil, err
	}

	return &mockPropagationNode{
		engine: engine,
		net:    net,
		pool:   pool,
	}, nil
}

func (n *mockPropagationNode) terminate() {
	<-n.engine.Done()
}

func terminate(nodes ...*mockPropagationNode) {
	for _, n := range nodes {
		n.terminate()
	}
}
