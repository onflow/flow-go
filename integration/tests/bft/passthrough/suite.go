package passthrough

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/insecure/attacknetwork"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/codec/cbor"
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
	attackNet               *attacknetwork.AttackNetwork
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
	s.T().Logf("integration/tests/bft/passthrough/suite.go before unittest.LoggerWithLevel()")
	logger := unittest.LoggerWithLevel(zerolog.DebugLevel).With().
		Str("testfile", "suite.go").
		Str("testcase", s.T().Name()).
		Logger()
	s.log = logger
	s.T().Logf("integration/tests/bft/passthrough/suite.go before Access nodeConfig")
	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.DebugLevel)))

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(4)
	for _, nodeID := range s.nodeIDs {
		s.T().Logf("integration/tests/bft/passthrough/suite.go - before consensus nodeConfig with nodeID{%s}", nodeID)
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.DebugLevel),
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=1"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=1"),
			//testnet.WithAdditionalFlag("--network=\"host\""),
		)
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	// generates one corrupted verification node
	s.verID = unittest.IdentifierFixture()
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before verConfig")
	verConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.verID),
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.AsCorrupted())
	//testnet.WithAdditionalFlag("--network=\"host\""))
	s.nodeConfigs = append(s.nodeConfigs, verConfig)

	// generates two corrupted execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before exe1Config")
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.AsCorrupted())
	//testnet.WithAdditionalFlag("--network=\"host\""))
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	s.exe2ID = unittest.IdentifierFixture()
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before exe2Config")
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe2ID),
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.AsCorrupted())
	//testnet.WithAdditionalFlag("--network=\"host\""))
	s.nodeConfigs = append(s.nodeConfigs, exe2Config)

	// generates two collection node
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before coll1Config")
	coll1Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.DebugLevel),
		//testnet.WithAdditionalFlag("--network=\"host\""),
	)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before coll2Config")
	coll2Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.DebugLevel),
		//testnet.WithAdditionalFlag("--network=\"host\""),
	)
	s.nodeConfigs = append(s.nodeConfigs, coll1Config, coll2Config)

	// Ghost Node
	// the ghost node's objective is to observe the messages exchanged on the
	// system and decide to terminate the test.
	// By definition, ghost node is subscribed to all channels.
	s.ghostID = unittest.IdentifierFixture()
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before ghostConfig")
	ghostConfig := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.ghostID),
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.DebugLevel))
	//testnet.WithAdditionalFlag("--network=\"host\""))
	s.nodeConfigs = append(s.nodeConfigs, ghostConfig)

	// generates, initializes, and starts the Flow network
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before testnet.NewNetworkConfig")
	netConfig := testnet.NewNetworkConfig(
		"bft_passthrough_test",
		s.nodeConfigs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before testnet.PrepareFlowNetwork")
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.BftTestnet)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - after testnet.PrepareFlowNetwork - iterating over container list")

	for key, val := range s.net.Containers {
		s.T().Logf("Key: %s, Value: %s\n", key, val.Name())
		val.AddFlag("network", "host")
	}
	s.T().Logf("integration/tests/bft/passthrough/suite.go - finished iterating over container list")
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.T().Logf("integration/tests/bft/passthrough/suite.go - before s.net.Start(ctx)")
	s.net.Start(ctx)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - after s.net.Start(ctx)")

	s.Orchestrator = NewDummyOrchestrator(logger)

	s.T().Logf("integration/tests/bft/passthrough/suite.go - created NewDummyOrchestrator")
	// start attack network
	//const serverAddress = "127.0.0.1:0" // we let OS picking an available port for attack network
	const serverAddress = "host.docker.internal:0" // we let OS picking an available port for attack network
	codec := cbor.NewCodec()
	s.T().Logf("integration/tests/bft/passthrough/suite.go - created NewCode")
	connector := attacknetwork.NewCorruptedConnector(s.net.CorruptedIdentities(), s.net.CorruptedPortMapping)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - created NewCorruptedConnector")
	attackNetwork, err := attacknetwork.NewAttackNetwork(s.log,
		serverAddress,
		codec,
		s.Orchestrator,
		connector,
		s.net.CorruptedIdentities())
	s.T().Logf("integration/tests/bft/passthrough/suite.go - created NewAttackNetwork")
	require.NoError(s.T(), err)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - assigning attackNetwork")
	s.attackNet = attackNetwork
	s.T().Logf("integration/tests/bft/passthrough/suite.go - about to irrecoverable.WithSignaler")
	attackCtx, errChan := irrecoverable.WithSignaler(ctx)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - about to go func()")
	go func() {
		select {
		case err := <-errChan:
			s.T().Error("attackNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()
	s.T().Logf("integration/tests/bft/passthrough/suite.go - about to attackNetwork.Start()")
	attackNetwork.Start(attackCtx)
	s.T().Logf("integration/tests/bft/passthrough/suite.go - about to unittest.RequireCloseBefore()")
	unittest.RequireCloseBefore(s.T(), attackNetwork.Ready(), 1*time.Second, "could not start attack network on time")

	// starts tracking blocks by the ghost node
	s.T().Logf("integration/tests/bft/passthrough/suite.go - about to starts tracking blocks by the ghost node")
	s.Track(s.T(), ctx, s.Ghost())
}

// TearDownSuite tears down the test network of Flow as well as the BFT testing attack network.
func (s *Suite) TearDownSuite() {
	s.net.Remove()
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.attackNet.Done(), 1*time.Second, "could not stop attack network on time")
}
