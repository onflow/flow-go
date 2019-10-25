package tests

import (
	"github.com/stretchr/testify/assert"
	"math/rand"

	"github.com/dapperlabs/flow-go/internal/roles/consensus/propagation"
	"github.com/dapperlabs/flow-go/pkg/model/collection"
	"github.com/dapperlabs/flow-go/pkg/module/committee"
	"github.com/dapperlabs/flow-go/pkg/module/mempool"
	"github.com/dapperlabs/flow-go/pkg/network/trickle/mocks"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"os"
	"testing"
)

type MockPropergationNode struct {
	net    *mocks.MockNetwork
	pool   *mempool.Mempool
	engine *propagation.Engine
}

func NewMockPropgationNode(hub *mocks.MockHub, allNodes []string, nodeIndex int) (*MockPropergationNode, error) {
	if nodeIndex >= len(allNodes) {
		return nil, errors.Errorf("nodeIndex is out of range: %v", nodeIndex)
	}
	nodeEntry := allNodes[nodeIndex]
	nodeID, err := committee.EntryToId(nodeEntry)
	if err != nil {
		return nil, err
	}

	log := zerolog.New(os.Stderr).With().Logger()
	pool, err := mempool.New()
	if err != nil {
		return nil, err
	}

	com, err := committee.New(allNodes, nodeID)
	if err != nil {
		return nil, err
	}

	net, err := mocks.NewNetwork(com, hub)
	if err != nil {
		return nil, err
	}

	prop, err := propagation.NewEngine(log, net, com, pool)
	if err != nil {
		return nil, err
	}

	return &MockPropergationNode{
		net:    net,
		pool:   pool,
		engine: prop,
	}, nil
}

func createConnectedNodes(nodeEntries []string) (*mocks.MockHub, []*MockPropergationNode, error) {
	if len(nodeEntries) == 0 {
		return nil, nil, errors.New("NodeEntries must not be empty")
	}
	hub := mocks.NewNetworkHub()
	nodes := make([]*MockPropergationNode, len(nodeEntries))
	for i := range nodeEntries {
		node, err := NewMockPropgationNode(hub, nodeEntries, i)
		if err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, node)
	}
	return hub, nodes, nil
}

// a utiliy func to return a random collection hash
func randHash() ([]byte, error) {
	hash := make([]byte, 32)
	_, err := rand.Read(hash)
	return hash, err
}

func TestSubmitCollection(t *testing.T) {
	// If a consensus node receives a collection hash, then another connected node should receive it as well.
	t.Run("should propagate collection to connected nodes", func(t *testing.T) {
		// create a mocked network for each node and connect them in a in-memory hub, so that events sent from one engine
		// can be delivery directly to another engine on a different node
		_, nodes, err := createConnectedNodes([]string{"consensus-consensus1@localhost:7297", "consensus-consensus2@localhost:7297"})
		assert.Nil(t, err)

		node1 := nodes[0]
		node2 := nodes[1]

		hash, err := randHash()
		assert.Nil(t, err)

		gc := &collection.GuaranteedCollection{
			Hash: hash,
		}
		// node1's engine receives a collection hash
		err = node1.engine.SubmitGuaranteedCollection(gc)
		assert.Nil(t, err)

		// inspect node2's mempool to check if node2's engine received the collection hash
		coll, err := node2.pool.Get(hash)
		assert.Nil(t, err)

		// should match
		assert.Equal(t, coll.Hash, hash)
	})
}
