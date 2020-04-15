package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// These tests are just examples of how to use the ghost node in an integration tests.
// They do no test any functionality

// TestGhostNodeExample_Subscribe demonstrates how to emulate a node and receive all inbound events for it
func TestGhostNodeExample_Subscribe(t *testing.T) {

	var (
		// one collection node
		collNode = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel("info"), testnet.WithIDInt(1))

		// three consensus nodes
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))

		// an actual execution node
		realExeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel("info"), testnet.WithIDInt(2))

		// a ghost node masquerading as an execution node
		ghostExeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel("debug"), testnet.WithIDInt(3),
			testnet.AsGhost(true))

		// a verification node
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel("info"))
	)

	nodes := append([]testnet.NodeConfig{collNode, conNode1, conNode2, conNode3, realExeNode, verNode, ghostExeNode})
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "example_ghost", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// get the ghost container
	ghostContainer, ok := net.ContainerByID(ghostExeNode.Identifier)
	assert.True(t, ok)

	// get a ghost client connected to the ghost node
	ghostClient, err := GetGhostClient(ghostContainer)
	assert.NoError(t, err)

	// subscribe to all the events the ghost execution node will receive
	msgReader, err := ghostClient.Subscribe(ctx)
	assert.NoError(t, err)

	// create and send a transaction to one of the collection node
	sendTransaction(t, net, collNode.Identifier)

	blocks := make([]*flow.Block, 0)

	for {
		_, event, err := msgReader.Next()
		assert.NoError(t, err)

		// the following switch should be similar to the one defined in the actual node that is being emulated
		switch v := event.(type) {
		case *flow.Block:
			blocks = append(blocks, v)
		default:
			t.Logf(" ignoring event: :%T: %v", v, v)
		}

		if len(blocks) == 2 {
			break
		}
	}

	assert.EqualValues(t, 1, blocks[0].Height)
	assert.EqualValues(t, 2, blocks[1].Height)

	err = net.StopContainers()
	assert.Nil(t, err)
}

func sendTransaction(t *testing.T, net *testnet.FlowNetwork, collectionID flow.Identifier) {
	colContainer1, ok := net.ContainerByID(collectionID)
	assert.True(t, ok)

	port, ok := colContainer1.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	collectionClient, err := testnet.NewClient(fmt.Sprintf(":%s", port))
	assert.Nil(t, err)

	tx := unittest.TransactionBodyFixture()
	tx, err = collectionClient.SignTransaction(tx)
	assert.Nil(t, err)

	t.Log("sending transaction: ", tx.ID())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = collectionClient.SendTransaction(ctx, tx)
	assert.Nil(t, err)
}
