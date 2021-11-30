package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
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

	// wait for 5 blocks proposals
	for i:=0;i<5;{
		from, event, err := msgReader.Next()
		assert.NoError(t, err)

		// the following switch should be similar to the one defined in the actual node that is being emulated
		switch v := event.(type) {
		case *messages.BlockProposal:
			fmt.Printf("Received block proposal: %s from %s\n", v.Header.ID().String(), from.String())
			i++
		default:
			t.Logf(" ignoring event: :%T: %v", v, v)
		}
	}
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

	// generate a test transaction
	tx := unittest.TransactionBodyFixture()

	// send the transaction as an event to a real collection node
	err = ghostClient.Send(ctx, engine.PushTransactions, &tx, realCollNode.Identifier)
	assert.NoError(t, err)
}
