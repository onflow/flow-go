package access

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// This is a collection of tests that validate various Access API endpoints work as expected.

const (
	simpleScript = `pub fun main(): Int { return %d; }`
)

func TestAccessAPI(t *testing.T) {
	suite.Run(t, new(AccessAPISuite))
}

type AccessAPISuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork

	accessNode    *testnet.Container
	sdkClient     *client.Client
	serviceClient *testnet.Client
}

func (s *AccessAPISuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *AccessAPISuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// access node
	bridgeANConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir)),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir)),
		testnet.WithAdditionalFlag("--execution-state-checkpoint=/bootstrap/execution-state/bootstrap-checkpoint"),
		testnet.WithAdditionalFlag("--execution-state-checkpoint-height=0"),
		testnet.WithAdditionalFlag("--script-execution-mode=local-only"),
	)

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),

		// AN1 should be the a vanilla node to allow other nodes to bootstrap successfully
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),

		// Tests will focus on AN2
		bridgeANConfig,
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)

	var err error
	s.accessNode = s.net.ContainerByName("access_2")

	s.sdkClient, err = s.accessNode.SDKClient()
	s.Require().NoError(err)

	// pause until the network is progressing
	var header *sdk.BlockHeader
	s.Require().Eventually(func() bool {
		header, err = s.sdkClient.GetLatestBlockHeader(s.ctx, true)
		s.Require().NoError(err)

		return header.Height > 0
	}, 10*time.Second, 1*time.Second)

	// the service client uses GetAccount and requires the first block to be indexed
	s.Require().Eventually(func() bool {
		s.serviceClient, err = s.accessNode.TestnetClient()
		return err == nil
	}, 10*time.Second, 1*time.Second)
}

func (s *AccessAPISuite) TestLocalExecutionState() {
	s.testGetAccount()
	s.testExecuteScriptWithSimpleScript()
	s.testExecuteScriptWithSimpleContract()
}

func (s *AccessAPISuite) testGetAccount() {
	header, err := s.sdkClient.GetLatestBlockHeader(s.ctx, true)
	s.Require().NoError(err)

	serviceAddress := s.serviceClient.SDKServiceAddress()

	s.Run("get account at latest block", func() {
		account, err := s.sdkClient.GetAccount(s.ctx, serviceAddress)
		s.Require().NoError(err)
		s.Assert().Equal(serviceAddress, account.Address)
		s.Assert().NotZero(serviceAddress, account.Balance)
	})

	s.Run("get account block ID", func() {
		account, err := s.sdkClient.GetAccountAtLatestBlock(s.ctx, serviceAddress)
		s.Require().NoError(err)
		s.Assert().Equal(serviceAddress, account.Address)
		s.Assert().NotZero(serviceAddress, account.Balance)
	})

	s.Run("get account block height", func() {
		account, err := s.sdkClient.GetAccountAtBlockHeight(s.ctx, serviceAddress, header.Height)
		s.Require().NoError(err)
		s.Assert().Equal(serviceAddress, account.Address)
		s.Assert().NotZero(serviceAddress, account.Balance)
	})
}

func (s *AccessAPISuite) testExecuteScriptWithSimpleScript() {
	script := fmt.Sprintf(simpleScript, 42)

	header, err := s.sdkClient.GetLatestBlockHeader(s.ctx, true)
	s.Require().NoError(err)

	s.Run("execute at latest block", func() {
		result, err := s.sdkClient.ExecuteScriptAtLatestBlock(s.ctx, []byte(script), nil)
		s.Require().NoError(err)
		s.Assert().Equal(cadence.NewInt(42), result)
	})

	s.Run("execute at block height", func() {
		result, err := s.sdkClient.ExecuteScriptAtBlockHeight(s.ctx, header.Height, []byte(script), nil)
		s.Require().NoError(err)
		s.Assert().Equal(cadence.NewInt(42), result)
	})

	s.Run("execute at block ID", func() {
		result, err := s.sdkClient.ExecuteScriptAtBlockID(s.ctx, header.ID, []byte(script), nil)
		s.Require().NoError(err)
		s.Assert().Equal(cadence.NewInt(42), result)
	})
}

func (s *AccessAPISuite) testExecuteScriptWithSimpleContract() {
	// deploy the test contract
	txResult := s.deployContract()
	targetHeight := txResult.BlockHeight + 1
	s.waitUntilIndexed(targetHeight)

	header, err := s.sdkClient.GetBlockHeaderByHeight(s.ctx, targetHeight)
	s.Require().NoError(err)

	// Check that the initial value is set
	serviceAccount := s.serviceClient.Account()
	script := lib.ReadCounterScript(serviceAccount.Address, serviceAccount.Address).ToCadence()

	s.Run("execute at latest block", func() {
		result, err := s.sdkClient.ExecuteScriptAtLatestBlock(s.ctx, []byte(script), nil)
		s.Require().NoError(err)
		s.Assert().Equal(lib.CounterInitializedValue, result.(cadence.Int).Int())
	})

	s.Run("execute at block height", func() {
		result, err := s.sdkClient.ExecuteScriptAtBlockHeight(s.ctx, header.Height, []byte(script), nil)
		s.Require().NoError(err)

		s.Assert().Equal(lib.CounterInitializedValue, result.(cadence.Int).Int())
	})

	s.Run("execute at block ID", func() {
		result, err := s.sdkClient.ExecuteScriptAtBlockID(s.ctx, header.ID, []byte(script), nil)
		s.Require().NoError(err)
		s.Assert().Equal(lib.CounterInitializedValue, result.(cadence.Int).Int())
	})
}

func (s *AccessAPISuite) deployContract() *sdk.TransactionResult {
	header, err := s.serviceClient.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	// Deploy the contract
	tx, err := s.serviceClient.DeployContract(s.ctx, header.ID, lib.CounterContract)
	s.Require().NoError(err)

	result, err := s.serviceClient.WaitForStatus(s.ctx, tx.ID(), sdk.TransactionStatusExecuted)
	s.Require().NoError(err)

	// Add counter to service account
	serviceAddress := s.serviceClient.SDKServiceAddress()
	createCounterTx := sdk.NewTransaction().
		SetScript([]byte(lib.CreateCounterTx(serviceAddress).ToCadence())).
		SetReferenceBlockID(sdk.Identifier(header.ID)).
		SetProposalKey(serviceAddress, 0, s.serviceClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		AddAuthorizer(serviceAddress).
		SetGasLimit(9999)

	err = s.serviceClient.SignAndSendTransaction(s.ctx, createCounterTx)
	s.Require().NoError(err)

	result, err = s.serviceClient.WaitForSealed(s.ctx, createCounterTx.ID())
	s.Require().NoError(err)
	s.Require().Empty(result.Error, "create counter tx should be accepted but got: %s", result.Error)

	return result
}

func (s *AccessAPISuite) waitUntilIndexed(height uint64) {
	// wait until the block is indexed
	// This relying on the fact that the API is configured to only use the local db, and will return
	// an error if the height is not indexed yet.
	//
	// TODO: once the indexed height is include in the Access API's metadata response, we can get
	// ride of this
	s.Require().Eventually(func() bool {
		_, err := s.sdkClient.ExecuteScriptAtBlockHeight(s.ctx, height, []byte(fmt.Sprintf(simpleScript, 42)), nil)
		return err == nil
	}, 30*time.Second, 1*time.Second)
}
