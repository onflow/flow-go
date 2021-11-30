package common

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// These tests are just examples of how to use the ghost node in an integration tests.
// They do no test any functionality and hence are marked to be skipped

// TestGhostNodeExample_Subscribe demonstrates how to emulate a node and receive all inbound events for it
func TestGhostNodeExample_Subscribe(t *testing.T) {

	var (
		// one collection node
		collNode = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithIDInt(1))

		// three consensus nodes
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))

		// an actual execution node
		realExeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithIDInt(2))

		// a ghost node masquerading as an execution node
		ghostExeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithIDInt(3),
			testnet.AsGhost())

		// a verification node
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))

		accessNode = testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel))
	)

	nodes := append([]testnet.NodeConfig{collNode, conNode1, conNode2, conNode3, realExeNode, verNode, ghostExeNode, accessNode})
	conf := testnet.NewNetworkConfig("ghost_example_subscribe", nodes)

	net := testnet.PrepareFlowNetwork(t, conf)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Remove()

	// get the ghost container
	ghostContainer := net.ContainerByID(ghostExeNode.Identifier)

	// get a ghost client connected to the ghost node
	ghostClient, err := GetGhostClient(ghostContainer)
	assert.NoError(t, err)

	// subscribe to all the events the ghost execution node will receive
	msgReader, err := ghostClient.Subscribe(ctx)
	assert.NoError(t, err)

	// create and send a transaction to one of the collection node
	sendTransaction(t, net, collNode.Identifier)

	proposals := make([]*messages.BlockProposal, 0)

	for {
		_, event, err := msgReader.Next()
		assert.NoError(t, err)

		// the following switch should be similar to the one defined in the actual node that is being emulated
		switch v := event.(type) {
		case *messages.BlockProposal:
			proposals = append(proposals, v)
		default:
			t.Logf(" ignoring event: :%T: %v", v, v)
		}

		if len(proposals) == 2 {
			break
		}
	}

	assert.EqualValues(t, 1, proposals[0].Header.Height)
	assert.EqualValues(t, 2, proposals[1].Header.Height)
}

// TestGhostNodeExample_Send demonstrates how to emulate a node and send an event from it
func TestGhostNodeExample_Send(t *testing.T) {

	var (
		// one real collection node
		realCollNode = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithIDInt(1))

		// a ghost node masquerading as a collection node
		ghostCollNode = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithIDInt(2),
			testnet.AsGhost())

		// three consensus nodes
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))

		// an actual execution node
		realExeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))

		// a verification node
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))

		accessNode = testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel))
	)

	nodes := append([]testnet.NodeConfig{realCollNode, ghostCollNode, conNode1, conNode2, conNode3, realExeNode, verNode, accessNode})
	conf := testnet.NewNetworkConfig("ghost_example_send", nodes)

	net := testnet.PrepareFlowNetwork(t, conf)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Remove()

	// get the ghost container
	ghostContainer := net.ContainerByID(ghostCollNode.Identifier)

	// get a ghost client connected to the ghost node
	ghostClient, err := GetGhostClient(ghostContainer)
	assert.NoError(t, err)

	// generate a signed transaction
	tx := generateTransaction(t, net, realCollNode.Identifier)

	// send the transaction as an event to the real collection node
	err = ghostClient.Send(ctx, engine.PushTransactions, &tx, realCollNode.Identifier)
	assert.NoError(t, err)
}

func sendTransaction(t *testing.T, net *testnet.FlowNetwork, collectionID flow.Identifier) {
	colContainer1 := net.ContainerByID(collectionID)

	collectionClient, err := testnet.NewClient(colContainer1.Addr(testnet.ColNodeAPIPort), net.Root().Header.ChainID.Chain())
	assert.Nil(t, err)

	txFixture := SDKTransactionFixture()
	tx, err := collectionClient.SignTransaction(&txFixture)
	assert.Nil(t, err)

	t.Log("sending transaction: ", tx.ID())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = collectionClient.SendTransaction(ctx, tx)
	assert.Nil(t, err)
}

func generateTransaction(t *testing.T, net *testnet.FlowNetwork, collectionID flow.Identifier) *sdk.Transaction {
	colContainer1 := net.ContainerByID(collectionID)

	collectionClient, err := testnet.NewClient(colContainer1.Addr(testnet.ColNodeAPIPort), net.Root().Header.ChainID.Chain())
	assert.Nil(t, err)

	txFixture := SDKTransactionFixture()
	tx, err := collectionClient.SignTransaction(&txFixture)
	assert.Nil(t, err)

	return tx
}
