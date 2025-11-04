package cohort1

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"
	restclient "github.com/onflow/flow/openapi/go-client-generated"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	mockcommonmodels "github.com/onflow/flow-go/engine/access/rest/common/models/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/integration/tests/mvp"
	"github.com/onflow/flow-go/integration/utils"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// This is a collection of tests that validate various Access API endpoints work as expected.

var (
	simpleScript       = `access(all) fun main(): Int { return 42; }`
	simpleScriptResult = cadence.NewInt(42)

	OriginalContract = dsl.Contract{
		Name: "TestingContract",
		Members: []dsl.CadenceCode{
			dsl.Code(`
				access(all) fun message(): String {
					return "Initial Contract"
				}`,
			),
		},
	}

	UpdatedContract = dsl.Contract{
		Name: "TestingContract",
		Members: []dsl.CadenceCode{
			dsl.Code(`
				access(all) fun message(): String {
					return "Updated Contract"
				}`,
			),
		},
	}
)

const (
	GetMessageScript = `
import TestingContract from 0x%s

access(all)
fun main(): String {
	return TestingContract.message()
}`
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
		testnet.WithAdditionalFlagf("--script-execution-mode=%s", query_mode.IndexQueryModeExecutionNodesOnly),
		testnet.WithAdditionalFlagf("--tx-result-query-mode=%s", query_mode.IndexQueryModeExecutionNodesOnly),
	)

	indexingAccessConfig := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--execution-data-sync-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-data-dir=%s", testnet.DefaultExecutionDataServiceDir),
		testnet.WithAdditionalFlag("--execution-data-retry-delay=1s"),
		testnet.WithAdditionalFlag("--execution-data-indexing-enabled=true"),
		testnet.WithAdditionalFlagf("--execution-state-dir=%s", testnet.DefaultExecutionStateDir),
		testnet.WithAdditionalFlagf("--script-execution-mode=%s", query_mode.IndexQueryModeLocalOnly),
	)

	consensusConfigs := []func(config *testnet.NodeConfig){
		// `cruise-ctl-fallback-proposal-duration` is set to 250ms instead to of 100ms
		// to purposely slow down the block rate. This is needed since the crypto module
		// update providing faster BLS operations.
		// TODO: fix the access integration test logic to function without slowing down
		// the block rate
		testnet.WithAdditionalFlag("--cruise-ctl-fallback-proposal-duration=250ms"),
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
	_ = s.deployContract(lib.CounterContract, false)
	txResult := s.deployCounter()
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
	_ = s.deployContract(lib.CounterContract, false)
	txResult := s.deployCounter()
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

// TestSendAndSubscribeTransactionStatuses tests the functionality of sending and subscribing to transaction statuses.
//
// This test verifies that a transaction can be created, signed, sent to the access API, and then the status of the transaction
// can be subscribed to. It performs the following steps:
// 1. Establishes a connection to the access API.
// 2. Creates a new account key and prepares a transaction for account creation.
// 3. Signs the transaction.
// 4. Sends and subscribes to the transaction status using the access API.
// 5. Verifies the received transaction statuses, ensuring they are received in order and the final status is "SEALED".
func (s *AccessAPISuite) TestSendAndSubscribeTransactionStatuses() {
	accessNodeContainer := s.net.ContainerByName(testnet.PrimaryAN)

	// Establish a gRPC connection to the access API
	conn, err := grpc.Dial(accessNodeContainer.Addr(testnet.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.Require().NotNil(conn)

	// Create a client for the access API
	accessClient := accessproto.NewAccessAPIClient(conn)
	serviceClient, err := accessNodeContainer.TestnetClient()
	s.Require().NoError(err)
	s.Require().NotNil(serviceClient)

	// Get the latest block ID
	latestBlockID, err := serviceClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// Generate a new account transaction
	accountKey := test.AccountKeyGenerator().New()
	payer := serviceClient.SDKServiceAddress()

	tx, err := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
	s.Require().NoError(err)
	tx.SetComputeLimit(1000).
		SetReferenceBlockID(sdk.HexToID(latestBlockID.String())).
		SetProposalKey(payer, 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(payer)

	tx, err = serviceClient.SignTransaction(tx)
	s.Require().NoError(err)

	// Convert the transaction to a message format expected by the access API
	authorizers := make([][]byte, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		authorizers[i] = auth.Bytes()
	}

	convertToMessageSig := func(sigs []sdk.TransactionSignature) []*entities.Transaction_Signature {
		msgSigs := make([]*entities.Transaction_Signature, len(sigs))
		for i, sig := range sigs {
			msgSigs[i] = &entities.Transaction_Signature{
				Address:   sig.Address.Bytes(),
				KeyId:     uint32(sig.KeyIndex),
				Signature: sig.Signature,
			}
		}

		return msgSigs
	}

	transactionMsg := &entities.Transaction{
		Script:           tx.Script,
		Arguments:        tx.Arguments,
		ReferenceBlockId: tx.ReferenceBlockID.Bytes(),
		GasLimit:         tx.GasLimit,
		ProposalKey: &entities.Transaction_ProposalKey{
			Address:        tx.ProposalKey.Address.Bytes(),
			KeyId:          uint32(tx.ProposalKey.KeyIndex),
			SequenceNumber: tx.ProposalKey.SequenceNumber,
		},
		Payer:              tx.Payer.Bytes(),
		Authorizers:        authorizers,
		PayloadSignatures:  convertToMessageSig(tx.PayloadSignatures),
		EnvelopeSignatures: convertToMessageSig(tx.EnvelopeSignatures),
	}

	// Send and subscribe to the transaction status using the access API
	subClient, err := accessClient.SendAndSubscribeTransactionStatuses(s.ctx, &accessproto.SendAndSubscribeTransactionStatusesRequest{
		Transaction:          transactionMsg,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	})
	s.Require().NoError(err)

	expectedCounter := uint64(0)
	lastReportedTxStatus := entities.TransactionStatus_UNKNOWN
	var txID sdk.Identifier

	for {
		resp, err := subClient.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			s.Require().NoError(err)
		}

		if txID == sdk.EmptyID {
			txID = sdk.Identifier(resp.TransactionResults.TransactionId)
		}

		s.Assert().Equal(expectedCounter, resp.GetMessageIndex())
		s.Assert().Equal(txID, sdk.Identifier(resp.TransactionResults.TransactionId))
		// Check if all statuses received one by one. The subscription should send responses for each of the statuses,
		// and the message should be sent in the order of transaction statuses.
		// Expected order: pending(1) -> finalized(2) -> executed(3) -> sealed(4)
		s.Assert().Equal(lastReportedTxStatus, resp.TransactionResults.Status-1)

		expectedCounter++
		lastReportedTxStatus = resp.TransactionResults.Status
	}

	// Check, if the final transaction status is sealed.
	s.Assert().Equal(entities.TransactionStatus_SEALED, lastReportedTxStatus)
}

// TestContractUpdate tests that the Access API can index contract updates, and that the program cache
// is invalidated when a contract is updated.
func (s *AccessAPISuite) TestContractUpdate() {
	txResult := s.deployContract(OriginalContract, false)
	targetHeight := txResult.BlockHeight + 1
	s.waitUntilIndexed(targetHeight)

	script := fmt.Sprintf(GetMessageScript, s.serviceClient.SDKServiceAddress().Hex())

	// execute script and verify we get the original message
	result, err := s.an2Client.ExecuteScriptAtBlockHeight(s.ctx, targetHeight, []byte(script), nil)
	s.Require().NoError(err)
	s.Require().Equal("Initial Contract", string(result.(cadence.String)))

	txResult = s.deployContract(UpdatedContract, true)
	targetHeight = txResult.BlockHeight + 1
	s.waitUntilIndexed(targetHeight)

	// execute script and verify we get the updated message
	result, err = s.an2Client.ExecuteScriptAtBlockHeight(s.ctx, targetHeight, []byte(script), nil)
	s.Require().NoError(err)
	s.Require().Equal("Updated Contract", string(result.(cadence.String)))
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

func (s *AccessAPISuite) deployContract(contract dsl.Contract, isUpdate bool) *sdk.TransactionResult {
	header, err := s.serviceClient.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	// Deploy the contract
	var tx *sdk.Transaction
	if isUpdate {
		tx, err = s.serviceClient.UpdateContract(s.ctx, header.ID, contract)
	} else {
		tx, err = s.serviceClient.DeployContract(s.ctx, header.ID, contract)
	}
	s.Require().NoError(err)

	result, err := s.serviceClient.WaitForExecuted(s.ctx, tx.ID())
	s.Require().NoError(err)
	s.Require().Empty(result.Error, "deploy tx should be accepted but got: %s", result.Error)

	return result
}

func (s *AccessAPISuite) deployCounter() *sdk.TransactionResult {
	header, err := s.serviceClient.GetLatestSealedBlockHeader(s.ctx)
	s.Require().NoError(err)

	// Add counter to service account
	serviceAddress := s.serviceClient.SDKServiceAddress()
	tx := sdk.NewTransaction().
		SetScript([]byte(lib.CreateCounterTx(serviceAddress).ToCadence())).
		SetReferenceBlockID(sdk.Identifier(header.ID)).
		SetProposalKey(serviceAddress, 0, s.serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(serviceAddress).
		AddAuthorizer(serviceAddress).
		SetComputeLimit(9999)

	err = s.serviceClient.SignAndSendTransaction(s.ctx, tx)
	s.Require().NoError(err)

	result, err := s.serviceClient.WaitForSealed(s.ctx, tx.ID())
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

// Helper function to convert signatures with ExtensionData
// - if input `extensionData` is non-nil, its value is used as the transaction signature extension data.
// - if input `extensionData` is nil, `sigs` extension data is used, and input `extensionData` is replaced
func convertToMessageSigWithExtensionData(sigs []sdk.TransactionSignature, extensionData *[]byte) []*entities.Transaction_Signature {
	msgSigs := make([]*entities.Transaction_Signature, len(sigs))

	for i, sig := range sigs {
		if *extensionData == nil {
			// replace extension data by sig.ExtensionData
			newExtensionData := make([]byte, len(sig.ExtensionData))
			copy(newExtensionData, sig.ExtensionData)
			*extensionData = newExtensionData
		}

		msgSigs[i] = &entities.Transaction_Signature{
			Address:       sig.Address.Bytes(),
			KeyId:         uint32(sig.KeyIndex),
			Signature:     sig.Signature,
			ExtensionData: *extensionData,
		}
	}
	return msgSigs
}

// TestTransactionSignaturePlainExtensionData tests that the Access API properly handles the ExtensionData field
// in transaction signatures for different authentication schemes.
func (s *AccessAPISuite) TestTransactionSignaturePlainExtensionData() {
	accessNodeContainer := s.net.ContainerByName(testnet.PrimaryAN)

	// Establish a gRPC connection to the access API
	conn, err := grpc.Dial(accessNodeContainer.Addr(testnet.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.Require().NotNil(conn)
	defer conn.Close()

	// Create a client for the access API
	accessClient := accessproto.NewAccessAPIClient(conn)
	serviceClient, err := accessNodeContainer.TestnetClient()
	s.Require().NoError(err)
	s.Require().NotNil(serviceClient)

	// Get the latest block ID
	latestBlockID, err := serviceClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// Generate a new account transaction
	accountKey := test.AccountKeyGenerator().New()
	payer := serviceClient.SDKServiceAddress()

	tx, err := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
	s.Require().NoError(err)
	tx.SetComputeLimit(1000).
		SetReferenceBlockID(sdk.HexToID(latestBlockID.String())).
		SetProposalKey(payer, 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(payer)

	tx, err = serviceClient.SignTransaction(tx)
	s.Require().NoError(err)

	// Convert the transaction to a message format expected by the access API
	authorizers := make([][]byte, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		authorizers[i] = auth.Bytes()
	}

	// Test cases for different ExtensionData values
	testCases := []struct {
		name          string
		extensionData []byte
		description   string
		expectSuccess bool
	}{
		{
			name:          "plain_scheme_nil",
			extensionData: nil,
			description:   "Plain authentication scheme with nil ExtensionData",
			expectSuccess: true,
		},
		{
			name:          "plain_scheme_empty",
			extensionData: []byte{},
			description:   "Plain authentication scheme with empty ExtensionData",
			expectSuccess: true,
		},
		{
			name:          "plain_scheme_explicit",
			extensionData: []byte{0x0},
			description:   "Plain authentication scheme with explicit ExtensionData",
			expectSuccess: true,
		},
		{
			name:          "custom_extension_data",
			extensionData: []byte{0x02, 0xAA, 0xBB, 0xCC}, // Invalid scheme with custom data
			description:   "Custom ExtensionData with invalid scheme",
			expectSuccess: false, // Expect failure at the access API level due to invalid scheme
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.T().Logf("Testing: %s", tc.description)

			transactionMsg := &entities.Transaction{
				Script:           tx.Script,
				Arguments:        tx.Arguments,
				ReferenceBlockId: tx.ReferenceBlockID.Bytes(),
				GasLimit:         tx.GasLimit,
				ProposalKey: &entities.Transaction_ProposalKey{
					Address:        tx.ProposalKey.Address.Bytes(),
					KeyId:          uint32(tx.ProposalKey.KeyIndex),
					SequenceNumber: tx.ProposalKey.SequenceNumber,
				},
				Payer:              tx.Payer.Bytes(),
				Authorizers:        authorizers,
				PayloadSignatures:  convertToMessageSigWithExtensionData(tx.PayloadSignatures, &tc.extensionData),
				EnvelopeSignatures: convertToMessageSigWithExtensionData(tx.EnvelopeSignatures, &tc.extensionData),
			}

			// Send and subscribe to the transaction status using the access API
			subClient, err := accessClient.SendAndSubscribeTransactionStatuses(s.ctx, &accessproto.SendAndSubscribeTransactionStatusesRequest{
				Transaction:          transactionMsg,
				EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
			})
			s.Require().NoError(err)

			expectedCounter := uint64(0)
			lastReportedTxStatus := entities.TransactionStatus_UNKNOWN
			var txID sdk.Identifier
			var statusCode uint32

			for {
				resp, err := subClient.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					if tc.expectSuccess { // For invalid cases, access API rejects the transaction
						s.Require().NoError(err)
					} else {
						s.Require().Error(err)
						s.Require().ErrorContains(err, "has invalid extension data")
						break
					}
				}

				if txID == sdk.EmptyID {
					txID = sdk.Identifier(resp.TransactionResults.TransactionId)
				}

				s.Assert().Equal(expectedCounter, resp.GetMessageIndex())
				s.Assert().Equal(txID, sdk.Identifier(resp.TransactionResults.TransactionId))

				// Check if all statuses received one by one. The subscription should send responses for each of the statuses,
				// and the message should be sent in the order of transaction statuses.
				// Expected order: pending(1) -> finalized(2) -> executed(3) -> sealed(4)
				s.Assert().Equal(lastReportedTxStatus, resp.TransactionResults.Status-1)

				expectedCounter++
				lastReportedTxStatus = resp.TransactionResults.Status
				statusCode = resp.TransactionResults.GetStatusCode()
			}

			if tc.expectSuccess {
				// Check that the final transaction status is sealed
				s.Assert().Equal(entities.TransactionStatus_SEALED, lastReportedTxStatus)
				s.Assert().Equal(uint32(codes.OK), statusCode, "Expected transaction to be successful, but got status code: %d", statusCode)
			}
		})
	}
}

// TestTransactionSignatureWebAuthnExtensionData tests the WebAuthn authentication scheme with properly constructed extension data.
func (s *AccessAPISuite) TestTransactionSignatureWebAuthnExtensionData() {
	accessNodeContainer := s.net.ContainerByName(testnet.PrimaryAN)

	// Establish a gRPC connection to the access API
	conn, err := grpc.Dial(accessNodeContainer.Addr(testnet.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.Require().NotNil(conn)
	defer conn.Close()

	// Create a client for the access API
	accessClient := accessproto.NewAccessAPIClient(conn)
	serviceClient, err := accessNodeContainer.TestnetClient()
	s.Require().NoError(err)
	s.Require().NotNil(serviceClient)

	// Get the latest block ID
	latestBlockID, err := serviceClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// Generate a new account transaction
	accountKey := test.AccountKeyGenerator().New()
	payer := serviceClient.SDKServiceAddress()

	tx, err := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
	s.Require().NoError(err)
	tx.SetComputeLimit(1000).
		SetReferenceBlockID(sdk.HexToID(latestBlockID.String())).
		SetProposalKey(payer, 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(payer)

	tx, err = serviceClient.SignTransactionWebAuthN(tx)
	s.Require().NoError(err)

	// Convert the transaction to a message format expected by the access API
	authorizers := make([][]byte, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		authorizers[i] = auth.Bytes()
	}
	s.Require().NoError(err)

	// Test WebAuthn extension data with different scenarios
	testCases := []struct {
		name                     string
		extensionDataReplacement []byte // If nil, use the original extension data from the signed transaction
		description              string
		expectSuccess            bool
	}{
		{
			name:                     "webauthn_valid",
			extensionDataReplacement: nil, // Use the original extension data from the signed transaction, which should be valid
			description:              "WebAuthn scheme with minimal extension data",
			expectSuccess:            true,
		},
		{
			name:                     "webauthn_invalid_minimal",
			extensionDataReplacement: []byte{0x1}, // WebAuthn scheme identifier only
			description:              "WebAuthn scheme with minimal extension data",
			expectSuccess:            false, // Should fail validation due to incomplete WebAuthn data
		},
		{
			name:                     "webauthn_invalid_scheme",
			extensionDataReplacement: []byte{0x3, 0x01, 0x02, 0x03}, // Invalid scheme identifier
			description:              "Invalid authentication scheme",
			expectSuccess:            false,
		},
		{
			name:                     "webauthn_malformed_data",
			extensionDataReplacement: []byte{0x1, 0x01, 0x02}, // WebAuthn scheme with malformed data
			description:              "WebAuthn scheme with malformed extension data",
			expectSuccess:            false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.T().Logf("Testing: %s", tc.description)

			transactionMsg := &entities.Transaction{
				Script:           tx.Script,
				Arguments:        tx.Arguments,
				ReferenceBlockId: tx.ReferenceBlockID.Bytes(),
				GasLimit:         tx.GasLimit,
				ProposalKey: &entities.Transaction_ProposalKey{
					Address:        tx.ProposalKey.Address.Bytes(),
					KeyId:          uint32(tx.ProposalKey.KeyIndex),
					SequenceNumber: tx.ProposalKey.SequenceNumber,
				},
				Payer:              tx.Payer.Bytes(),
				Authorizers:        authorizers,
				PayloadSignatures:  convertToMessageSigWithExtensionData(tx.PayloadSignatures, nil),
				EnvelopeSignatures: convertToMessageSigWithExtensionData(tx.EnvelopeSignatures, &tc.extensionDataReplacement),
			}

			// Validate that the ExtensionData is set correctly before sending
			for _, sig := range transactionMsg.EnvelopeSignatures {
				// For these test cases specifically, we expect ExtensionData to be set
				s.Assert().GreaterOrEqual(len(sig.ExtensionData), 1, "ExtensionData should have at least 1 byte for scheme identifier")
			}

			// Send and subscribe to the transaction status using the access API
			subClient, err := accessClient.SendAndSubscribeTransactionStatuses(s.ctx, &accessproto.SendAndSubscribeTransactionStatusesRequest{
				Transaction:          transactionMsg,
				EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
			})

			s.Require().NoError(err)

			expectedCounter := uint64(0)
			lastReportedTxStatus := entities.TransactionStatus_UNKNOWN
			var txID sdk.Identifier
			var statusCode uint32
			var errorMessage string

			for {
				resp, err := subClient.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					if tc.expectSuccess { // For invalid cases, access API rejects the transaction
						s.Require().NoError(err)
					} else {
						s.Require().Error(err)
						s.Require().ErrorContains(err, "has invalid extension data")
						break
					}
				}

				if txID == sdk.EmptyID {
					txID = sdk.Identifier(resp.TransactionResults.TransactionId)
				}

				s.Assert().Equal(expectedCounter, resp.GetMessageIndex())
				s.Assert().Equal(txID, sdk.Identifier(resp.TransactionResults.TransactionId))

				// Check if all statuses received one by one. The subscription should send responses for each of the statuses,
				// and the message should be sent in the order of transaction statuses.
				// Expected order: pending(1) -> finalized(2) -> executed(3) -> sealed(4)
				s.Assert().Equal(lastReportedTxStatus, resp.TransactionResults.Status-1)

				expectedCounter++
				lastReportedTxStatus = resp.TransactionResults.Status
				statusCode = resp.TransactionResults.GetStatusCode()
				errorMessage = resp.TransactionResults.GetErrorMessage()
			}

			if tc.expectSuccess {
				// Check that the final transaction status is sealed
				s.Assert().Equal(entities.TransactionStatus_SEALED, lastReportedTxStatus)

				if !tc.expectSuccess {
					// For invalid cases, we expect the transaction to be rejected
					s.Assert().NotEqual(uint32(codes.OK), statusCode, "Expected transaction to fail, but got status code: %d", statusCode)
					return
				}

				s.Assert().Equal(uint32(codes.OK), statusCode, "Expected transaction to be successful, but got status code: %d with message: %s", statusCode, errorMessage)
			}
		})
	}
}

// TestExtensionDataPreservation tests that the ExtensionData field is properly preserved
// when transactions are submitted and retrieved through the Access API.
func (s *AccessAPISuite) TestExtensionDataPreservation() {
	accessNodeContainer := s.net.ContainerByName(testnet.PrimaryAN)

	// Establish a gRPC connection to the access API
	conn, err := grpc.Dial(accessNodeContainer.Addr(testnet.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.Require().NotNil(conn)
	defer conn.Close()

	// Create a client for the access API
	accessClient := accessproto.NewAccessAPIClient(conn)
	serviceClient, err := accessNodeContainer.TestnetClient()
	s.Require().NoError(err)
	s.Require().NotNil(serviceClient)

	// Get the latest block ID
	latestBlockID, err := serviceClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// Generate a new account transaction
	accountKey := test.AccountKeyGenerator().New()
	payer := serviceClient.SDKServiceAddress()

	// Test with different ExtensionData values to ensure they are preserved
	testCases := []struct {
		name          string
		extensionData []byte
		description   string
	}{
		{
			name:          "plain_scheme_preservation",
			extensionData: []byte{0x0},
			description:   "Plain authentication scheme ExtensionData preservation",
		},
		{
			name:          "webauthn_scheme_preservation",
			extensionData: nil, // valid extension data is populated below in the test
			description:   "WebAuthn authentication scheme ExtensionData preservation",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.T().Logf("Testing: %s", tc.description)

			tx, err := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
			s.Require().NoError(err)
			tx.SetComputeLimit(1000).
				SetReferenceBlockID(sdk.HexToID(latestBlockID.String())).
				SetProposalKey(payer, 0, serviceClient.GetAndIncrementSeqNumber()).
				SetPayer(payer)

			switch tc.name {
			case "plain_scheme_preservation":
				tx, err = serviceClient.SignTransaction(tx)
			case "webauthn_scheme_preservation":
				tx, err = serviceClient.SignTransactionWebAuthN(tx)
			default:
				err = fmt.Errorf("test must be signed for plain or webauthn schemes")
			}
			s.Require().NoError(err)

			// Convert the transaction to a message format expected by the access API
			authorizers := make([][]byte, len(tx.Authorizers))
			for i, auth := range tx.Authorizers {
				authorizers[i] = auth.Bytes()
			}

			transactionMsg := &entities.Transaction{
				Script:           tx.Script,
				Arguments:        tx.Arguments,
				ReferenceBlockId: tx.ReferenceBlockID.Bytes(),
				GasLimit:         tx.GasLimit,
				ProposalKey: &entities.Transaction_ProposalKey{
					Address:        tx.ProposalKey.Address.Bytes(),
					KeyId:          uint32(tx.ProposalKey.KeyIndex),
					SequenceNumber: tx.ProposalKey.SequenceNumber,
				},
				Payer:              tx.Payer.Bytes(),
				Authorizers:        authorizers,
				PayloadSignatures:  convertToMessageSigWithExtensionData(tx.PayloadSignatures, &tc.extensionData),
				EnvelopeSignatures: convertToMessageSigWithExtensionData(tx.EnvelopeSignatures, &tc.extensionData),
			}

			// Validate that the ExtensionData is set correctly before sending in the webauthn case
			if tc.name == "webauthn_scheme_preservation" {
				for _, sig := range transactionMsg.EnvelopeSignatures {
					// For these test cases specifically, we expect ExtensionData to be at least 2 bytes
					s.Assert().GreaterOrEqual(len(sig.ExtensionData), 2, "ExtensionData should have at least 2 byte for webauthn scheme")
				}
			}

			// Send and subscribe to the transaction status using the access API
			subClient, err := accessClient.SendAndSubscribeTransactionStatuses(s.ctx, &accessproto.SendAndSubscribeTransactionStatusesRequest{
				Transaction:          transactionMsg,
				EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
			})
			s.Require().NoError(err)

			var txID sdk.Identifier
			lastReportedTxStatus := entities.TransactionStatus_UNKNOWN

			// Wait for the transaction to be sealed
			for {
				resp, err := subClient.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					s.Require().NoError(err)
				}

				if txID == sdk.EmptyID {
					txID = sdk.Identifier(resp.TransactionResults.TransactionId)
				}

				lastReportedTxStatus = resp.TransactionResults.Status

				if lastReportedTxStatus == entities.TransactionStatus_SEALED {
					break
				}
			}

			// Verify the transaction was sealed
			s.Assert().Equal(entities.TransactionStatus_SEALED, lastReportedTxStatus)

			// Now retrieve the transaction and verify ExtensionData is preserved
			s.T().Logf("Transaction %s was successfully processed with ExtensionData: %v", txID, tc.extensionData)

			txFromAccess, err := accessClient.GetTransaction(s.ctx, &accessproto.GetTransactionRequest{
				Id: txID.Bytes(),
			})
			s.Require().NoError(err)

			// Verify the retrieved transaction matches the original
			envelopSigs := txFromAccess.GetTransaction().EnvelopeSignatures
			s.Assert().Equal(tc.extensionData, envelopSigs[0].ExtensionData, "ExtensionData should be preserved in the envelope signature")
		})
	}
}

// TestInvalidTransactionSignature tests that the access API performs sanity checks
// on the transaction signature format and rejects invalid formats
func (s *AccessAPISuite) TestRejectedInvalidSignatureFormat() {
	accessNodeContainer := s.net.ContainerByName(testnet.PrimaryAN)

	// Establish a gRPC connection to the access API
	conn, err := grpc.Dial(accessNodeContainer.Addr(testnet.GRPCPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.Require().NotNil(conn)
	defer conn.Close()

	// Create a client for the access API
	accessClient := accessproto.NewAccessAPIClient(conn)
	serviceClient, err := accessNodeContainer.TestnetClient()
	s.Require().NoError(err)
	s.Require().NotNil(serviceClient)

	// Get the latest block ID
	latestBlockID, err := serviceClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// Generate a new account transaction
	accountKey := test.AccountKeyGenerator().New()
	payer := serviceClient.SDKServiceAddress()

	tx, err := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
	s.Require().NoError(err)
	tx.SetComputeLimit(1000).
		SetReferenceBlockID(sdk.HexToID(latestBlockID.String())).
		SetProposalKey(payer, 0, serviceClient.GetAndIncrementSeqNumber()).
		SetPayer(payer)

	tx, err = serviceClient.SignTransaction(tx)
	s.Require().NoError(err)

	// Convert the transaction to a message format expected by the access API
	authorizers := make([][]byte, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		authorizers[i] = auth.Bytes()
	}

	// Test with different ExtensionData values to ensure they are preserved
	testCases := []struct {
		name          string
		extensionData []byte
	}{
		{
			name:          "invalid_plain_scheme",
			extensionData: []byte{0x0, 0x1},
		},
		{
			name:          "invalid_webauthn_scheme",
			extensionData: []byte{0x1, 0x2, 0x3, 0x4, 0x5},
		},
		{
			name:          "invalid_authentication_scheme",
			extensionData: []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.T().Logf("Testing: %s", tc.name)

			transactionMsg := &entities.Transaction{
				Script:           tx.Script,
				Arguments:        tx.Arguments,
				ReferenceBlockId: tx.ReferenceBlockID.Bytes(),
				GasLimit:         tx.GasLimit,
				ProposalKey: &entities.Transaction_ProposalKey{
					Address:        tx.ProposalKey.Address.Bytes(),
					KeyId:          uint32(tx.ProposalKey.KeyIndex),
					SequenceNumber: tx.ProposalKey.SequenceNumber,
				},
				Payer:              tx.Payer.Bytes(),
				Authorizers:        authorizers,
				PayloadSignatures:  convertToMessageSigWithExtensionData(tx.PayloadSignatures, &tc.extensionData),
				EnvelopeSignatures: convertToMessageSigWithExtensionData(tx.EnvelopeSignatures, &tc.extensionData),
			}

			// Send and subscribe to the transaction status using the access API
			subClient, err := accessClient.SendAndSubscribeTransactionStatuses(s.ctx, &accessproto.SendAndSubscribeTransactionStatusesRequest{
				Transaction:          transactionMsg,
				EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
			})
			s.Require().NoError(err)

			// check that the tx submission errors at the access API level
			_, err = subClient.Recv()
			s.Require().Error(err)
			s.Require().ErrorContains(err, "has invalid extension data")
		})
	}
}

// TestScheduledTransactions tests that scheduled transactions are properly indexed and can be
// retrieved through the Access API.
func (s *AccessAPISuite) TestScheduledTransactions() {
	sc := systemcontracts.SystemContractsForChain(s.net.Root().HeaderBody.ChainID)

	accessClient, err := s.net.ContainerByName("access_2").TestnetClient()
	s.Require().NoError(err)
	rpcClient := s.an2Client.RPCClient()

	// Deploy the test contract first
	deployTxID, err := lib.DeployScheduledTransactionsTestContract(accessClient, sc)
	require.NoError(s.T(), err, "could not deploy test contract")

	// wait for the tx to be sealed before attempting to schedule the callback. this helps make sure
	// the proposer's sequence number is updated.
	_, err = accessClient.WaitForSealed(s.ctx, deployTxID)
	s.Require().NoError(err)

	// Schedule a callback for 10 seconds in the future. Use a larger wait time to ensure that there
	// is enough time to submit the tx even on slower CI machines.
	futureTimestamp := time.Now().Unix() + int64(10)

	s.T().Logf("scheduling transaction at timestamp: %v, current timestamp: %v", futureTimestamp, time.Now().Unix())
	callbackID, err := lib.ScheduleCallbackAtTimestamp(futureTimestamp, accessClient, sc)
	require.NoError(s.T(), err, "could not schedule transaction")
	s.T().Logf("scheduled transaction with ID: %d", callbackID)

	// construct the pending execution event using the parameters used by ScheduleCallbackAtTimestamp
	g := fixtures.NewGeneratorSuite()
	expectedPendingExecutionEvent := g.PendingExecutionEvents().Fixture(
		fixtures.PendingExecutionEvent.WithID(callbackID),
		fixtures.PendingExecutionEvent.WithPriority(0), // high priority
		fixtures.PendingExecutionEvent.WithExecutionEffort(1000),
	)

	// construct the expected scheduled transaction body to compare to the API responses
	scheduledTxs, err := blueprints.ExecuteCallbacksTransactions(s.net.Root().ChainID.Chain(), []flow.Event{expectedPendingExecutionEvent})
	require.NoError(s.T(), err, "could not execute callback transaction")
	expectedTx := scheduledTxs[0]

	// Block until the API returns the scheduled transaction.
	require.Eventually(s.T(), func() bool {
		_, err := rpcClient.GetScheduledTransaction(s.ctx, &accessproto.GetScheduledTransactionRequest{Id: callbackID})
		return err == nil
	}, 30*time.Second, 500*time.Millisecond)

	// test gRPC endpoints
	scheduledTxResult := s.testScheduledTransactionsGrpc(callbackID, expectedTx)

	// test REST endpoints
	s.testScheduledTransactionsRest(callbackID, expectedTx, scheduledTxResult)
}

func (s *AccessAPISuite) testScheduledTransactionsGrpc(callbackID uint64, expectedTx *flow.TransactionBody) *accessmodel.TransactionResult {
	expectedTxID := expectedTx.ID()
	rpcClient := s.an2Client.RPCClient()

	// Verify the results of the scheduled transaction and its result.
	s.Run("GetScheduledTransaction", func() {
		scheduledTxResponse, err := rpcClient.GetScheduledTransaction(s.ctx, &accessproto.GetScheduledTransactionRequest{Id: callbackID})
		s.Require().NoError(err)

		actual, err := convert.MessageToTransaction(scheduledTxResponse.GetTransaction(), s.net.Root().ChainID.Chain())
		s.Require().NoError(err)
		s.Require().Equal(expectedTxID, actual.ID())
	})

	var scheduledTxResult *accessmodel.TransactionResult
	s.Run("GetScheduledTransactionResult", func() {
		scheduledTxResultResponse, err := rpcClient.GetScheduledTransactionResult(s.ctx, &accessproto.GetScheduledTransactionResultRequest{Id: callbackID})
		s.Require().NoError(err)

		actual, err := convert.MessageToTransactionResult(scheduledTxResultResponse)
		s.Require().NoError(err)

		s.Greater(actual.BlockHeight, uint64(0))                 // make block height is set
		s.NotEqual(flow.ZeroID, flow.Identifier(actual.BlockID)) // make sure block id is set
		s.Equal(expectedTxID, actual.TransactionID)
		s.Equal(flow.TransactionStatusSealed, actual.Status)
		s.Equal(uint(0), actual.StatusCode)
		s.Empty(actual.ErrorMessage)

		scheduledTxResult = actual
	})

	s.Run("GetTransaction", func() {
		txReponse, err := rpcClient.GetTransaction(s.ctx, &accessproto.GetTransactionRequest{Id: expectedTxID[:]})
		s.Require().NoError(err)

		actualTx, err := convert.MessageToTransaction(txReponse.GetTransaction(), s.net.Root().ChainID.Chain())
		s.Require().NoError(err)
		s.Equal(expectedTxID, actualTx.ID())
	})

	blockID := scheduledTxResult.BlockID
	s.Run("GetTransactionResult", func() {
		txResultResponse, err := rpcClient.GetTransactionResult(s.ctx, &accessproto.GetTransactionRequest{Id: expectedTxID[:], BlockId: blockID[:]})
		s.Require().NoError(err)

		actualTxResult, err := convert.MessageToTransactionResult(txResultResponse)
		s.Require().NoError(err)
		s.Equal(scheduledTxResult, actualTxResult)
	})

	return scheduledTxResult
}

func (s *AccessAPISuite) testScheduledTransactionsRest(callbackID uint64, expectedTx *flow.TransactionBody, scheduledTxResult *accessmodel.TransactionResult) {
	expectedTxID := expectedTx.ID()

	restClient := s.RestClient("access_2")

	link := mockcommonmodels.NewLinkGenerator(s.T())
	link.On("TransactionLink", expectedTxID).Return(fmt.Sprintf("/v1/transactions/%s", expectedTxID.String()), nil)
	link.On("TransactionResultLink", expectedTxID).Return(fmt.Sprintf("/v1/transaction_results/%s", expectedTxID.String()), nil)

	s.Run("REST /v1/transactions/{txID} without result", func() {
		txResponse, _, err := restClient.TransactionsApi.TransactionsIdGet(s.ctx, expectedTxID.String(), nil)
		s.Require().NoError(err)

		var expected commonmodels.Transaction
		expected.Build(expectedTx, nil, link)

		assertSystemTxResponse(s.T(), expected, &txResponse)
	})

	s.Run("REST /v1/transactions/{txID} with result", func() {
		txResponse, _, err := restClient.TransactionsApi.TransactionsIdGet(s.ctx, expectedTxID.String(), &restclient.TransactionsApiTransactionsIdGetOpts{
			Expand: optional.NewInterface("result"),
		})
		s.Require().NoError(err)

		var expected commonmodels.Transaction
		expected.Build(expectedTx, scheduledTxResult, link)

		assertSystemTxResponse(s.T(), expected, &txResponse)
	})

	s.Run("REST /v1/transactions/{scheduledTxID} without result", func() {
		txResponse, _, err := restClient.TransactionsApi.TransactionsIdGet(s.ctx, fmt.Sprint(callbackID), nil)
		s.Require().NoError(err)

		var expected commonmodels.Transaction
		expected.Build(expectedTx, nil, link)

		assertSystemTxResponse(s.T(), expected, &txResponse)
	})

	s.Run("REST /v1/transactions/{scheduledTxID} with result", func() {
		txResponse, _, err := restClient.TransactionsApi.TransactionsIdGet(s.ctx, fmt.Sprint(callbackID), &restclient.TransactionsApiTransactionsIdGetOpts{
			Expand: optional.NewInterface("result"),
		})
		s.Require().NoError(err)

		var expected commonmodels.Transaction
		expected.Build(expectedTx, scheduledTxResult, link)

		assertSystemTxResponse(s.T(), expected, &txResponse)
	})

	s.Run("REST /v1/transaction_results/{txID}", func() {
		txResultResponse, _, err := restClient.TransactionsApi.TransactionResultsTransactionIdGet(s.ctx, expectedTxID.String(), nil)
		s.Require().NoError(err)

		var expected commonmodels.TransactionResult
		expected.Build(scheduledTxResult, expectedTxID, link)

		assertSystemTxResultResponse(s.T(), &expected, &txResultResponse)
	})

	s.Run("REST /v1/transaction_results/{scheduledTxID}", func() {
		txResultResponse, _, err := restClient.TransactionsApi.TransactionResultsTransactionIdGet(s.ctx, fmt.Sprint(callbackID), nil)
		s.Require().NoError(err)

		var expected commonmodels.TransactionResult
		expected.Build(scheduledTxResult, expectedTxID, link)

		assertSystemTxResultResponse(s.T(), &expected, &txResultResponse)
	})
}

func (s *AccessAPISuite) RestClient(containerName string) *restclient.APIClient {
	restAddr := s.net.ContainerByName(containerName).Addr(testnet.RESTPort)

	config := restclient.NewConfiguration()
	config.BasePath = fmt.Sprintf("http://%s/v1", restAddr)
	return restclient.NewAPIClient(config)
}

// TestSystemTransactions tests getting a system transaction using each of the supported endpoints.
func (s *AccessAPISuite) TestSystemTransactions() {
	rpcClient := s.an2Client.RPCClient()

	// block until a few blocks have executed to ensure there are blocks with system transactions
	var blockID flow.Identifier
	require.Eventually(s.T(), func() bool {
		header, err := rpcClient.GetBlockHeaderByHeight(s.ctx, &accessproto.GetBlockHeaderByHeightRequest{Height: 5})
		if err == nil {
			blockID = convert.MessageToIdentifier(header.GetBlock().GetId())
			return true
		}
		return false
	}, 30*time.Second, 500*time.Millisecond)

	// construct the expected system collection
	systemCollection, err := blueprints.SystemCollection(s.net.Root().ChainID.Chain(), nil)
	s.Require().NoError(err)
	s.Require().Len(systemCollection.Transactions, 2)

	txResults := s.testSystemTransactionsGrpc(systemCollection, blockID)
	s.testSystemTransactionsRest(systemCollection, blockID, txResults)
}

func (s *AccessAPISuite) testSystemTransactionsGrpc(systemCollection *flow.Collection, blockID flow.Identifier) []*accessmodel.TransactionResult {
	rpcClient := s.an2Client.RPCClient()

	systemTxs := make([]flow.Identifier, len(systemCollection.Transactions))
	for i, tx := range systemCollection.Transactions {
		systemTxs[i] = tx.ID()
	}

	// query the system transactions using each of the supported endpoints and verify the results are correct
	s.Run("GetTransaction", func() {
		for _, txID := range systemTxs {
			txReponse, err := rpcClient.GetTransaction(s.ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
			s.Require().NoError(err)

			actualTx, err := convert.MessageToTransaction(txReponse.GetTransaction(), s.net.Root().ChainID.Chain())
			s.Require().NoError(err)
			s.Equal(txID, actualTx.ID())
		}
	})

	s.Run("GetSystemTransaction", func() {
		for _, txID := range systemTxs {
			systemTxResponse, err := rpcClient.GetSystemTransaction(s.ctx, &accessproto.GetSystemTransactionRequest{Id: txID[:], BlockId: blockID[:]})
			s.Require().NoError(err)

			actualSystemTx, err := convert.MessageToTransaction(systemTxResponse.GetTransaction(), s.net.Root().ChainID.Chain())
			s.Require().NoError(err)
			s.Equal(txID, actualSystemTx.ID())
		}
	})

	var txResults []*accessmodel.TransactionResult
	s.Run("GetTransactionResult", func() {
		for _, txID := range systemTxs {
			txResultResponse, err := rpcClient.GetTransactionResult(s.ctx, &accessproto.GetTransactionRequest{Id: txID[:], BlockId: blockID[:]})
			s.Require().NoError(err)

			actualTxResult, err := convert.MessageToTransactionResult(txResultResponse)
			s.Require().NoError(err)

			s.Equal(txID, actualTxResult.TransactionID)
			s.Equal(blockID, actualTxResult.BlockID)
			s.Equal(uint(0), actualTxResult.StatusCode)
			s.Empty(actualTxResult.ErrorMessage)

			txResults = append(txResults, actualTxResult)
		}
	})

	s.Run("GetSystemTransactionResult", func() {
		for _, txID := range systemTxs {
			systemTxResultResponse, err := rpcClient.GetSystemTransactionResult(s.ctx, &accessproto.GetSystemTransactionResultRequest{Id: txID[:], BlockId: blockID[:]})
			s.Require().NoError(err)

			actualSystemTxResult, err := convert.MessageToTransactionResult(systemTxResultResponse)
			s.Require().NoError(err)

			s.Equal(txID, actualSystemTxResult.TransactionID)
			s.Equal(blockID, actualSystemTxResult.BlockID)
			s.Equal(uint(0), actualSystemTxResult.StatusCode)
			s.Empty(actualSystemTxResult.ErrorMessage)
		}
	})

	return txResults
}

func (s *AccessAPISuite) testSystemTransactionsRest(systemCollection *flow.Collection, blockID flow.Identifier, txResults []*accessmodel.TransactionResult) {
	restClient := s.RestClient("access_2")

	link := mockcommonmodels.NewLinkGenerator(s.T())
	for _, tx := range systemCollection.Transactions {
		txID := tx.ID()
		link.On("TransactionLink", txID).Return(fmt.Sprintf("/v1/transactions/%s", txID.String()), nil)
		link.On("TransactionResultLink", txID).Return(fmt.Sprintf("/v1/transaction_results/%s", txID.String()), nil)
	}

	s.Run("REST /v1/transactions/{txID} without result", func() {
		for _, tx := range systemCollection.Transactions {
			txID := tx.ID()

			txResponse, _, err := restClient.TransactionsApi.TransactionsIdGet(s.ctx, txID.String(), nil)
			s.Require().NoError(err)

			var expected commonmodels.Transaction
			expected.Build(tx, nil, link)

			assertSystemTxResponse(s.T(), expected, &txResponse)
		}
	})

	s.Run("REST /v1/transactions/{txID} with result", func() {
		for i, tx := range systemCollection.Transactions {
			txID := tx.ID()
			txResult := txResults[i]

			txResponse, _, err := restClient.TransactionsApi.TransactionsIdGet(s.ctx, txID.String(), &restclient.TransactionsApiTransactionsIdGetOpts{
				Expand:  optional.NewInterface("result"),
				BlockId: optional.NewInterface(blockID.String()), // required to get system tx result
			})
			s.Require().NoError(err)

			var expected commonmodels.Transaction
			expected.Build(tx, txResult, link)

			assertSystemTxResponse(s.T(), expected, &txResponse)
		}
	})

	s.Run("REST /v1/transaction_results/{txID}", func() {
		for i, tx := range systemCollection.Transactions {
			txID := tx.ID()
			txResult := txResults[i]

			txResultResponse, _, err := restClient.TransactionsApi.TransactionResultsTransactionIdGet(s.ctx, txID.String(), &restclient.TransactionsApiTransactionResultsTransactionIdGetOpts{
				BlockId: optional.NewInterface(blockID.String()), // required to get system tx result
			})
			s.Require().NoError(err)

			var expected commonmodels.TransactionResult
			expected.Build(txResult, txID, link)

			assertSystemTxResultResponse(s.T(), &expected, &txResultResponse)
		}
	})
}

func assertSystemTxResponse(t *testing.T, expected commonmodels.Transaction, actual *restclient.Transaction) {
	assert.Equal(t, expected.Script, actual.Script)
	assert.Equal(t, expected.Arguments, actual.Arguments)
	assert.Equal(t, expected.ReferenceBlockId, actual.ReferenceBlockId)
	assert.Equal(t, expected.GasLimit, actual.GasLimit)
	assert.Equal(t, expected.Payer, actual.Payer)
	assert.Equal(t, expected.Authorizers, actual.Authorizers)

	// these should always be empty for scheduled tx
	assert.Equal(t, flow.EmptyAddress.Hex(), actual.ProposalKey.Address)
	assert.Equal(t, "0", actual.ProposalKey.KeyIndex)
	assert.Equal(t, "0", actual.ProposalKey.SequenceNumber)
	assert.Len(t, actual.PayloadSignatures, 0)
	assert.Len(t, actual.EnvelopeSignatures, 0)

	assert.Equal(t, expected.Expandable.Result, actual.Expandable.Result)
	assert.Equal(t, expected.Links.Self, actual.Links.Self)

	if expected.Result == nil {
		assert.Nil(t, actual.Result)
	} else {
		assertSystemTxResultResponse(t, expected.Result, actual.Result)
	}
}

func assertSystemTxResultResponse(t *testing.T, expected *commonmodels.TransactionResult, actual *restclient.TransactionResult) {
	assert.Equal(t, expected.BlockId, actual.BlockId)
	assert.Equal(t, expected.CollectionId, actual.CollectionId)
	assert.Equal(t, string(*expected.Execution), string(*actual.Execution))
	assert.Equal(t, string(*expected.Status), string(*actual.Status))
	assert.Equal(t, expected.StatusCode, actual.StatusCode)
	assert.Equal(t, expected.ErrorMessage, actual.ErrorMessage)
	assert.Equal(t, expected.ComputationUsed, actual.ComputationUsed)
	assert.Equal(t, expected.Links.Self, actual.Links.Self)
	assert.Equal(t, len(expected.Events), len(actual.Events))
	for i, event := range expected.Events {
		actualEvent := actual.Events[i]
		assert.Equal(t, event.Type_, actualEvent.Type_)
		assert.Equal(t, event.TransactionId, actualEvent.TransactionId)
		assert.Equal(t, event.TransactionIndex, actualEvent.TransactionIndex)
		assert.Equal(t, event.EventIndex, actualEvent.EventIndex)
		assert.Equal(t, event.Payload, actualEvent.Payload)
	}
}
