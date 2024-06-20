package cohort3

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"testing"

	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccessNodeTransactionValidator(t *testing.T) {
	suite.Run(t, new(AccessNodeTransactionValidatorTestSuite))
}

type AccessNodeTransactionValidatorTestSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	flowNetwork *testnet.FlowNetwork

	accessNodeGrpcClient *client.Client //TODO: maybe it will be unused
}

func (s *AccessNodeTransactionValidatorTestSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.flowNetwork.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *AccessNodeTransactionValidatorTestSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
	accessNodeConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		// make sure test continues to test as expected if the default config changes
		testnet.WithAdditionalFlagf("--script-execution-mode=%s", backend.IndexQueryModeLocalOnly),
		testnet.WithAdditionalFlagf("--tx-result-query-mode=%s", backend.IndexQueryModeLocalOnly),
		testnet.WithAdditionalFlag("--check-payer-balance=true"),
	)

	accessNodeConfig2 := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		// make sure test continues to test as expected if the default config changes
		testnet.WithAdditionalFlagf("--script-execution-mode=%s", backend.IndexQueryModeLocalOnly),
		testnet.WithAdditionalFlagf("--tx-result-query-mode=%s", backend.IndexQueryModeLocalOnly),
		testnet.WithAdditionalFlag("--check-payer-balance=true"),
	)

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		accessNodeConfig,
		accessNodeConfig2,
	}

	// start the network
	conf := testnet.NewNetworkConfig("access_node_transaction_validator_test", nodeConfigs)
	s.flowNetwork = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	s.T().Log("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.flowNetwork.Start(s.ctx)
}

func (s *AccessNodeTransactionValidatorTestSuite) TestTransactionValidatorExecutesScript() {
	//var err error
	//s.accessNodeGrpcClient, err = s.flowNetwork.ContainerByName(testnet.PrimaryAN).SDKClient()
	//s.Require().NoError(err)

	s.log.Info().Msg("TestTransactionValidatorExecutesScript is running")
	//address := s.flowNetwork.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)
	//s.log.Info().Msg("Address:" + address)
	//tx := testutil.CreateAccountCreationTransaction()
	//s.accessNodeGrpcClient.SendTransaction(s.ctx, tx)
}
