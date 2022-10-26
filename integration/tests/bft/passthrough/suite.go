package passthrough

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/insecure/orchestrator"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite evaluating the integration of the testnet against
// happy path of Corrupted Conduit Framework (CCF) for BFT testing.
type Suite struct {
	suite.Suite
	log                     zerolog.Logger
	lib.TestnetStateTracker                      // used to track messages over testnet
	cancel                  context.CancelFunc   // used to tear down the testnet
	net                     *testnet.FlowNetwork // used to keep an instance of testnet
	nodeConfigs             []testnet.NodeConfig // used to keep configuration of nodes in testnet
	nodeIDs                 []flow.Identifier    // used to keep identifier of nodes in testnet
	ghostID                 flow.Identifier      // represents id of ghost node
	exe1ID                  flow.Identifier      // corrupted execution node 1
	exe2ID                  flow.Identifier      // corrupted execution node 2
	verID                   flow.Identifier      // corrupted verification node
	PreferredUnicasts       string               // preferred unicast protocols between execution and verification nodes.
	Orchestrator            *dummyOrchestrator
	orchestratorNetwork     *orchestrator.Network
}

// Ghost returns a client to interact with the Ghost node on testnet.
func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	cli, err := lib.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return cli
}

// AccessClient returns a client to interact with the access node api on testnet.
func (s *Suite) AccessClient() *testnet.Client {
	chain := s.net.Root().Header.ChainID.Chain()
	cli, err := testnet.NewClient(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(s.T(), err, "could not get access client")
	return cli
}

// SetupSuite runs a bare minimum Flow network to function correctly with the following roles:
// - Two collector nodes
// - Four consensus nodes
// - two corrupted execution node
// - One corrupted verification node
// - One ghost node (as an execution node)
func (s *Suite) SetupSuite() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)))

	blockRateFlag := "--block-rate-delay=1ms"

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(4)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=1"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=1"),
			testnet.WithAdditionalFlag(blockRateFlag),
		)
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	// generates one corrupted verification node
	s.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.verID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.nodeConfigs = append(s.nodeConfigs, verConfig)

	// generates two corrupted execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	s.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe2ID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.nodeConfigs = append(s.nodeConfigs, exe2Config)

	// generates two collection node
	coll1Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	coll2Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	s.nodeConfigs = append(s.nodeConfigs, coll1Config, coll2Config)

	// Ghost Node
	// the ghost node's objective is to observe the messages exchanged on the
	// system and decide to terminate the test.
	// By definition, ghost node is subscribed to all channels.
	s.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.ghostID),
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.FatalLevel))
	s.nodeConfigs = append(s.nodeConfigs, ghostConfig)

	// generates, initializes, and starts the Flow network
	netConfig := testnet.NewNetworkConfig(
		"bft_passthrough_test",
		s.nodeConfigs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)

	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.BftTestnet)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// starts tracking blocks by the ghost node
	s.Track(s.T(), ctx, s.Ghost())

	s.Orchestrator = NewDummyOrchestrator(s.log)

	// start orchestrator network
	codec := unittest.NetworkCodec()
	connector := orchestrator.NewCorruptedConnector(s.log, s.net.CorruptedIdentities(), s.net.CorruptedPortMapping)
	orchestratorNetwork, err := orchestrator.NewOrchestratorNetwork(s.log,
		codec,
		s.Orchestrator,
		connector,
		s.net.CorruptedIdentities())
	require.NoError(s.T(), err)
	s.orchestratorNetwork = orchestratorNetwork

	attackCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			s.T().Error("orchestratorNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	orchestratorNetwork.Start(attackCtx)
	unittest.RequireCloseBefore(s.T(), orchestratorNetwork.Ready(), 1*time.Second, "could not start orchestrator network on time")
}

// TearDownSuite tears down the test network of Flow as well as the BFT testing orchestrator network.
func (s *Suite) TearDownSuite() {
	s.net.Remove()
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.orchestratorNetwork.Done(), 1*time.Second, "could not stop orchestrator network on time")
}
