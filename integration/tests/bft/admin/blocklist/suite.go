package blocklist

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
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

// Suite represents a test suite ensures the admin block list command works as expected.
type Suite struct {
	suite.Suite
	log                     zerolog.Logger
	lib.TestnetStateTracker                      // used to track messages over testnet
	cancel                  context.CancelFunc   // used to tear down the testnet
	net                     *testnet.FlowNetwork // used to keep an instance of testnet
	nodeConfigs             []testnet.NodeConfig // used to keep configuration of nodes in testnet
	nodeIDs                 []flow.Identifier    // used to keep identifier of nodes in testnet
	senderVN                flow.Identifier      // node ID of corrupted node that will send messages in the test. The sender node will be blocked.
	receiverEN              flow.Identifier      // node ID of corrupted node that will receive messages in the test
	ghostID                 flow.Identifier      // represents id of ghost node
	Orchestrator            *AdminBlockListAttackOrchestrator
	orchestratorNetwork     *orchestrator.Network
}

// Ghost returns a client to interact with the Ghost node on testnet.
func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	cli, err := lib.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return cli
}

// SetupSuite runs a bare minimum Flow network to function correctly along with 2 attacker nodes and 1 victim node.
// - Corrupt VN that will be used to send messages, this node will be the node that is blocked by the receiver corrupt EN.
// - Corrupt EN that will receive messages from the corrupt VN, we will execute the admin command on this node.
func (s *Suite) SetupSuite() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)))

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)))

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

	// generate 1 corrupt verification node
	s.senderVN = unittest.IdentifierFixture()
	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.senderVN),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)))

	// generate 1 corrupt execution node
	s.receiverEN = unittest.IdentifierFixture()
	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.receiverEN),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)))

	// generates two collection node
	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag)), testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag)))

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
		"bft_signature_validation_test",
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

	s.Orchestrator = NewOrchestrator(s.T(), s.log, s.senderVN, s.receiverEN)

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

// blockNode submit request to our EN admin server to block sender VN.
func (s *Suite) blockNode(nodeID flow.Identifier) {
	url := fmt.Sprintf("http://0.0.0.0:%s/admin/run_command", s.net.AdminPortsByNodeID[s.receiverEN])
	body := fmt.Sprintf(`{"commandName": "set-config", "data": {"network-id-provider-blocklist": ["%s"]}}`, nodeID.String())
	reqBody := bytes.NewBuffer([]byte(body))
	resp, err := http.Post(url, "application/json", reqBody)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 200, resp.StatusCode)
	require.NoError(s.T(), resp.Body.Close())
}
