package access

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestObserver(t *testing.T) {
	suite.Run(t, new(ObserverSuite))
}

type ObserverSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *ObserverSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (suite *ObserverSuite) SetupTest() {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "api.go").
		Str("testcase", suite.T().Name()).
		Logger()
	suite.log = logger
	suite.log.Info().Msg("================> SetupTest")
	defer func() {
		suite.log.Info().Msg("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel)),
	}

	// Add an access node for observer access
	bsConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel), testnet.AsBootstrap())
	nodeConfigs = append(nodeConfigs, bsConfig)

	// Add an observer node
	//obsConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel), testnet.AsObserver())
	//nodeConfigs = append(nodeConfigs, obsConfig)

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
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf, flow.Localnet)

	// start the network
	suite.T().Logf("starting flow network with docker containers")
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)
}

func (suite *ObserverSuite) _TestHTTPProxyPortOpen() {
	httpProxyAddress := fmt.Sprintf(":%s", suite.net.AccessPorts[testnet.AccessNodeAPIProxyPort])
	conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
	require.NoError(suite.T(), err, "http proxy port not open on the access node")
	conn.Close()
}

func (suite *ObserverSuite) _TestObserverPortOpen() {
	httpProxyAddress := fmt.Sprintf(":%s", suite.net.AccessPorts[testnet.AccessNodeAPIProxyPort])
	conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
	require.NoError(suite.T(), err, "http proxy port not open on the observer node")
	conn.Close()
}
