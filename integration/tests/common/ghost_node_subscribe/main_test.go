package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

// Tests to check if the Ghost node works as expected

// TestGhostNodeExample_Subscribe demonstrates how to emulate a node and receive all inbound events for it
func TestGhostNodeExample_Subscribe(t *testing.T) {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "ghost_node_subscribe/main_test.go").
		Str("testcase", t.Name()).
		Logger()
	logger.Info().Msgf("================> START TESTING")

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
	defer func() {
		logger.Info().Msgf("================> Start TearDownTest")
		net.Remove()
		logger.Info().Msgf("================> Finish TearDownTest")
	}()

	// get the ghost container
	ghostContainer := net.ContainerByID(ghostExeNode.Identifier)

	// get a ghost client connected to the ghost node
	ghostClient, err := common.GetGhostClient(ghostContainer)
	assert.NoError(t, err)

	// subscribe to all the events the ghost execution node will receive
	msgReader, err := ghostClient.Subscribe(ctx)
	assert.NoError(t, err)

	// wait for 5 blocks proposals
	for i := 0; i < 5; {
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
