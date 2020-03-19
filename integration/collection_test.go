package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dgraph-io/badger/v2"
	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clusterstate "github.com/dapperlabs/flow-go/cluster/badger"
	"github.com/dapperlabs/flow-go/integration/network"
	cluster "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

func TestCollection(t *testing.T) {

	var (
		colNode1 = network.NewNodeConfig(flow.RoleCollection)
		colNode2 = network.NewNodeConfig(flow.RoleCollection)
		conNode  = network.NewNodeConfig(flow.RoleConsensus)
		exeNode  = network.NewNodeConfig(flow.RoleExecution)
		verNode  = network.NewNodeConfig(flow.RoleVerification)
	)

	nodes := []*network.NodeConfig{colNode1, colNode2, conNode, exeNode, verNode}

	testingdock.Verbose = true

	ctx := context.Background()

	net, err := network.PrepareFlowNetwork(ctx, t, "col", nodes)
	require.Nil(t, err)

	net.Start(ctx)
	defer net.Stop()

	// get the collection node container
	colContainer, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	// get the node's ingress port and create an RPC client
	ingressPort, ok := colContainer.Ports[network.IngressApiPort]
	assert.True(t, ok)
	client, err := client.New(fmt.Sprintf(":%s", ingressPort))
	assert.Nil(t, err)

	sdkTx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
	}
	err = client.SendTransaction(ctx, sdkTx)
	assert.Nil(t, err)

	identities := net.Identities()

	// create a database
	db, err := badger.Open(badger.DefaultOptions(colContainer.DataDir).WithLogger(nil))
	require.Nil(t, err)

	// eventually the transaction should be included in the storage
	assert.Eventually(t, func() bool {
		chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))

		state, err := clusterstate.NewState(db, chainID)
		assert.Nil(t, err)
		head, err := state.Final().Head()
		assert.Nil(t, err)

		fmt.Println(">>>> cluster id: ", chainID)
		fmt.Println(">>>> height: ", head.Height)
		fmt.Println(">>>> id: ", head.ID())
		var payload cluster.Payload
		err = db.View(procedure.RetrieveClusterPayload(head.ID(), &payload))
		assert.Nil(t, err)
		fmt.Println(">>>> payload size: ", len(payload.Collection.Transactions))

		return len(payload.Collection.Transactions) > 0
	}, 10*time.Second, time.Second)

}
