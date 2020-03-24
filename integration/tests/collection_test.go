package tests

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dgraph-io/badger/v2"
	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clusterstate "github.com/dapperlabs/flow-go/cluster/badger"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/protocol"
)

func TestCollection(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
		})
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
		})
		conNode = testnet.NewNodeConfig(flow.RoleConsensus)
		exeNode = testnet.NewNodeConfig(flow.RoleExecution)
		verNode = testnet.NewNodeConfig(flow.RoleVerification)
	)

	nodes := []*testnet.NodeConfig{colNode1, colNode2, conNode, exeNode, verNode}

	testingdock.Verbose = true

	ctx := context.Background()

	net, err := testnet.PrepareFlowNetwork(ctx, t, "col", nodes)
	require.Nil(t, err)

	net.Start(ctx)
	defer net.Cleanup()

	// get the collection node container
	colContainer, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	// get the node's ingress port and create an RPC client
	ingressPort, ok := colContainer.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)
	client, err := client.New(fmt.Sprintf(":%s", ingressPort))
	assert.Nil(t, err)

	sdkTx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		Nonce:              rand.Uint64(),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
	}
	err = client.SendTransaction(ctx, sdkTx)
	assert.Nil(t, err)

	// give the cluster a chance to do some consensus
	time.Sleep(10 * time.Second)
	err = net.StopContainers()
	assert.Nil(t, err)

	identities := net.Identities()

	// create a database
	chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))
	db, err := badger.Open(badger.DefaultOptions(colContainer.DataDir).WithLogger(nil))
	require.Nil(t, err)

	state, err := clusterstate.NewState(db, chainID)
	assert.Nil(t, err)
	head, err := state.Final().Head()
	assert.Nil(t, err)

	// should be able to read a valid latest block
	assert.Equal(t, chainID, head.ChainID)
}
