package access

import (
	"context"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type AccessRPCSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *AccessRPCSuite) TearDownTest() {
	s.log.Info().Msgf("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}

func (suite *AccessRPCSuite) SetupTest() {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "api.go").
		Str("testcase", suite.T().Name()).
		Logger()
	suite.log = logger
	suite.log.Info().Msgf("================> SetupTest")
	defer func() {
		suite.log.Info().Msgf("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel)),
	}

	// need one dummy execution node (unused ghost)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one controllable collection node (unused ghost)
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, collConfig)

	// need three consensus nodes (unused ghost)
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithID(conID),
			testnet.AsGhost())
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.T().Logf("starting flow network with docker containers")
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)
}

func (suite *AccessRPCSuite) TestExecutionResultID() {
	// setup RPC connection

	// get sample execution result ID

	// request same result and compute ID

}
