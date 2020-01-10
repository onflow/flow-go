package tests

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// If a consensus node receives a collection hash, then another connected node should receive it as well.
func TestSubmitCollectionOneToOne(t *testing.T) {

	// create a mocked network for each node and connect them in a in-memory hub, so that events sent from one engine
	// can be delivered directly to another engine on a different node
	_, nodes, err := createConnectedNodes(
		fmt.Sprintf("consensus-%s@address1=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%s@address2=1000", unittest.IdentifierFixture()),
	)
	require.Nil(t, err)

	node1 := nodes[0]
	node2 := nodes[1]

	// prepare a random collection hash
	guarantee := unittest.CollectionGuaranteeFixture()

	// node1's engine receives a collection hash
	_ = node1.engine.ProcessLocal(guarantee)
	node1.net.FlushAll()

	// inspect node2's mempool to check if node2's engine received the collection hash
	coll, err := node2.pool.Get(guarantee.ID())
	require.Nil(t, err)

	// should match
	require.Equal(t, coll.ID(), guarantee.ID())
	terminate(node1, node2)
}

// Verify the behavior property:
// If N nodes are connected together, then sending any distinct collection hash to any node will
// result all nodes receive all hashes, and their mempools should all produce the same hash.
func TestSubmitCollectionManyToManySynchronous(t *testing.T) {

	_, nodes, err := createConnectedNodes(
		fmt.Sprintf("consensus-%s@address1=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%s@address2=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%s@address3=1000", unittest.IdentifierFixture()),
	)
	require.Nil(t, err)

	// prepare 3 nodes that are connected to each other
	node1 := nodes[0]
	node2 := nodes[1]
	node3 := nodes[2]

	// prepare 3 different CollectionGuarantees: guarantee1, guarantee2, guarantee3
	guarantee1 := unittest.CollectionGuaranteeFixture()
	guarantee2 := unittest.CollectionGuaranteeFixture()
	guarantee3 := unittest.CollectionGuaranteeFixture()

	// check the collections are different
	require.NotEqual(t, guarantee1.ID(), guarantee2.ID())

	// send guarantee1 to node1, which will broadcast to other nodes synchronously
	_ = node1.engine.ProcessLocal(guarantee1)
	node1.net.FlushAll()

	// send guarantee2 to node2, which will broadcast to other nodes synchronously
	_ = node1.engine.ProcessLocal(guarantee2)
	node1.net.FlushAll()

	// send guarantee3 to node3, which will broadcast to other nodes synchronously
	_ = node3.engine.ProcessLocal(guarantee3)
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
		fmt.Sprintf("consensus-%s@address1=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%s@address2=1000", unittest.IdentifierFixture()),
		fmt.Sprintf("consensus-%s@address3=1000", unittest.IdentifierFixture()),
	)
	require.Nil(t, err)

	// prepare 3 nodes that are connected to each other
	node1 := nodes[0]
	node2 := nodes[1]
	node3 := nodes[2]

	// prepare 3 different CollectionGuarantees: guarantee1, guarantee2, guarantee3
	guarantee1 := unittest.CollectionGuaranteeFixture()
	guarantee2 := unittest.CollectionGuaranteeFixture()
	guarantee3 := unittest.CollectionGuaranteeFixture()

	// send different collection to different nodes concurrently
	var wg sync.WaitGroup
	wg.Add(1)
	go sendOne(node1, guarantee1, &wg)
	wg.Add(1)
	go sendOne(node2, guarantee2, &wg)
	wg.Add(1)
	go sendOne(node3, guarantee3, &wg)
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

	nodes, guarantees, err := prepareNodesAndCollections(5, 100)
	require.Nil(t, err)

	N := len(nodes)
	M := len(guarantees)
	t.Logf("preparing %v nodes to send %v messages concurrently", N, M)

	// send each collection concurrently to a random node
	var wg sync.WaitGroup
	for _, guarantee := range guarantees {
		wg.Add(1)
		randNodeIndex := rand.Intn(N)
		go sendOne(nodes[randNodeIndex], guarantee, &wg)
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
func sendOne(node *mockPropagationNode, guarantee *flow.CollectionGuarantee, wg *sync.WaitGroup) {
	_ = node.engine.ProcessLocal(guarantee)
	node.net.FlushAll()
	wg.Done()
}
