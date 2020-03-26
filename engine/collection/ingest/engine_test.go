package ingest_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// Malformed, incomplete, unsigned, or otherwise invalid transactions should be
// detected and should result in an error.
func TestInvalidTransaction(t *testing.T) {
	identity := unittest.IdentityFixture()
	identity.Role = flow.RoleCollection
	hub := stub.NewNetworkHub()

	t.Run("missing field", func(t *testing.T) {
		node := testutil.CollectionNode(t, hub, identity, []*flow.Identity{identity})

		tx := unittest.TransactionBodyFixture()
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
	// number of nodes
	const nNodes = 3

	// number of clusters
	const nClusters = 3

	t.Run("should store transaction for local cluster", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, nNodes, protocol.SetClusters(nClusters))

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		// get the list of clusters
		clusters, err := localNode.State.AtNumber(0).Clusters()
		require.NoError(t, err)

		// set target cluster to the local cluster
		localCluster, ok := clusters.ByNodeID(localNode.Me.NodeID())
		require.True(t, ok)

		// get a transaction that will be routed to local
		tx := testutil.TransactionForCluster(clusters, localCluster)

		// submit transaction locally to test storing
		err = localNode.IngestionEngine.ProcessLocal(tx)
		assert.NoError(t, err)

		// flush the network to make sure all messages are sent
		net, _ := hub.GetNetwork(localNode.Me.NodeID())
		net.DeliverAll(false)

		// transaction should be in target cluster's pool, not in other pool
		assert.EqualValues(t, 1, localNode.Pool.Size())
		assert.EqualValues(t, 0, remoteNode.Pool.Size())
		assert.EqualValues(t, 0, noopNode.Pool.Size())
	})

	t.Run("should propagate locally submitted transaction", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, nNodes, protocol.SetClusters(nClusters))

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		// get the list of clusters
		clusters, err := localNode.State.AtNumber(0).Clusters()
		require.NoError(t, err)

		// set target cluster to remote cluster
		remoteCluster, ok := clusters.ByNodeID(remoteNode.Me.NodeID())
		require.True(t, ok)

		// get a transaction that will be routed to the target cluster
		tx := testutil.TransactionForCluster(clusters, remoteCluster)

		// submit transaction locally to test propagation
		err = localNode.IngestionEngine.ProcessLocal(tx)
		assert.NoError(t, err)

		// flush the network to make sure all messages are sent
		net, _ := hub.GetNetwork(localNode.Me.NodeID())
		net.DeliverAll(true)

		// transaction should be in target cluster's pool, not in other pool
		assert.EqualValues(t, 0, localNode.Pool.Size())
		assert.EqualValues(t, 1, remoteNode.Pool.Size())
		assert.EqualValues(t, 0, noopNode.Pool.Size())
	})

	t.Run("should not propagate remotely submitted transaction", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, nNodes, protocol.SetClusters(nClusters))

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		// get the list of clusters
		clusters, err := localNode.State.AtNumber(0).Clusters()
		require.NoError(t, err)

		// set target cluster to remote cluster
		targetCluster, ok := clusters.ByNodeID(remoteNode.Me.NodeID())
		require.True(t, ok)

		// get a transaction that will be routed to remote cluster
		tx := testutil.TransactionForCluster(clusters, targetCluster)

		// submit transaction with remote origin to test non-propagation
		err = localNode.IngestionEngine.Process(remoteNode.Me.NodeID(), tx)
		assert.NoError(t, err)

		// flush the network to make sure all messages are sent
		net, _ := hub.GetNetwork(localNode.Me.NodeID())
		net.DeliverAll(false)

		// transaction should not be in any pool
		assert.EqualValues(t, 0, localNode.Pool.Size())
		assert.EqualValues(t, 0, remoteNode.Pool.Size())
		assert.EqualValues(t, 0, noopNode.Pool.Size())
	})

	t.Run("should not process invalid transaction", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, nNodes, protocol.SetClusters(nClusters))

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		// get the list of clusters
		clusters, err := localNode.State.AtNumber(0).Clusters()
		require.NoError(t, err)

		// set the target cluster to local cluster
		targetCluster, ok := clusters.ByNodeID(localNode.Me.NodeID())
		require.True(t, ok)

		// get transaction for target cluster, but make it invalid
		tx := testutil.TransactionForCluster(clusters, targetCluster)
		tx.Script = nil

		// submit transaction locally (should not be relevant)
		err = localNode.IngestionEngine.ProcessLocal(tx)
		assert.Error(t, err)

		// flush the network to make sure all messages are sent
		net, _ := hub.GetNetwork(localNode.Me.NodeID())
		net.DeliverAll(false)

		// the transaction should not be stored in the ingress, nor routed
		assert.EqualValues(t, 0, localNode.Pool.Size())
		assert.EqualValues(t, 0, remoteNode.Pool.Size())
		assert.EqualValues(t, 0, noopNode.Pool.Size())
	})
}
