package common

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Tests to check if the Ghost node works as expected

// TestGhostNodeExample_Send demonstrates how to emulate a node and send an event from it
func TestGhostNodeExample_Send(t *testing.T) {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "ghost_node_send/main_test.go").
		Str("testcase", t.Name()).
		Logger()
	logger.Info().Msgf("================> START TESTING")

	var (
		// one real collection node
		realCollNode = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithIDInt(1))

		// a ghost node masquerading as a collection node
		ghostCollNode = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithIDInt(2),
			testnet.AsGhost())

		// three consensus nodes
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())

		// an execution node
		realExeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))

		// a verification node
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())

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
	ghostClient, err := common.GetGhostClient(ghostContainer)
	assert.NoError(t, err)

	// generate a test transaction
	tx := unittest.TransactionBodyFixture()

	// send the transaction as an event to a real collection node
	err = ghostClient.Send(ctx, engine.PushTransactions, &tx, realCollNode.Identifier)
	assert.NoError(t, err)
	t.Logf("%v ================> FINISH TESTING %v", time.Now().UTC(), t.Name())
}
