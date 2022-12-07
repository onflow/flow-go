package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	log zerolog.Logger
	lib.TestnetStateTracker
	cancel      context.CancelFunc
	net         *testnet.FlowNetwork
	nodeConfigs []testnet.NodeConfig
	nodeIDs     []flow.Identifier
	ghostID     flow.Identifier
	exe1ID      flow.Identifier
	verID       flow.Identifier
}

func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	client, err := lib.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *Suite) AccessClient() *testnet.Client {
	chain := s.net.Root().Header.ChainID.Chain()
	client, err := testnet.NewClient(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(s.T(), err, "could not get access client")
	return client
}

func (s *Suite) ExecutionClient() *testnet.Client {
	execNode := s.net.ContainerByID(s.exe1ID)
	chain := s.net.Root().Header.ChainID.Chain()
	client, err := testnet.NewClient(fmt.Sprintf(":%s", execNode.Ports[testnet.ExeNodeAPIPort]), chain)
	require.NoError(s.T(), err, "could not get execution client")
	return client
}

type AdminCommandRequest struct {
	CommandName string `json:"commandName"`
	Data        any    `json:"data"`
}

type AdminCommandResponse struct {
	Output any `json:"output"`
}

// SendExecutionAdminCommand sends admin command to EN. data will be serialized to JSON and sent as data part of the command request.
// Response will be deserialized into output object.
// It bubbles up errors from (un)marshalling of data and handling the request
func (s *Suite) SendExecutionAdminCommand(ctx context.Context, command string, data any, output any) error {
	enContainer := s.net.ContainerByID(s.exe1ID)

	request := AdminCommandRequest{
		CommandName: command,
		Data:        data,
	}

	marshal, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error while marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("http://localhost:%s/admin/run_command", enContainer.Ports[testnet.ExeNodeAdminPort]),
		bytes.NewBuffer(marshal),
	)
	if err != nil {
		return fmt.Errorf("error while building request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error while sending request: %w", err)
	}

	adminCommandResponse := AdminCommandResponse{
		Output: output,
	}

	err = json.NewDecoder(resp.Body).Decode(&adminCommandResponse)
	if err != nil {
		return fmt.Errorf("error while reading/decoding response: %w", err)
	}

	return nil
}

func (s *Suite) AccessPort() string {
	return s.net.AccessPorts[testnet.AccessNodeAPIPort]
}

func (s *Suite) MetricsPort() string {
	return s.net.AccessPorts[testnet.ExeNodeMetricsPort]
}

func (s *Suite) SetupTest2() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")

	blockRateFlag := "--block-rate-delay=1ms"

	s.nodeConfigs = append(s.nodeConfigs, testnet.NewNodeConfig(flow.RoleAccess))

	// generate the four consensus identities
	s.nodeIDs = unittest.IdentifierListFixture(4)
	for _, nodeID := range s.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
			testnet.WithAdditionalFlag(blockRateFlag),
			testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
			testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		)
		s.nodeConfigs = append(s.nodeConfigs, nodeConfig)
	}

	// need one execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.DebugLevel))
	s.nodeConfigs = append(s.nodeConfigs, exe1Config)

	vn1Config := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	vn2Config := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	vn3Config := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))

	s.nodeConfigs = append(s.nodeConfigs, vn1Config, vn2Config, vn3Config)

	// need two collection node
	coll1Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	coll2Config := testnet.NewNodeConfig(flow.RoleCollection,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag(blockRateFlag),
	)
	s.nodeConfigs = append(s.nodeConfigs, coll1Config, coll2Config)

	// add the ghost (verification) node config
	s.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleAccess,
		testnet.WithID(s.ghostID),
		testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.InfoLevel))
	s.nodeConfigs = append(s.nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig(
		"upgrade_tests",
		s.nodeConfigs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.Localnet)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())

}

func (s *Suite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")

	//s.ctx, s.cancel = context.WithCancel(context.Background())
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=10ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	// a ghost node masquerading as an access node
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost())

	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.WarnLevel)),
		ghostNode,
	}

	netConfig := testnet.NewNetworkConfig(
		"upgrade_tests",
		confs,
		// set long staking phase to avoid QC/DKG transactions during test run
		testnet.WithViewsInStakingAuction(10_000),
		testnet.WithViewsInEpoch(100_000),
	)
	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConfig, flow.Localnet)

	// start the network

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())
}

func (s *Suite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}
