package cohort3

import (
	"context"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	accessNodeContainer := s.flowNetwork.ContainerByName(testnet.PrimaryAN)

	// Establish a gRPC connection to the access API
	conn, err := grpc.Dial(accessNodeContainer.Addr(testnet.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.Require().NotNil(conn)

	// Create a client for the access API
	serviceClient, err := accessNodeContainer.TestnetClient()
	s.Require().NoError(err)
	s.Require().NotNil(serviceClient)

	header, err := serviceClient.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	serviceAddress := serviceClient.SDKServiceAddress()
	tx := sdk.NewTransaction().
		SetScript([]byte(lib.CreateCounterTx(serviceAddress).ToCadence())).
		SetReferenceBlockID(header.ID).
		SetProposalKey(serviceAddress, 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(serviceAddress).
		AddAuthorizer(serviceAddress).
		SetComputeLimit(9999)

	err = serviceClient.SignAndSendTransaction(s.ctx, tx)
	s.Require().NoError(err)

	//accessClient.SendTransaction()
	// TODO:
	// 1.
	// - create account without money (sign with service account)
	// - send some write tx
	// - check if validator rejects tx
	// 2.
	// - create account with money
	// - send write tx
	// - check if validator accepts tx
}
