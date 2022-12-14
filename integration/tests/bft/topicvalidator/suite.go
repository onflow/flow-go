package topicvalidator

import (
	"context"
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
	attackerANID            flow.Identifier      // corrupt attacker AN id
	attackerENID            flow.Identifier      // corrupt attacker EN id
	victimENID              flow.Identifier      // corrupt victim EN id
	victimVNID              flow.Identifier      // corrupt victim VN id
	ghostID                 flow.Identifier      // represents id of ghost node
	Orchestrator            *TopicValidatorAttackOrchestrator
	orchestratorNetwork     *orchestrator.Network
}

// Ghost returns a client to interact with the Ghost node on testnet.
func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	cli, err := lib.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return cli
}

// SetupSuite runs a bare minimum Flow network to function correctly along with 2 attacker nodes
// and 2 victim nodes.
// - Corrupt AN that will serve as an attacker and send unauthorized messages to a victim EN.
// - Corrupt EN that will serve as an attacker and send unauthorized messages to a victim VN.
// - Corrupt EN with the topic validator enabled that will serve as a victim.
// - Corrupt VN with the topic validator enabled that will serve as a victim.
func (s *Suite) SetupSuite() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)))

	// create corrupt access node
	s.attackerANID = unittest.IdentifierFixture()
	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess,
		testnet.WithID(s.attackerANID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	blockRateFlag := "--block-rate-delay=1ms"

	// generate the four consensus node configs
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

	// create corrupt verification node with the topic validator enabled. This is the victim
	// node that will be published unauthorized messages from the attacker execution node.
	s.victimVNID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.victimVNID),
		testnet.WithAdditionalFlag("--topic-validator-disabled=false"),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.nodeConfigs = append(s.nodeConfigs, verConfig)

	// generates two execution nodes, 1 of them will be corrupt
	s.attackerENID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.attackerENID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	// create corrupt execution node with the topic validator enabled. This is the victim
	// node that will be published unauthorized messages from the attacker execution node.
	s.victimENID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.victimENID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag("--topic-validator-disabled=false"),
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
		"bft_topic_validator_test",
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

	s.Orchestrator = NewOrchestrator(s.T(), s.log, s.attackerANID, s.attackerENID, s.victimENID, s.victimVNID)

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
