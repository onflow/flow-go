package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/convert"
	"github.com/dgraph-io/badger/v2"
	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/network"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/ingress"
	storage "github.com/dapperlabs/flow-go/storage/badger"
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
	tx, err := ingress.MessageToTransaction(convert.TransactionToMessage(sdkTx))
	assert.Nil(t, err)

	// create a database
	db, err := badger.Open(badger.DefaultOptions(colContainer.DataDir).WithLogger(nil))
	require.Nil(t, err)

	// eventually the transaction should be included in the storage
	assert.Eventually(t, func() bool {
		transactions := storage.NewTransactions(db)
		_, err := transactions.ByID(tx.ID())
		return err == nil
	}, 10*time.Second, 500*time.Millisecond)

}
