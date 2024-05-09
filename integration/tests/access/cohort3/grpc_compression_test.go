package cohort3

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccessGRPCCompression(t *testing.T) {
	suite.Run(t, new(AccessGRPCSuite))
}

type AccessGRPCSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func (s *AccessGRPCSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *AccessGRPCSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.InfoLevel),
			testnet.WithAdditionalFlag("--grpc-compressor=gzip")),
	}

	// need one execution node
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one controllable collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--hotstuff-proposal-duration=100ms"))
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
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

// TestGRPCCompression is a test suite function that tests GRPC compression in access nodes by providing a flag for compressor name.
func (s *AccessGRPCSuite) TestGRPCCompression() {
	// Get the access node container and client
	accessContainer := s.net.ContainerByName(testnet.PrimaryAN)
	accessClient, err := accessContainer.TestnetClient()
	require.NoError(s.T(), err, "failed to get access node client")
	require.NotNil(s.T(), accessClient, "failed to get access node client")

	lastBlockID, err := accessClient.GetLatestBlockID(s.ctx)
	require.NoError(s.T(), err)

	blockIds := []sdk.Identifier{sdk.Identifier(lastBlockID)}
	eventType := string(flow.EventAccountCreated)

	events, err := accessClient.GetEventsForBlockIDs(s.ctx, eventType, blockIds)
	require.NoError(s.T(), err)

	require.GreaterOrEqual(s.T(), len(events), 1, "expect received event")
}
