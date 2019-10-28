package tests

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
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

	// only log error logs
	// (but still printed some INFO logs, why?)
	log := zerolog.New(os.Stderr).With().Logger().Level(zerolog.ErrorLevel)

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

// send one collection to one node.
// extracted in order to be reused in different tests
func sendOne(node *mockPropagationNode, gc *collection.GuaranteedCollection, wg *sync.WaitGroup) {
	node.engine.SubmitGuaranteedCollection(gc)
	wg.Done()
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

	// Verify the behavior property:
	// If N nodes are connected together, then sending any distinct collection hash to any node will
	// result all nodes receive all hashes, and their mempools should all produce the same hash.
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

	t.Run("should produce the same hash with concurrent calls", func(t *testing.T) {
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

		// send different collection to different nodes concurrently
		var wg sync.WaitGroup
		wg.Add(1)
		go sendOne(node1, gc1, &wg)
		wg.Add(1)
		go sendOne(node2, gc2, &wg)
		wg.Add(1)
		go sendOne(node3, gc3, &wg)
		wg.Wait()

		// now, check that all 3 nodes should have the same mempool state

		// check mempool should have all 3 collection hashes
		require.Equal(t, 3, int(node1.pool.Size()))

		// check mempool hash are the same
		require.Equal(t, node1.pool.Hash(), node2.pool.Hash())
		require.Equal(t, node1.pool.Hash(), node3.pool.Hash())
	})

	testConcrrencyOnce := func(t *testing.T) {
		N := rand.Intn(100) + 1 // at least 1 node, at most 100 nodes
		M := rand.Intn(200) + 1 // at least 1 collection, at most 200 collections
		t.Logf("preparing %v nodes to send %v messages concurrently", N, M)

		// prepare N connected nodes
		entries := make([]string, N)
		for e := 0; e < N; e++ {
			entries[e] = fmt.Sprintf("consensus-consensus%v@localhost:10%v", e, e)
		}
		_, nodes, err := createConnectedNodes(entries)
		require.Nil(t, err)

		// prepare M distinct collection hashes
		gcs := make([]*collection.GuaranteedCollection, M)
		for m := 0; m < M; m++ {
			gc, err := randCollectionHash()
			require.Nil(t, err)
			gcs[m] = gc
		}

		// send each collection concurrently to a random node
		var wg sync.WaitGroup
		for _, gc := range gcs {
			wg.Add(1)
			randNodeIndex := rand.Intn(N)
			go sendOne(nodes[randNodeIndex], gc, &wg)
		}
		wg.Wait()

		// now check all nodes should have received M collections in their mempool
		// and the Hash are the same
		sameHash := nodes[0].pool.Hash()
		for _, node := range nodes {
			require.Equal(t, M, int(node.pool.Size()))
			require.Equal(t, sameHash, node.pool.Hash())
		}
	}

	// N or M could be arbitarily big.
	t.Run("should produce the same hash for N nodes to send M messages concurrently to eath other",
		testConcrrencyOnce)

	// will take roughly 15 seconds
	t.Run("run the above tests for 100 times", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			testConcrrencyOnce(t)
		}
	})
}
