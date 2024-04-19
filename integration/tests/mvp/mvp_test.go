package mvp

import (
	"context"
	"fmt"
	"testing"

	"github.com/dapperlabs/testingdock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMVP_Network(t *testing.T) {
	logger := unittest.LoggerForTest(t, zerolog.InfoLevel)
	logger.Info().Msgf("================> START TESTING")
	flowNetwork := testnet.PrepareFlowNetwork(t, buildMVPNetConfig(), flow.Localnet)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork.Start(ctx)
	defer func() {
		logger.Info().Msg("================> Start TearDownTest")
		flowNetwork.Remove()
		logger.Info().Msg("================> Finish TearDownTest")
	}()

	RunMVPTest(t, ctx, flowNetwork, flowNetwork.ContainerByName(testnet.PrimaryAN))
}

func TestMVP_Bootstrap(t *testing.T) {
	logger := unittest.LoggerForTest(t, zerolog.InfoLevel)
	logger.Info().Msgf("================> START TESTING")
	unittest.SkipUnless(t, unittest.TEST_TODO, "skipping to be re-visited in https://github.com/dapperlabs/flow-go/issues/5451")

	testingdock.Verbose = false

	flowNetwork := testnet.PrepareFlowNetwork(t, buildMVPNetConfig(), flow.Localnet)
	defer func() {
		logger.Info().Msg("================> Start TearDownTest")
		flowNetwork.Remove()
		logger.Info().Msg("================> Finish TearDownTest")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork.Start(ctx)

	accessNode := flowNetwork.ContainerByName(testnet.PrimaryAN)

	client, err := accessNode.TestnetClient()
	require.NoError(t, err)

	t.Log("@@ running mvp test 1")

	// run mvp test to build a few blocks
	RunMVPTest(t, ctx, flowNetwork, accessNode)

	t.Log("@@ finished running mvp test 1")

	// download root snapshot from access node
	snapshot, err := client.GetLatestProtocolSnapshot(ctx)
	require.NoError(t, err)

	// verify that the downloaded snapshot is not for the root block
	header, err := snapshot.Head()
	require.NoError(t, err)
	assert.True(t, header.ID() != flowNetwork.Root().Header.ID())

	t.Log("@@ restarting network with new root snapshot")

	flowNetwork.StopContainers()
	flowNetwork.RemoveContainers()

	// pick 1 consensus node to restart with empty database and downloaded snapshot
	cons := flowNetwork.Identities().Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))
	random, err := rand.Uintn(uint(len(cons)))
	require.NoError(t, err)
	con1 := cons[random]

	t.Log("@@ booting from non-root state on consensus node ", con1.NodeID)

	flowNetwork.DropDBs(filter.HasNodeID[flow.Identity](con1.NodeID))
	con1Container := flowNetwork.ContainerByID(con1.NodeID)
	con1Container.DropDB()
	con1Container.WriteRootSnapshot(snapshot)

	t.Log("@@ running mvp test 2")

	flowNetwork.Start(ctx)

	// Run MVP tests
	RunMVPTest(t, ctx, flowNetwork, accessNode)

	t.Log("@@ finished running mvp test 2")
}

func buildMVPNetConfig() testnet.NetworkConfig {
	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-proposal-duration=100ms"),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
	}

	return testnet.NewNetworkConfig("mvp", net)
}
