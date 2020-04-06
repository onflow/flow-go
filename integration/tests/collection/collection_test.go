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
	"github.com/dapperlabs/flow-go/integration/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// default set of non-collection nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus)
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus)
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus)
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution)
		verNode  = testnet.NewNodeConfig(flow.RoleVerification)
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, exeNode, verNode}
}

func TestConsistentGenesis(t *testing.T) {

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
	//key, err := testutil.RandomAccountKey()
	//assert.Nil(t, err)
	//client, err := testnet.NewClient(fmt.Sprintf(":%s", ingressPort), key)
	//assert.Nil(t, err)
	//
	//err = client.SendTransaction(ctx, testutil.NoopTransaction())
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

func TestTransactionIngress_SingleCluster(t *testing.T) {
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

	key, err := testutil.RandomAccountKey()
	assert.Nil(t, err)
	client, err := testnet.NewClient(fmt.Sprintf(":%s", port), key)

	t.Run("malformed transaction", func(t *testing.T) {
		t.Run("missing reference block hash", func(t *testing.T) {
			malformed := flow.TransactionBody{
				ReferenceBlockID: flow.Identifier{}, // missing
				Script:           []byte(testutil.NoopTransaction().ToCadence()),
				ComputeLimit:     10,
				PayerAccount:     flow.RootAddress,
			}

			err := client.SendTransactionManual(context.Background(), malformed)
			assert.Error(t, err)
		})

		t.Run("missing script", func(t *testing.T) {

		})
	})
}
