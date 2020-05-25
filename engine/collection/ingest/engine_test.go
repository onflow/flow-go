package ingest_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type InvalidTransactionSuite struct {
	suite.Suite
	node    mock.CollectionNode
	genesis *flow.Header
}

func TestInvalidTransactions(t *testing.T) {
	suite.Run(t, new(InvalidTransactionSuite))
}

func (suite *InvalidTransactionSuite) SetupTest() {
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	identity := identities.Filter(filter.HasRole(flow.RoleCollection))[0]
	hub := stub.NewNetworkHub()

	suite.node = testutil.CollectionNode(suite.T(), hub, identity, identities)

	genesis, err := suite.node.State.Final().Head()
	require.Nil(suite.T(), err)
	suite.genesis = genesis
}

func (suite *InvalidTransactionSuite) TearDownSuite() {
	suite.node.Done()
}

func (suite *InvalidTransactionSuite) TestMissingField() {
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.genesis.ID()
	tx.Script = nil

	err := suite.node.IngestionEngine.ProcessLocal(&tx)
	if assert.Error(suite.T(), err) {
		assert.True(suite.T(), errors.Is(err, ingest.IncompleteTransactionError{}))
	}
}

func (suite *InvalidTransactionSuite) TestGasLimit() {
	tx := unittest.TransactionBodyFixture()
	tx.GasLimit = flow.DefaultGasLimit + 1

	err := suite.node.IngestionEngine.ProcessLocal(&tx)
	if assert.Error(suite.T(), err) {
		assert.True(suite.T(), errors.Is(err, ingest.GasLimitExceededError{}))
	}
}

func (suite *InvalidTransactionSuite) SetIngestConfig(conf ingest.Config) {
	var err error
	suite.node.IngestionEngine, err = ingest.New(suite.node.Log, suite.node.Net, suite.node.State, suite.node.Metrics, suite.node.Metrics, suite.node.Me, suite.node.Pool, conf)
	require.Nil(suite.T(), err)
}

func (suite *InvalidTransactionSuite) TestUnknownReferenceBlockID() {

	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = unittest.IdentifierFixture()

	suite.Run("allowing unknown reference block", func() {
		// set the config to allow unknown references
		conf := ingest.DefaultConfig()
		conf.AllowUnknownReference = true
		suite.SetIngestConfig(conf)

		err := suite.node.IngestionEngine.ProcessLocal(&tx)
		assert.Nil(suite.T(), err)
	})

	suite.Run("dis-allowing unknown reference block", func() {
		// set the config to disallow unknown references
		conf := ingest.DefaultConfig()
		conf.AllowUnknownReference = false
		suite.SetIngestConfig(conf)

		err := suite.node.IngestionEngine.ProcessLocal(&tx)
		assert.True(suite.T(), errors.Is(err, ingest.ErrUnknownReferenceBlock))
	})
}

func (suite *InvalidTransactionSuite) TestUnparseableScript() {

	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.genesis.ID()
	tx.Script = []byte("definitely a real transaction")

	suite.Run("checking scripts enabled", func() {
		// set the config to enable script checking
		conf := ingest.DefaultConfig()
		conf.CheckScriptsParse = true
		suite.SetIngestConfig(conf)

		err := suite.node.IngestionEngine.ProcessLocal(&tx)
		assert.True(suite.T(), errors.Is(err, ingest.InvalidScriptError{}))
	})

	suite.Run("checking scripts disabled", func() {
		// set the config to disable script checking
		conf := ingest.DefaultConfig()
		conf.CheckScriptsParse = false
		suite.SetIngestConfig(conf)

		err := suite.node.IngestionEngine.ProcessLocal(&tx)
		assert.Nil(suite.T(), err)
	})
}

func (suite *InvalidTransactionSuite) TestSignature() {
	// TODO cannot check signatures in MVP
	suite.T().SkipNow()
}

func (suite *InvalidTransactionSuite) TestExpiredReferenceBlock() {
	util.RunWithStorageLayer(suite.T(), func(_ *badger.DB, _ *storage.Headers, _ *storage.Identities, _ *storage.Guarantees, _ *storage.Seals, _ *storage.Index, _ *storage.Payloads, blocks *storage.Blocks) {

		// build enough blocks to make genesis an expired reference
		parent := suite.genesis
		for i := 0; i < flow.DefaultTransactionExpiry+1; i++ {
			next := unittest.BlockWithParentFixture(parent)
			next.Payload.Guarantees = nil
			next.Header.PayloadHash = next.Payload.Hash()
			err := suite.node.State.Mutate().Extend(&next)
			require.Nil(suite.T(), err)
			err = suite.node.State.Mutate().Finalize(next.ID())
			parent = next.Header
		}

		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.genesis.ID()

		err := suite.node.IngestionEngine.ProcessLocal(&tx)
		assert.True(suite.T(), errors.Is(err, ingest.ExpiredTransactionError{}))
	})
}

//
//// Malformed, incomplete, unsigned, or otherwise invalid transactions should be
//// detected and should result in an error.
//func TestInvalidTransaction(t *testing.T) {
//
//	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
//	identity := identities.Filter(filter.HasRole(flow.RoleCollection))[0]
//	hub := stub.NewNetworkHub()
//
//	node := testutil.CollectionNode(t, hub, identity, identities)
//	defer node.Done()
//
//	genesis, err := node.State.Final().Head()
//	require.Nil(t, err)
//
//	t.Run("missing field", func(t *testing.T) {
//		tx := unittest.TransactionBodyFixture()
//		tx.ReferenceBlockID = genesis.ID()
//		tx.Script = nil
//
//		err := node.IngestionEngine.ProcessLocal(&tx)
//		if assert.Error(t, err) {
//			assert.True(t, errors.Is(err, ingest.IncompleteTransactionError{}))
//		}
//	})
//
//	t.Run("gas limit exceeds the maximum allowed", func(t *testing.T) {
//		tx := unittest.TransactionBodyFixture()
//		tx.GasLimit = flow.DefaultGasLimit + 1
//
//		err := node.IngestionEngine.ProcessLocal(&tx)
//		if assert.Error(t, err) {
//			assert.True(t, errors.Is(err, ingest.GasLimitExceededError{}))
//		}
//	})
//
//	// check that reference block IDs are checked correctly based on configuration
//	t.Run("reference block ID", func(t *testing.T) {
//		tx := unittest.TransactionBodyFixture()
//		tx.ReferenceBlockID = unittest.IdentifierFixture()
//
//		t.Run("allowing unknown reference block", func(t *testing.T) {
//			// set the config to allow unknown references
//			conf := ingest.DefaultConfig()
//			conf.AllowUnknownReference = true
//			node.IngestionEngine, err = ingest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, node.Pool, conf)
//			require.Nil(t, err)
//
//			err := node.IngestionEngine.ProcessLocal(&tx)
//			assert.Nil(t, err)
//		})
//
//		t.Run("dis-allowing unknown reference block", func(t *testing.T) {
//			// set the config to disallow unknown references
//			conf := ingest.DefaultConfig()
//			conf.AllowUnknownReference = false
//			node.IngestionEngine, err = ingest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, node.Pool, conf)
//			require.Nil(t, err)
//
//			err := node.IngestionEngine.ProcessLocal(&tx)
//			t.Log(err)
//			assert.True(t, errors.Is(err, ingest.ErrUnknownReferenceBlock))
//		})
//	})
//
//	// check that scripts are checked correctly based on configuration
//	t.Run("un-parseable script", func(t *testing.T) {
//		tx := unittest.TransactionBodyFixture()
//		tx.ReferenceBlockID = genesis.ID()
//		tx.Script = []byte("definitely a real transaction")
//
//		t.Run("checking scripts enabled", func(t *testing.T) {
//			// set the config to enable script checking
//			conf := ingest.DefaultConfig()
//			conf.CheckScriptsParse = true
//			node.IngestionEngine, err = ingest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, node.Pool, conf)
//			require.Nil(t, err)
//
//			err := node.IngestionEngine.ProcessLocal(&tx)
//			t.Log(err)
//			assert.True(t, errors.Is(err, ingest.InvalidScriptError{}))
//		})
//
//		t.Run("checking scripts disabled", func(t *testing.T) {
//			// set the config to disable script checking
//			conf := ingest.DefaultConfig()
//			conf.CheckScriptsParse = false
//			node.IngestionEngine, err = ingest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, node.Pool, conf)
//			require.Nil(t, err)
//
//			err := node.IngestionEngine.ProcessLocal(&tx)
//			assert.Nil(t, err)
//		})
//	})
//
//	t.Run("invalid signature", func(t *testing.T) {
//		// TODO cannot check signatures in MVP
//		t.Skip()
//	})
//
//	t.Run("expired reference block ID", func(t *testing.T) {
//		util.RunWithStorageLayer(t, func(_ *badger.DB, _ *storage.Headers, _ *storage.Identities, _ *storage.Guarantees, _ *storage.Seals, _ *storage.Index, _ *storage.Payloads, blocks *storage.Blocks) {
//
//			// build enough blocks to make genesis an expired reference
//			parent := genesis
//			for i := 0; i < flow.DefaultTransactionExpiry+1; i++ {
//				next := unittest.BlockWithParentFixture(parent)
//				next.Payload.Guarantees = nil
//				next.Header.PayloadHash = next.Payload.Hash()
//				err = node.State.Mutate().Extend(&next)
//				require.Nil(t, err)
//				err = node.State.Mutate().Finalize(next.ID())
//				parent = next.Header
//			}
//
//			tx := unittest.TransactionBodyFixture()
//			tx.ReferenceBlockID = genesis.ID()
//
//			err := node.IngestionEngine.ProcessLocal(&tx)
//			t.Log(err)
//			assert.True(t, errors.Is(err, ingest.ExpiredTransactionError{}))
//		})
//	})
//}

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
