// Package bft provides testing facilities for Flow BFT protocols.
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

// BaseSuite serves as a base test suite offering various utility functions
// and default setup/teardown steps. It facilitates the creation of Flow networks
// with pre-configured nodes and allows for easier interaction with the network,
// reducing boilerplate code in individual tests.
//
// BaseSuite comes with a lot of functionality out-of-the-box, including the ability to:
// - Create a bare-minimum Flow network.
// - Start and stop the network.
// - Track messages over testnet using TestnetStateTracker.
// - Tear down the testnet environment.
// - Handle Ghost nodes and Orchestrator network.
//
// BaseSuite embeds testify's Suite to leverage setup, teardown, and assertion capabilities.
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

// Ghost returns a client to interact with the Ghost node on the testnet.
// It is essential for observing the messages exchanged in the network.
func (b *BaseSuite) Ghost() *client.GhostClient {
	c, err := b.Net.ContainerByID(b.GhostID).GhostClient()
	require.NoError(b.T(), err, "could not get ghost client")
	return c
}

// AccessClient returns a client to interact with the access node api on testnet.
func (b *BaseSuite) AccessClient() *testnet.Client {
	c, err := b.Net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(b.T(), err, "could not get access client")
	return c
}

// SetupSuite initializes the BaseSuite, setting up a bare-minimum Flow network.
// It configures nodes with roles such as access, consensus, verification, execution,
// and collection. It also sets up a Ghost node for observing messages exchanged on the network.
func (b *BaseSuite) SetupSuite() {
	b.Log = unittest.LoggerForTest(b.Suite.T(), zerolog.InfoLevel)

	// setup single access node
	b.NodeConfigs = append(b.NodeConfigs,
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
	)

	// setup consensus nodes
	for _, nodeID := range unittest.IdentifierListFixture(3) {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=1"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=1"),
			// `cruise-ctl-fallback-proposal-duration` is set to 250ms instead to of 1ms
			// to purposely slow down the block rate. This is needed since the crypto module
			// update providing faster BLS operations.
			// TODO: fix the access integration test logic to function without slowing down
			// the block rate
			testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=250ms"),
		)
		b.NodeConfigs = append(b.NodeConfigs, nodeConfig)
	}

	// setup single verification node
	b.NodeConfigs = append(b.NodeConfigs,
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

// TearDownSuite cleans up the resources, stopping both the Flow network and the
// orchestrator network if they have been initialized.
func (b *BaseSuite) TearDownSuite() {
	b.Net.Remove()
	b.Cancel()
	// check if orchestrator network is set on the base suite, not all tests use the corrupted network.
	if b.OrchestratorNetwork != nil {
		unittest.RequireCloseBefore(b.T(), b.OrchestratorNetwork.Done(), 1*time.Second, "could not stop orchestrator network on time")
	}
}

// StartCorruptedNetwork initializes and starts a corrupted Flow network.
// This should be called after the test suite is set up. The function accepts
// configurations like the name of the network, the number of views in the staking auction,
// the number of views in an epoch, and a function to attack the orchestrator.
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
