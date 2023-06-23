package bft

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/orchestrator"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

type BaseSuite struct {
	suite.Suite
	Log                     zerolog.Logger
	lib.TestnetStateTracker // used to track messages over testnet
	Ctx                     context.Context
	Cancel                  context.CancelFunc   // used to tear down the testnet
	Net                     *testnet.FlowNetwork // used to keep an instance of testnet
	GhostID                 flow.Identifier      // represents id of ghost node
	NodeConfigs             testnet.NodeConfigs  // used to keep configuration of nodes in testnet
	OrchestratorNetwork     *orchestrator.Network
}

// Ghost returns a client to interact with the Ghost node on testnet.
func (b *BaseSuite) Ghost() *client.GhostClient {
	client, err := b.Net.ContainerByID(b.GhostID).GhostClient()
	require.NoError(b.T(), err, "could not get ghost client")
	return client
}

// AccessClient returns a client to interact with the access node api on testnet.
func (b *BaseSuite) AccessClient() *testnet.Client {
	client, err := b.Net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(b.T(), err, "could not get access client")
	return client
}

// SetupSuite sets up node configs to run a bare minimum Flow network to function correctly.
func (b *BaseSuite) SetupSuite() {
	b.Log = unittest.LoggerForTest(b.Suite.T(), zerolog.InfoLevel)

	// setup access nodes
	b.NodeConfigs = append(b.NodeConfigs,
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
	)

	// setup consensus nodes
	for _, nodeID := range unittest.IdentifierListFixture(4) {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=1"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=1"),
			testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=1ms"),
		)
		b.NodeConfigs = append(b.NodeConfigs, nodeConfig)
	}

	// setup verification nodes
	b.NodeConfigs = append(b.NodeConfigs,
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
	)

	// setup execution nodes
	b.NodeConfigs = append(b.NodeConfigs,
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
	)

	// setup collection nodes
	b.NodeConfigs = append(b.NodeConfigs,
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--hotstuff-proposal-duration=1ms")),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--hotstuff-proposal-duration=1ms")),
	)

	// Ghost Node
	// the ghost node's objective is to observe the messages exchanged on the
	// system and decide to terminate the test.
	// By definition, ghost node is subscribed to all channels.
	b.GhostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(b.GhostID),
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel))
	b.NodeConfigs = append(b.NodeConfigs, ghostConfig)
}

// TearDownSuite tears down the test network of Flow as well as the BFT testing orchestrator network.
func (b *BaseSuite) TearDownSuite() {
	b.Net.Remove()
	b.Cancel()
	// check if orchestrator network is set on the base suite, not all tests use the corrupted network.
	if b.OrchestratorNetwork != nil {
		unittest.RequireCloseBefore(b.T(), b.OrchestratorNetwork.Done(), 1*time.Second, "could not stop orchestrator network on time")
	}
}

// StartCorruptedNetwork starts the corrupted network with the configured node configs, this func should be used after test suite is setup.
func (b *BaseSuite) StartCorruptedNetwork(name string, viewsInStakingAuction, viewsInEpoch uint64, attackOrchestrator func() insecure.AttackOrchestrator) {
	// generates, initializes, and starts the Flow network
	netConfig := testnet.NewNetworkConfig(
		name,
		b.NodeConfigs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(viewsInStakingAuction),
		testnet.WithViewsInEpoch(viewsInEpoch),
	)

	b.Net = testnet.PrepareFlowNetwork(b.T(), netConfig, flow.BftTestnet)
	b.Ctx, b.Cancel = context.WithCancel(context.Background())
	b.Net.Start(b.Ctx)

	// starts tracking blocks by the ghost node
	b.Track(b.T(), b.Ctx, b.Ghost())

	// start orchestrator network
	codec := unittest.NetworkCodec()
	connector := orchestrator.NewCorruptedConnector(b.Log, b.Net.CorruptedIdentities(), b.Net.CorruptedPortMapping)
	orchestratorNetwork, err := orchestrator.NewOrchestratorNetwork(b.Log,
		codec,
		attackOrchestrator(),
		connector,
		b.Net.CorruptedIdentities())
	require.NoError(b.T(), err)
	b.OrchestratorNetwork = orchestratorNetwork

	attackCtx, errChan := irrecoverable.WithSignaler(b.Ctx)
	go func() {
		select {
		case err := <-errChan:
			b.T().Error("orchestratorNetwork startup encountered fatal error", err)
		case <-b.Ctx.Done():
			return
		}
	}()

	orchestratorNetwork.Start(attackCtx)
	unittest.RequireCloseBefore(b.T(), orchestratorNetwork.Ready(), 1*time.Second, "could not start orchestrator network on time")
}
