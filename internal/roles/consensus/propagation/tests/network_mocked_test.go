package tests

import (
	"math/rand"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/internal/roles/consensus/propagation"
	"github.com/dapperlabs/flow-go/pkg/model/collection"
	"github.com/dapperlabs/flow-go/pkg/module/committee"
	"github.com/dapperlabs/flow-go/pkg/module/mempool"
	"github.com/dapperlabs/flow-go/pkg/network/mock"
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

	log := zerolog.New(os.Stderr).With().Logger()

	pool, err := mempool.New()
	if err != nil {
		return nil, err
	}

	com, err := committee.New(allNodes, nodeID)
	if err != nil {
		return nil, err
	}

	net, err := mock.NewNetwork(com, hub)
	if err != nil {
		return nil, err
	}

	engine, err := propagation.NewEngine(log, net, com, pool)
	if err != nil {
		return nil, err
	}

	return &mockPropagationNode{
		engine: engine,
		net:    net,
		pool:   pool,
	}, nil
}

func createConnectedNodes(nodeEntries []string) (*mock.Hub, []*mockPropagationNode, error) {
	if len(nodeEntries) == 0 {
		return nil, nil, errors.New("NodeEntries must not be empty")
	}

	hub := mock.NewNetworkHub()

	nodes := make([]*mockPropagationNode, 0)
	for i := range nodeEntries {
		node, err := newMockPropagationNode(hub, nodeEntries, i)
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

// a utiliy func to generate a GuaranteedCollection with random hash
func randCollectionHash() (*collection.GuaranteedCollection, error) {
	hash, err := randHash()
	if err != nil {
		return nil, err
	}
	return &collection.GuaranteedCollection{
		Hash: hash,
	}, nil
}

func TestSubmitCollection(t *testing.T) {
	// If a consensus node receives a collection hash, then another connected node should receive it as well.
	t.Run("should propagate collection to connected nodes", func(t *testing.T) {
		// create a mocked network for each node and connect them in a in-memory hub, so that events sent from one engine
		// can be delivery directly to another engine on a different node
		_, nodes, err := createConnectedNodes([]string{"consensus-consensus1@localhost:7297", "consensus-consensus2@localhost:7298"})
		require.Nil(t, err)

		node1 := nodes[0]
		node2 := nodes[1]

		// prepare a random collection hash
		gc, err := randCollectionHash()
		require.Nil(t, err)

		// node1's engine receives a collection hash
		err = node1.engine.SubmitGuaranteedCollection(gc)
		require.Nil(t, err)

		// inspect node2's mempool to check if node2's engine received the collection hash
		coll, err := node2.pool.Get(gc.Hash)
		require.Nil(t, err)

		// should match
		require.Equal(t, coll.Hash, gc.Hash)
	})

	// The propagation engine has a behavior property:
	// If 3 nodes are connected together, then sending M1 to A, M2 to B, M3 to C will result
	// the 3 nodes having all 3 messages in their mempool, and their mempool should produce
	// the same hash
	t.Run("all nodes should have the same mempool state after exchanging received collections",
		func(t *testing.T) {
			_, nodes, err := createConnectedNodes([]string{
				"consensus-consensus1@localhost:7297",
				"consensus-consensus2@localhost:7298",
				"consensus-consensus3@localhost:7299",
			})
			require.Nil(t, err)

			// prepare 3 nodes that are connected to each other
			node1 := nodes[0]
			node2 := nodes[1]
			node3 := nodes[2]

			// prepare 3 different GuaranteedCollections: gc1, gc2, gc3
			gc1, err := randCollectionHash()
			require.Nil(t, err)

			gc2, err := randCollectionHash()
			require.Nil(t, err)

			gc3, err := randCollectionHash()
			require.Nil(t, err)

			// check the collections are different
			require.NotEqual(t, gc1.Hash, gc2.Hash)

			// send gc1 to node1, which will broadcast to other nodes synchronously
			err = node1.engine.SubmitGuaranteedCollection(gc1)
			require.Nil(t, err)

			// send gc2 to node2, which will broadcast to other nodes synchronously
			err = node2.engine.SubmitGuaranteedCollection(gc2)
			require.Nil(t, err)

			// send gc3 to node3, which will broadcast to other nodes synchronously
			err = node3.engine.SubmitGuaranteedCollection(gc3)
			require.Nil(t, err)

			// now, check that all 3 nodes should have the same mempool state

			// check mempool should have all 3 collection hashes
			require.Equal(t, 3, int(node1.pool.Size()))

			// check mempool hash are the same
			require.Equal(t, node1.pool.Hash(), node2.pool.Hash())
			require.Equal(t, node1.pool.Hash(), node3.pool.Hash())
		})
}
