package tests

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/collection"
)

func TestSubmitCollection(t *testing.T) {

	// If a consensus node receives a collection hash, then another connected node should receive it as well.
	t.Run("should propagate collection to connected nodes", func(t *testing.T) {
		// create a mocked network for each node and connect them in a in-memory hub, so that events sent from one engine
		// can be delivered directly to another engine on a different node
		_, nodes, err := createConnectedNodes("consensus-consensus1@localhost:7297", "consensus-consensus2@localhost:7298")
		require.Nil(t, err)

		node1 := nodes[0]
		node2 := nodes[1]

		// prepare a random collection hash
		gc := randCollection()

		// node1's engine receives a collection hash
		node1.engine.Submit(gc)
		node1.net.FlushAll()

		// inspect node2's mempool to check if node2's engine received the collection hash
		coll, err := node2.pool.Get(gc.Hash)
		require.Nil(t, err)

		// should match
		require.Equal(t, coll.Hash, gc.Hash)
		terminate(node1, node2)
	})

	// Verify the behavior property:
	// If N nodes are connected together, then sending any distinct collection hash to any node will
	// result all nodes receive all hashes, and their mempools should all produce the same hash.
	t.Run("all nodes should have the same mempool state after exchanging received collections",
		func(t *testing.T) {
			_, nodes, err := createConnectedNodes(
				"consensus-consensus1@localhost:7297",
				"consensus-consensus2@localhost:7298",
				"consensus-consensus3@localhost:7299",
			)
			require.Nil(t, err)

			// prepare 3 nodes that are connected to each other
			node1 := nodes[0]
			node2 := nodes[1]
			node3 := nodes[2]

			// prepare 3 different GuaranteedCollections: gc1, gc2, gc3
			gc1 := randCollection()
			gc2 := randCollection()
			gc3 := randCollection()

			// check the collections are different
			require.NotEqual(t, gc1.Hash, gc2.Hash)

			// send gc1 to node1, which will broadcast to other nodes synchronously
			node1.engine.Submit(gc1)
			node1.net.FlushAll()

			// send gc2 to node2, which will broadcast to other nodes synchronously
			node2.engine.Submit(gc2)
			node1.net.FlushAll()

			// send gc3 to node3, which will broadcast to other nodes synchronously
			node3.engine.Submit(gc3)
			node1.net.FlushAll()

			// now, check that all 3 nodes should have the same mempool state

			// check mempool should have all 3 collection hashes
			require.Equal(t, 3, int(node1.pool.Size()))

			// check mempool hash are the same
			require.Equal(t, node1.pool.Hash(), node2.pool.Hash())
			require.Equal(t, node1.pool.Hash(), node3.pool.Hash())

			terminate(node1, node2, node3)
		})

	t.Run("should produce the same hash with concurrent calls", func(t *testing.T) {
		_, nodes, err := createConnectedNodes(
			"consensus-consensus1@localhost:7297",
			"consensus-consensus2@localhost:7298",
			"consensus-consensus3@localhost:7299",
		)
		require.Nil(t, err)

		// prepare 3 nodes that are connected to each other
		node1 := nodes[0]
		node2 := nodes[1]
		node3 := nodes[2]

		// prepare 3 different GuaranteedCollections: gc1, gc2, gc3
		gc1 := randCollection()
		gc2 := randCollection()
		gc3 := randCollection()

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

		terminate(node1, node2, node3)
	})

	testConcrrencyOnce := func(t *testing.T) {
		nodes, gcs, err := prepareNRandomNodesAndMRandomCollectionHashes()
		require.Nil(t, err)

		N := len(nodes)
		M := len(gcs)
		t.Logf("preparing %v nodes to send %v messages concurrently", N, M)

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

		// terminates nodes
		terminate(nodes...)
	}

	// N or M could be arbitarily big.
	t.Run("should produce the same hash for N nodes to send M messages concurrently to eath other",
		testConcrrencyOnce)

	// will take roughly 6 seconds
	t.Run("run the above tests for 100 times", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			testConcrrencyOnce(t)
			runtime.GC()
		}
	})
}

// send one collection to one node.
// extracted in order to be reused in different tests
func sendOne(node *mockPropagationNode, gc *collection.GuaranteedCollection, wg *sync.WaitGroup) {
	node.engine.Submit(gc)
	node.net.FlushAll()
	wg.Done()
}
