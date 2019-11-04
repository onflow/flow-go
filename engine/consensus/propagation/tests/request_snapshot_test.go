package tests

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/collection"
)

func TestPropagateSnapshotRequest(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	// If a propagation engine found its peer has different snapshot state from its peer, it should pull the missing
	// collections from that peer and after that both of them should be in sync.
	t.Run("should pull missing collection hashes from its peers", func(t *testing.T) {
		// create a mocked network for each node and connect them in a in-memory hub, so that events sent from one engine
		// can be delivery directly to another engine on a different node
		_, nodes, err := createConnectedNodes("consensus-consensus1@localhost:7297", "consensus-consensus2@localhost:7298")
		require.Nil(t, err)

		node1 := nodes[0]
		node2 := nodes[1]

		// prepare a random collection hash
		gc := randCollection()

		// should be in sync at the beginning
		require.Equal(t, node1.pool.Hash(), node2.pool.Hash())

		// node1's engine receives a collection hash
		node1.engine.Submit(gc)

		// block the forwarded updates to its peer
		node1.net.FlushUnblocked(blockGuaranteedCollection)

		// should get out of sync
		require.NotEqual(t, node1.pool.Hash(), node2.pool.Hash())

		// forcing to check if there is missing updates
		err = node2.engine.PropagateSnapshotRequest()
		require.Nil(t, err)

		// flush all but still block the push updates
		node1.net.FlushUnblocked(blockGuaranteedCollection)

		// should be in sync
		require.Equal(t, node1.pool.Hash(), node2.pool.Hash())

		// clean up
		terminate(node1, node2)
	})

	t.Run("fuzzy test once that PropagateSnapshotRequest can keep node stay in sync", fuzzyTestPropagationSnapshotRequest)
	t.Run("fuzzy test 100 times that PropagateSnapshotRequest can keep node stay in sync", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			fuzzyTestPropagationSnapshotRequest(t)
		}
	})
}

func sendOneAndBlockBroadcast(node *mockPropagationNode, gc *collection.GuaranteedCollection, wg *sync.WaitGroup) {
	node.engine.Submit(gc)
	node.net.FlushUnblocked(blockGuaranteedCollection)
	node.engine.PropagateSnapshotRequest()
	node.net.FlushUnblocked(blockGuaranteedCollection)
	wg.Done()
}

func fuzzyTestPropagationSnapshotRequest(t *testing.T) {
	nodes, gcs, err := prepareNRandomNodesAndMRandomCollectionHashes()
	require.Nil(t, err)

	N := len(nodes)
	M := len(gcs)
	t.Logf("preparing %v nodes to send %v messages concurrently", N, M)

	// send each collection concurrently to a random node
	var allSent sync.WaitGroup
	for _, gc := range gcs {
		allSent.Add(1)
		randNodeIndex := rand.Intn(N)
		go sendOneAndBlockBroadcast(nodes[randNodeIndex], gc, &allSent)
	}
	allSent.Wait()

	// each node will check missing collection from its peer and fetch missed collections
	var allSynced sync.WaitGroup
	for _, node := range nodes {
		allSynced.Add(1)
		go func(node *mockPropagationNode) {
			err = node.engine.PropagateSnapshotRequest()
			require.Nil(t, err)

			// flush all but still block the push updates
			node.net.FlushUnblocked(blockGuaranteedCollection)
			allSynced.Done()
		}(node)
	}
	allSynced.Wait()

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
