package tests

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// If a consensus node receives a collection hash, then another connected node should receive it as well.
func TestSubmitCollectionOneToOne(t *testing.T) {

	// create a mocked network for each node and connect them in a in-memory hub, so that events sent from one engine
	// can be delivered directly to another engine on a different node
	_, nodes, err := createConnectedNodes(
		fmt.Sprintf("consensus-%x@address1=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%x@address2=1000", unittest.IdentifierFixture()),
	)
	require.Nil(t, err)

	node1 := nodes[0]
	node2 := nodes[1]

	// prepare a random collection hash
	gc := randCollection()

	// node1's engine receives a collection hash
	_ = node1.engine.ProcessLocal(gc)
	node1.net.FlushAll()

	// inspect node2's mempool to check if node2's engine received the collection hash
	coll, err := node2.pool.Get(gc.Hash())
	require.Nil(t, err)

	// should match
	require.Equal(t, coll.Hash(), gc.Hash())
	terminate(node1, node2)
}

// Verify the behavior property:
// If N nodes are connected together, then sending any distinct collection hash to any node will
// result all nodes receive all hashes, and their mempools should all produce the same hash.
func TestSubmitCollectionManyToManySynchronous(t *testing.T) {

	_, nodes, err := createConnectedNodes(
		fmt.Sprintf("consensus-%x@address1=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%x@address2=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%x@address3=1000", unittest.IdentifierFixture()),
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
	require.NotEqual(t, gc1.Hash(), gc2.Hash())

	// send gc1 to node1, which will broadcast to other nodes synchronously
	_ = node1.engine.ProcessLocal(gc1)
	node1.net.FlushAll()

	// send gc2 to node2, which will broadcast to other nodes synchronously
	_ = node1.engine.ProcessLocal(gc2)
	node1.net.FlushAll()

	// send gc3 to node3, which will broadcast to other nodes synchronously
	_ = node3.engine.ProcessLocal(gc3)
	node1.net.FlushAll()

	// now, check that all 3 nodes should have the same mempool state

	// check mempool should have all 3 collection hashes
	require.Equal(t, 3, int(node1.pool.Size()))

	// check mempool hash are the same
	require.Equal(t, node1.pool.Hash(), node2.pool.Hash())
	require.Equal(t, node1.pool.Hash(), node3.pool.Hash())

	terminate(node1, node2, node3)
}

func TestSubmitCollectionManyToManyAsynchronous(t *testing.T) {

	_, nodes, err := createConnectedNodes(
		fmt.Sprintf("consensus-%x@address1=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%x@address2=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%x@address3=1000", unittest.IdentifierFixture()),
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
}

func TestSubmitCollectionManyToManyRandom(t *testing.T) {

	nodes, gcs, err := prepareNodesAndCollectionsConfigurable(5, 100)
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
	// and the hashes are the same
	sameHash := nodes[0].pool.Hash()
	for _, node := range nodes {
		require.Equal(t, M, int(node.pool.Size()))
		require.Equal(t, sameHash, node.pool.Hash())
	}

	// terminates nodes
	terminate(nodes...)
}

// send one collection to one node.
// extracted in order to be reused in different tests
func sendOne(node *mockPropagationNode, gc *collection.GuaranteedCollection, wg *sync.WaitGroup) {
	_ = node.engine.ProcessLocal(gc)
	node.net.FlushAll()
	wg.Done()
}
