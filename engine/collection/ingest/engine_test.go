package ingest_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// Malformed, incomplete, unsigned, or otherwise invalid transactions should be
// detected and should result in an error.
func TestInvalidTransaction(t *testing.T) {

	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	identity := identities.Filter(filter.HasRole(flow.RoleCollection))[0]
	hub := stub.NewNetworkHub()

	node := testutil.CollectionNode(t, hub, identity, identities)
	defer node.Done()

	genesis, err := node.State.Final().Head()
	require.Nil(t, err)

	t.Run("missing field", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = genesis.ID()
		tx.Script = nil

		err := node.IngestionEngine.ProcessLocal(&tx)
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, ingest.IncompleteTransactionError{}))
		}
	})

	t.Run("invalid reference block ID", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = unittest.IdentifierFixture()

		err := node.IngestionEngine.ProcessLocal(&tx)
		t.Log(err)
		assert.True(t, errors.Is(err, ingest.ErrUnknownReferenceBlock))
	})

	t.Run("un-parseable script", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = genesis.ID()
		tx.Script = []byte("definitely a real transaction")

		err := node.IngestionEngine.ProcessLocal(&tx)
		t.Log(err)
		assert.True(t, errors.Is(err, ingest.InvalidScriptError{}))
	})

	t.Run("invalid signature", func(t *testing.T) {
		// TODO cannot check signatures in MVP
		t.Skip()
	})

	t.Run("expired reference block ID", func(t *testing.T) {
		util.RunWithStorageLayer(t, func(_ *badger.DB, _ *storage.Headers, _ *storage.Identities, _ *storage.Guarantees, _ *storage.Seals, _ *storage.Payloads, blocks *storage.Blocks) {

			// build enough blocks to make genesis an expired reference
			parent := genesis
			for i := 0; i < flow.DefaultTransactionExpiry+1; i++ {
				next := unittest.BlockWithParentFixture(parent)
				err = node.State.Mutate().Extend(&next)
				require.Nil(t, err)
				err = node.State.Mutate().Finalize(next.ID())
				parent = next.Header
			}

			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = genesis.ID()

			err := node.IngestionEngine.ProcessLocal(&tx)
			t.Log(err)
			assert.True(t, errors.Is(err, ingest.ExpiredTransactionError{}))
		})
	})
}

// Transactions should be routed to the correct cluster and should not be
// routed unnecessarily.
func TestClusterRouting(t *testing.T) {

	const (
		nNodes    = 3
		nClusters = 3
	)

	t.Run("should store transaction for local cluster", func(t *testing.T) {
		hub := stub.NewNetworkHub()
		nodes := testutil.CollectionNodes(t, hub, nNodes, protocol.SetClusters(nClusters))
		for _, node := range nodes {
			defer node.Done()
		}

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]
		genesis, err := localNode.State.Final().Head()
		require.Nil(t, err)

		// get the list of clusters
		clusters, err := localNode.State.Final().Clusters()
		require.NoError(t, err)

		// set target cluster to the local cluster
		localCluster, ok := clusters.ByNodeID(localNode.Me.NodeID())
		require.True(t, ok)

		// get a transaction that will be routed to local
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = genesis.ID()
		tx = unittest.AlterTransactionForCluster(tx, clusters, localCluster, func(transaction *flow.TransactionBody) {})

		// submit transaction locally to test storing
		err = localNode.IngestionEngine.ProcessLocal(&tx)
		assert.Nil(t, err)

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
		for _, node := range nodes {
			defer node.Done()
		}

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		genesis, err := localNode.State.Final().Head()
		require.Nil(t, err)

		// get the list of clusters
		clusters, err := localNode.State.Final().Clusters()
		require.NoError(t, err)

		// set target cluster to remote cluster
		remoteCluster, ok := clusters.ByNodeID(remoteNode.Me.NodeID())
		require.True(t, ok)

		// get a transaction that will be routed to the target cluster
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = genesis.ID()
		tx = unittest.AlterTransactionForCluster(tx, clusters, remoteCluster, func(*flow.TransactionBody) {})

		// submit transaction locally to test propagation
		err = localNode.IngestionEngine.ProcessLocal(&tx)
		assert.Nil(t, err)

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
		for _, node := range nodes {
			defer node.Done()
		}

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		genesis, err := localNode.State.Final().Head()
		require.Nil(t, err)

		// get the list of clusters
		clusters, err := localNode.State.Final().Clusters()
		require.Nil(t, err)

		// set target cluster to remote cluster
		targetCluster, ok := clusters.ByNodeID(remoteNode.Me.NodeID())
		require.True(t, ok)

		// get a transaction that will be routed to remote cluster
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = genesis.ID()
		tx = unittest.AlterTransactionForCluster(tx, clusters, targetCluster, func(*flow.TransactionBody) {})

		// submit transaction with remote origin to test non-propagation
		err = localNode.IngestionEngine.Process(remoteNode.Me.NodeID(), &tx)
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
		for _, node := range nodes {
			defer node.Done()
		}

		// name the various nodes
		localNode, remoteNode, noopNode := nodes[0], nodes[1], nodes[2]

		// get the list of clusters
		clusters, err := localNode.State.Final().Clusters()
		require.Nil(t, err)

		// set the target cluster to local cluster
		targetCluster, ok := clusters.ByNodeID(localNode.Me.NodeID())
		require.True(t, ok)

		// get transaction for target cluster, but make it invalid
		tx := unittest.TransactionBodyFixture()
		tx = unittest.AlterTransactionForCluster(tx, clusters, targetCluster, func(*flow.TransactionBody) {})

		// submit transaction locally (should not be relevant)
		err = localNode.IngestionEngine.ProcessLocal(&tx)
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
