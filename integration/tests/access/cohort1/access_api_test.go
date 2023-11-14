package cohort1

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go/integration/tests/mvp"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/cadence"

	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// This is a collection of tests that validate various Access API endpoints work as expected.

var (
	simpleScript       = `pub fun main(): Int { return 42; }`
	simpleScriptResult = cadence.NewInt(42)
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

	accessNode2   *testnet.Container
	an1Client     *client.Client
	an2Client     *client.Client
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
	defaultAccessConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		// make sure test continues to test as expected if the default config changes
		testnet.WithAdditionalFlagf("--script-execution-mode=%s", backend.ScriptExecutionModeExecutionNodesOnly),
	)

	indexingAccessConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlagf("--script-execution-mode=%s", backend.ScriptExecutionModeLocalOnly),
	)

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=100ms"),
		testnet.WithAdditionalFlagf("--required-verification-seal-approvals=%d", 1),
		testnet.WithAdditionalFlagf("--required-construction-seal-approvals=%d", 1),
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
		defaultAccessConfig,

		// Tests will focus on AN2
		indexingAccessConfig,
	}

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.T().Logf("starting flow network with docker containers")
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)

	var err error
	s.accessNode2 = s.net.ContainerByName("access_2")

	s.an2Client, err = s.accessNode2.SDKClient()
	s.Require().NoError(err)

	s.an1Client, err = s.net.ContainerByName(testnet.PrimaryAN).SDKClient()
	s.Require().NoError(err)

	// pause until the network is progressing
	var header *sdk.BlockHeader
	s.Require().Eventually(func() bool {
		header, err = s.an2Client.GetLatestBlockHeader(s.ctx, true)
		s.Require().NoError(err)

		return header.Height > 0
	}, 30*time.Second, 1*time.Second)

	// the service client uses GetAccount and requires the first block to be indexed
	s.Require().Eventually(func() bool {
		s.serviceClient, err = s.accessNode2.TestnetClient()
		return err == nil
	}, 30*time.Second, 1*time.Second)
}

// TestScriptExecutionAndGetAccountsAN1 test the Access API endpoints for executing scripts and getting
// accounts using execution nodes.
//
// Note: not combining AN1, AN2 tests together because that causes a drastic increase in test run times. test cases are read-only
// and should not interfere with each other.
func (s *AccessAPISuite) TestScriptExecutionAndGetAccountsAN1() {
	// deploy the test contract
	txResult := s.deployContract()
	targetHeight := txResult.BlockHeight + 1
	s.waitUntilIndexed(targetHeight)

	// Run tests against Access 1, which uses the execution node
	s.testGetAccount(s.an1Client)
	s.testExecuteScriptWithSimpleScript(s.an1Client)
	s.testExecuteScriptWithSimpleContract(s.an1Client, targetHeight)
}

// TestScriptExecutionAndGetAccountsAN2 test the Access API endpoints for executing scripts and getting
// accounts using local storage.
//
// Note: not combining AN1, AN2 tests together because that causes a drastic increase in test run times. test cases are read-only
// and should not interfere with each other.
func (s *AccessAPISuite) TestScriptExecutionAndGetAccountsAN2() {
	// deploy the test contract
	txResult := s.deployContract()
	targetHeight := txResult.BlockHeight + 1
	s.waitUntilIndexed(targetHeight)

	// Run tests against Access 2, which uses local storage
	s.testGetAccount(s.an2Client)
	s.testExecuteScriptWithSimpleScript(s.an2Client)
	s.testExecuteScriptWithSimpleContract(s.an2Client, targetHeight)
}

func (s *AccessAPISuite) TestMVPScriptExecutionLocalStorage() {
	// this is a specialized test that creates accounts, deposits funds, deploys contracts, etc, and
	// uses the provided access node to handle the Access API calls. there is an existing test that
	// covers the default config, so we only need to test with local storage.
	mvp.RunMVPTest(s.T(), s.ctx, s.net, s.accessNode2)
}

func (s *AccessAPISuite) testGetAccount(client *client.Client) {
	header, err := client.GetLatestBlockHeader(s.ctx, true)
	s.Require().NoError(err)

	serviceAddress := s.serviceClient.SDKServiceAddress()

	s.Run("get account at latest block", func() {
		account, err := s.waitAccountsUntilIndexed(func() (*sdk.Account, error) {
			return client.GetAccount(s.ctx, serviceAddress)
		})
		s.Require().NoError(err)
		s.Assert().Equal(serviceAddress, account.Address)
		s.Assert().NotZero(account.Balance)
	})

	s.Run("get account block ID", func() {
		account, err := s.waitAccountsUntilIndexed(func() (*sdk.Account, error) {
			return client.GetAccountAtLatestBlock(s.ctx, serviceAddress)
		})
		s.Require().NoError(err)
		s.Assert().Equal(serviceAddress, account.Address)
		s.Assert().NotZero(account.Balance)
	})

	s.Run("get account block height", func() {
		account, err := s.waitAccountsUntilIndexed(func() (*sdk.Account, error) {
			return client.GetAccountAtBlockHeight(s.ctx, serviceAddress, header.Height)
		})
		s.Require().NoError(err)
		s.Assert().Equal(serviceAddress, account.Address)
		s.Assert().NotZero(account.Balance)
	})

	s.Run("get newly created account", func() {
		addr, err := utils.CreateFlowAccount(s.ctx, s.serviceClient)
		s.Require().NoError(err)
		acc, err := client.GetAccount(s.ctx, addr)
		s.Require().NoError(err)
		s.Assert().Equal(addr, acc.Address)
	})
}

func (s *AccessAPISuite) testExecuteScriptWithSimpleScript(client *client.Client) {
	header, err := client.GetLatestBlockHeader(s.ctx, true)
	s.Require().NoError(err)

	s.Run("execute at latest block", func() {
		result, err := s.waitScriptExecutionUntilIndexed(func() (cadence.Value, error) {
			return client.ExecuteScriptAtLatestBlock(s.ctx, []byte(simpleScript), nil)
		})
		s.Require().NoError(err)
		s.Assert().Equal(simpleScriptResult, result)
	})

	s.Run("execute at block height", func() {
		result, err := s.waitScriptExecutionUntilIndexed(func() (cadence.Value, error) {
			return client.ExecuteScriptAtBlockHeight(s.ctx, header.Height, []byte(simpleScript), nil)
		})
		s.Require().NoError(err)
		s.Assert().Equal(simpleScriptResult, result)
	})

	s.Run("execute at block ID", func() {
		result, err := s.waitScriptExecutionUntilIndexed(func() (cadence.Value, error) {
			return client.ExecuteScriptAtBlockID(s.ctx, header.ID, []byte(simpleScript), nil)
		})
		s.Require().NoError(err)
		s.Assert().Equal(simpleScriptResult, result)
	})
}

func (s *AccessAPISuite) testExecuteScriptWithSimpleContract(client *client.Client, targetHeight uint64) {
	header, err := client.GetBlockHeaderByHeight(s.ctx, targetHeight)
	s.Require().NoError(err)

	// Check that the initialized value is set
	serviceAccount := s.serviceClient.Account()
	script := lib.ReadCounterScript(serviceAccount.Address, serviceAccount.Address).ToCadence()

	s.Run("execute at latest block", func() {
		result, err := s.waitScriptExecutionUntilIndexed(func() (cadence.Value, error) {
			return client.ExecuteScriptAtLatestBlock(s.ctx, []byte(script), nil)
		})
		s.Require().NoError(err)
		s.Assert().Equal(lib.CounterInitializedValue, result.(cadence.Int).Int())
	})

	s.Run("execute at block height", func() {
		result, err := s.waitScriptExecutionUntilIndexed(func() (cadence.Value, error) {
			return client.ExecuteScriptAtBlockHeight(s.ctx, header.Height, []byte(script), nil)
		})
		s.Require().NoError(err)
		s.Assert().Equal(lib.CounterInitializedValue, result.(cadence.Int).Int())
	})

	s.Run("execute at block ID", func() {
		result, err := s.waitScriptExecutionUntilIndexed(func() (cadence.Value, error) {
			return client.ExecuteScriptAtBlockID(s.ctx, header.ID, []byte(script), nil)
		})
		s.Require().NoError(err)
		s.Assert().Equal(lib.CounterInitializedValue, result.(cadence.Int).Int())
	})

	s.Run("execute at past block height", func() {
		// targetHeight is when the counter was deployed, use a height before that to check that
		// the contract was deployed, but the value was not yet set
		pastHeight := targetHeight - 2

		result, err := client.ExecuteScriptAtBlockHeight(s.ctx, pastHeight, []byte(script), nil)
		s.Require().NoError(err)

		s.Assert().Equal(lib.CounterDefaultValue, result.(cadence.Int).Int())
	})
}

func (s *AccessAPISuite) deployContract() *sdk.TransactionResult {
	header, err := s.serviceClient.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	// Deploy the contract
	tx, err := s.serviceClient.DeployContract(s.ctx, header.ID, lib.CounterContract)
	s.Require().NoError(err)

	_, err = s.serviceClient.WaitForExecuted(s.ctx, tx.ID())
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

	result, err := s.serviceClient.WaitForSealed(s.ctx, createCounterTx.ID())
	s.Require().NoError(err)
	s.Require().Empty(result.Error, "create counter tx should be accepted but got: %s", result.Error)

	return result
}

type getAccount func() (*sdk.Account, error)
type executeScript func() (cadence.Value, error)

var indexDelay = 10 * time.Second
var indexRetry = 100 * time.Millisecond

// wait for sealed block to get indexed, as there is a delay in syncing blocks between nodes
func (s *AccessAPISuite) waitAccountsUntilIndexed(get getAccount) (*sdk.Account, error) {
	var account *sdk.Account
	var err error
	s.Require().Eventually(func() bool {
		account, err = get()
		return notOutOfRangeError(err)
	}, indexDelay, indexRetry)

	return account, err
}

func (s *AccessAPISuite) waitScriptExecutionUntilIndexed(execute executeScript) (cadence.Value, error) {
	var val cadence.Value
	var err error
	s.Require().Eventually(func() bool {
		val, err = execute()
		return notOutOfRangeError(err)
	}, indexDelay, indexRetry)

	return val, err
}

func (s *AccessAPISuite) waitUntilIndexed(height uint64) {
	// wait until the block is indexed
	// This relying on the fact that the API is configured to only use the local db, and will return
	// an error if the height is not indexed yet.
	//
	// TODO: once the indexed height is include in the Access API's metadata response, we can get
	// ride of this
	s.Require().Eventually(func() bool {
		_, err := s.an2Client.ExecuteScriptAtBlockHeight(s.ctx, height, []byte(simpleScript), nil)
		return err == nil
	}, 30*time.Second, 1*time.Second)
}

// make sure we either don't have an error or the error is not out of range error, since in that case we have to wait a bit longer for index to get synced
func notOutOfRangeError(err error) bool {
	statusErr, ok := status.FromError(err)
	if !ok || err == nil {
		return true
	}
	return statusErr.Code() != codes.OutOfRange
}
