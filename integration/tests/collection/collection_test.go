package collection

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// default set of non-collection nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel("info"))
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel("info"))
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, exeNode, verNode}
}

func TestConsistentGenesis(t *testing.T) {

	t.Skip()

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
		})
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
		})
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2}, defaultOtherNodes()...)
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "col", conf)
	require.Nil(t, err)

	net.Start(context.Background())
	defer net.Cleanup()

	// get the collection node containers
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)
	colContainer2, ok := net.ContainerByID(colNode2.Identifier)
	assert.True(t, ok)

	//// get the node's ingress port and create an RPC client
	//ingressPort, ok := colContainer.Ports[testnet.ColNodeAPIPort]
	//assert.True(t, ok)
	//
	//key, err := testutil.AccountKeyFixture()
	//assert.Nil(t, err)
	//client, err := testnet.NewClientWithKey(fmt.Sprintf(":%s", ingressPort), key)
	//assert.Nil(t, err)
	//
	//err = client.SendTransactionDSL(ctx, testutil.NoopTransactionDSL())
	//assert.Nil(t, err)

	// give the cluster a chance to do some consensus
	time.Sleep(10 * time.Second)
	err = net.StopContainers()
	assert.Nil(t, err)

	identities := net.Identities()

	chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))

	// create a database for col1 and col2
	db1, err := colContainer1.DB()
	require.Nil(t, err)
	db2, err := colContainer2.DB()
	require.Nil(t, err)

	// get cluster state for col1 and col2
	state1, err := clusterstate.NewState(db1, chainID)
	assert.Nil(t, err)
	state2, err := clusterstate.NewState(db2, chainID)
	assert.Nil(t, err)

	// get chain head for col1 and col2
	head1, err := state1.Final().Head()
	assert.Nil(t, err)
	head2, err := state2.Final().Head()
	assert.Nil(t, err)

	// the head should be either equal, or at most off by one
	assert.True(t, math.Abs(float64(head1.Height-head2.Height)) < 2)
	t.Logf("COL1 height: %d\tCOL2 height: %d\n", head1.Height, head2.Height)

}

func TestTransactionIngress_InvalidTransaction(t *testing.T) {
	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
		})
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
		})
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000003")
		})
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2, colNode3}, defaultOtherNodes()...)
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "col", conf)
	require.Nil(t, err)

	net.Start(context.Background())
	defer net.Cleanup()

	// we will test against COL1
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	port, ok := colContainer1.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	client, err := testnet.NewClient(fmt.Sprintf(":%s", port))
	assert.Nil(t, err)

	t.Run("missing reference block hash", func(t *testing.T) {
		txDSL := unittest.TransactionDSLFixture()
		malformed := unittest.TransactionBodyFixture(unittest.WithTransactionDSL(txDSL))
		malformed.ReferenceBlockID = flow.ZeroID

		err := client.SignAndSendTransaction(context.Background(), malformed)
		assert.Error(t, err)
	})

	t.Run("missing script", func(t *testing.T) {
		malformed := unittest.TransactionBodyFixture()
		malformed.Script = nil

		err := client.SignAndSendTransaction(context.Background(), malformed)
		assert.Error(t, err)
	})

	t.Run("unparseable script", func(t *testing.T) {
		// TODO script parsing not implemented
		t.Skip()
	})
	t.Run("invalid signature", func(t *testing.T) {
		// TODO signature validation not implemented
		t.Skip()
	})
	t.Run("invalid nonce", func(t *testing.T) {
		// TODO nonce validation not implemented
		t.Skip()
	})
	t.Run("insufficient payer balance", func(t *testing.T) {
		// TODO balance checking not implemented
		t.Skip()
	})
	t.Run("expired transaction", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("non-existent reference block ID", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
}

func TestTransactionIngress_ValidTransaction(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
		})
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
		})
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000003")
		})
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2, colNode3}, defaultOtherNodes()...)
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "col", conf)
	require.Nil(t, err)

	net.Start(context.Background())
	defer net.Cleanup()

	// we will test against COL1
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	port, ok := colContainer1.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	client, err := testnet.NewClient(fmt.Sprintf(":%s", port))

	t.Run("valid transaction", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx, err := client.SignTransaction(tx)
		assert.Nil(t, err)
		t.Log("sending transaction: ", tx.ID())

		err = client.SendTransaction(context.Background(), tx)
		assert.Nil(t, err)

		// wait for consensus to complete
		time.Sleep(10 * time.Second)

		// TODO stop then start containers
		err = net.StopContainers()
		assert.Nil(t, err)

		identities := net.Identities()

		chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))

		// get database for COL1
		db, err := colContainer1.DB()
		require.Nil(t, err)

		state, err := clusterstate.NewState(db, chainID)
		assert.Nil(t, err)

		// the transaction should be included in exactly one collection
		head, err := state.Final().Head()
		assert.Nil(t, err)

		foundTx := false
		for head.Height > 0 {
			collection, err := state.AtBlockID(head.ID()).Collection()
			assert.Nil(t, err)

			head, err = state.AtBlockID(head.ParentID).Head()
			assert.Nil(t, err)

			if collection.Len() == 0 {
				continue
			}

			for _, txID := range collection.Transactions {
				assert.Equal(t, tx.ID(), txID, "found unexpected transaction")
				if txID == tx.ID() {
					assert.False(t, foundTx, "found duplicate transaction")
					foundTx = true
				}
			}
		}

		assert.True(t, foundTx)
	})
}
