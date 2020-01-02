package ingest_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// Malformed, incomplete, unsigned, or otherwise invalid transactions should be
// detected and should result in an error.
func TestInvalidTransaction(t *testing.T) {
	identity := unittest.IdentityFixture()
	identity.Role = flow.RoleCollection
	hub := stub.NewNetworkHub()

	t.Run("missing field", func(t *testing.T) {
		genesis := mock.Genesis(flow.IdentityList{identity})
		node := testutil.CollectionNode(t, hub, identity, genesis)

		tx := unittest.TransactionFixture()
		tx.Script = nil

		err := node.IngestionEngine.Process(node.Me.NodeID(), &tx)
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, ingest.ErrIncompleteTransaction{}))
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		// TODO cannot check signatures in MVP
		t.Skip()
	})
}

// Transactions should be routed to the correct cluster and should not be
// routed unnecessarily.
func TestClusterRouting(t *testing.T) {
	// number of nodes and number of clusters
	const N = 3

	t.Run("should not route transactions for my cluster", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, N)

		state := nodes[0].State
		identities, err := state.AtNumber(0).Identities()
		require.NoError(t, err)

		clusters := protocol.Cluster(identities)

		// the node we will send transactions to
		ingressNode := nodes[0]
		// the transaction will target the ingress node's cluster
		targetCluster := clusters.ClusterIDFor(ingressNode.Me.NodeID())

		// the other nodes
		otherNode1, otherNode2 := nodes[1], nodes[2]

		// get a transaction that will be routed to the target cluster
		tx := testutil.TransactionForCluster(N, targetCluster)

		err = ingressNode.IngestionEngine.Process(ingressNode.Me.NodeID(), tx)
		assert.NoError(t, err)

		// transaction should be in target cluster's pool, not in other pool
		assert.EqualValues(t, 1, ingressNode.Pool.Size())
		assert.EqualValues(t, 0, otherNode1.Pool.Size())
		assert.EqualValues(t, 0, otherNode2.Pool.Size())
	})

	t.Run("should route transactions for a different cluster", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, N)

		state := nodes[0].State
		identities, err := state.AtNumber(0).Identities()
		require.NoError(t, err)

		clusters := protocol.Cluster(identities)

		// the node we will send transactions to
		ingressNode := nodes[0]

		// the node in the target cluster
		targetNode := nodes[1]
		targetCluster := clusters.ClusterIDFor(targetNode.Me.NodeID())

		otherNode := nodes[2]

		// get a transaction that will be routed to the target cluster
		tx := testutil.TransactionForCluster(N, targetCluster)

		err = ingressNode.IngestionEngine.Process(ingressNode.Me.NodeID(), tx)
		assert.NoError(t, err)

		// flush messages from the ingress node
		net, ok := hub.GetNetwork(targetNode.Me.NodeID())
		require.True(t, ok)
		net.FlushAll()

		// transaction should be in target cluster's pool, not in other pool
		assert.EqualValues(t, 0, ingressNode.Pool.Size())
		assert.EqualValues(t, 1, targetNode.Pool.Size())
		assert.EqualValues(t, 0, otherNode.Pool.Size())
	})

	t.Run("should not route invalid transactions", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, N)

		ingressNode := nodes[0]
		otherNode1, otherNode2 := nodes[1], nodes[2]

		// get an invalid transaction
		tx := unittest.TransactionFixture()
		tx.Script = nil

		err := ingressNode.IngestionEngine.Process(ingressNode.Me.NodeID(), tx)
		assert.Error(t, err)

		// the transaction should not be stored in the ingress, nor routed
		assert.EqualValues(t, 0, ingressNode.Pool.Size())
		assert.EqualValues(t, 0, otherNode1.Pool.Size())
		assert.EqualValues(t, 0, otherNode2.Pool.Size())
	})
}
