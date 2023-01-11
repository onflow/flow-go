package access

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccess(t *testing.T) {
	suite.Run(t, new(AccessSuite))
}

type AccessSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *AccessSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (suite *AccessSuite) SetupTest() {
	suite.log = unittest.LoggerForTest(suite.Suite.T(), zerolog.InfoLevel)
	suite.log.Info().Msg("================> SetupTest")
	defer func() {
		suite.log.Info().Msg("================> Finish SetupTest")
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
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf, flow.Localnet)

	// start the network
	suite.T().Logf("starting flow network with docker containers")
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	suite.net.Start(suite.ctx)
}

func (suite *AccessSuite) TestAPIsAvailable() {
	suite.T().Run("TestHTTPProxyPortOpen", func(t *testing.T) {
		httpProxyAddress := net.JoinHostPort("localhost", suite.net.AccessPorts[testnet.AccessNodeAPIProxyPort])

		conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
		require.NoError(suite.T(), err, "http proxy port not open on the access node")

		conn.Close()
	})

	suite.T().Run("TestAccessConnection", func(t *testing.T) {
		grpcAddress := net.JoinHostPort("localhost", suite.net.AccessPorts[testnet.AccessNodeAPIPort])

		ctx, cancel := context.WithTimeout(suite.ctx, 1*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err, "failed to connect to access node")
		defer conn.Close()

		client := accessproto.NewAccessAPIClient(conn)

		_, err = client.Ping(suite.ctx, &accessproto.PingRequest{})
		assert.NoError(t, err, "failed to ping access node")
	})
}
