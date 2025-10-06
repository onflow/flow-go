package cohort4

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go/integration/client"
	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/store"
)

const maxReceiptHeightMetric = "access_ingestion_max_receipt_height"

func TestAccessStoreTxErrorMessages(t *testing.T) {
	suite.Run(t, new(AccessStoreTxErrorMessagesSuite))
}

// AccessStoreTxErrorMessagesSuite tests the access for storing transaction error messages.
type AccessStoreTxErrorMessagesSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork

	accessContainerName string
}

func (s *AccessStoreTxErrorMessagesSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

// SetupTest sets up the test suite by starting the network.
// The access are started with correct parameters and store transaction error messages.
func (s *AccessStoreTxErrorMessagesSuite) SetupTest() {
	defaultAccess := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
	)

	s.accessContainerName = "access_2"
	storeTxAccess := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlagf("--store-tx-result-error-messages=true"),
		testnet.WithMetricsServer(),
	)

	consensusConfigs := []func(config *testnet.NodeConfig){
		// `cruise-ctl-fallback-proposal-duration` is set to 250ms instead to of 100ms
		// to purposely slow down the block rate. This is needed since the crypto module
		// update providing faster BLS operations.
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

		defaultAccess,
		storeTxAccess,
	}

	// prepare the network
	conf := testnet.NewNetworkConfig("access_store_tx_error_messages_test", nodeConfigs)
	s.net = testnet.PrepareFlowNetwork(s.T(), conf, flow.Localnet)

	// start the network
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.net.Start(s.ctx)
}

// TestAccessStoreTxErrorMessages verifies that transaction result error messages
// are stored correctly in the database by sending a transaction, generating an error,
// and checking if the error message is properly stored and retrieved from the database.
func (s *AccessStoreTxErrorMessagesSuite) TestAccessStoreTxErrorMessages() {
	// Create and send a transaction that will result in an error.
	txResult := s.createAndSendTxWithTxError()

	// Wait until execution receipts are handled, transaction error messages are stored.
	s.Eventually(func() bool {
		value, err := s.getMaxReceiptHeight(s.accessContainerName)
		return err == nil && value > txResult.BlockHeight
	}, 60*time.Second, 1*time.Second)

	// Stop the network containers before checking the results.
	s.net.StopContainers()

	txResults := []*sdk.TransactionResult{txResult}
	txErrorMessages := s.fetchTxErrorMessages(txResults, s.accessContainerName)
	s.verifyTxErrorMessage(txResults, txErrorMessages)
}

// TestBackfillTxErrorMessages verifies that transaction error messages are backfilled correctly
// by creating a transaction that results in an error, running the backfill command, and checking
// if the error message is stored and retrieved from the database.
func (s *AccessStoreTxErrorMessagesSuite) TestBackfillTxErrorMessages() {
	// Create and send a transactions that will result in an error.
	transactionCount := 5
	txResults := make([]*sdk.TransactionResult, transactionCount)
	for i := 0; i < transactionCount; i++ {
		txResults[i] = s.createAndSendTxWithTxError()
	}

	serverAddr := fmt.Sprintf("localhost:%s", s.net.ContainerByName(s.accessContainerName).Port(testnet.AdminPort))
	adminClient := client.NewAdminClient(serverAddr)

	startHeight := 1
	endHeight := txResults[len(txResults)-1].BlockHeight // last tx result block height
	data := map[string]interface{}{"start-height": startHeight, "endHeight": endHeight}
	// executes the backfill command for transaction error messages
	_, err := adminClient.RunCommand(context.Background(), "backfill-tx-error-messages", data)
	require.NoError(s.T(), err)

	// Stop the network containers before checking the results.
	s.net.StopContainers()

	txErrorMessages := s.fetchTxErrorMessages(txResults, s.accessContainerName)
	s.verifyTxErrorMessage(txResults, txErrorMessages)
}

// createAndSendTxWithTxError creates and sends a transaction that will result in an error.
// This function creates a new account, causing an error during execution.
func (s *AccessStoreTxErrorMessagesSuite) createAndSendTxWithTxError() *sdk.TransactionResult {
	// prepare environment to create a new account
	serviceAccountClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	latestBlockID, err := serviceAccountClient.GetLatestBlockID(s.ctx)
	s.Require().NoError(err)

	// create new account to deploy Counter to
	accountPrivateKey := lib.RandomPrivateKey()

	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(serviceAccountClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx, err := templates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]templates.Contract{}, serviceAddress)
	s.Require().NoError(err)

	// Generate the account creation transaction
	createAccountTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 1, serviceAccountClient.GetAndIncrementSeqNumber()).
		SetPayer(serviceAddress).
		SetComputeLimit(9999)

	// Sign and send the transaction.
	err = serviceAccountClient.SignAndSendTransaction(s.ctx, createAccountTx)
	s.Require().NoError(err)

	// Wait for the transaction to be sealed and return the transaction result.
	accountCreationTxRes, err := serviceAccountClient.WaitForSealed(s.ctx, createAccountTx.ID())
	s.Require().NoError(err)

	return accountCreationTxRes
}

// getMaxReceiptHeight retrieves the maximum receipt height for a given container by
// querying the metrics endpoint. This is used to confirm that the transaction receipts
// have been processed.
func (s *AccessStoreTxErrorMessagesSuite) getMaxReceiptHeight(containerName string) (uint64, error) {
	node := s.net.ContainerByName(containerName)
	metricsURL := fmt.Sprintf("http://0.0.0.0:%s/metrics", node.Port(testnet.MetricsPort))
	values := s.net.GetMetricFromContainer(s.T(), containerName, metricsURL, maxReceiptHeightMetric)

	// If no values are found in the metrics, return an error.
	if len(values) == 0 {
		return 0, fmt.Errorf("no values found")
	}

	// Return the first value found as the max receipt height.
	return uint64(values[0].GetGauge().GetValue()), nil
}

// fetchTxErrorMessage retrieves the stored transaction error message for a given transaction result.
func (s *AccessStoreTxErrorMessagesSuite) fetchTxErrorMessages(txResults []*sdk.TransactionResult, containerName string) []*flow.TransactionResultErrorMessage {
	accessNode := s.net.ContainerByName(containerName)
	anDB, err := accessNode.DB()
	require.NoError(s.T(), err, "could not open db")

	metrics := metrics.NewNoopCollector()
	anTxErrorMessages := store.NewTransactionResultErrorMessages(metrics, anDB, store.DefaultCacheSize)

	txResultErrorMessages := make([]*flow.TransactionResultErrorMessage, len(txResults))
	for i, txResult := range txResults {
		txBlockID := convert.IDFromSDK(txResult.BlockID)
		txID := convert.IDFromSDK(txResult.TransactionID)

		errMsgResult, err := anTxErrorMessages.ByBlockIDTransactionID(txBlockID, txID)
		s.Require().NoError(err)

		txResultErrorMessages[i] = errMsgResult
	}

	return txResultErrorMessages
}

// verifyTxErrorMessage compares the expected and retrieved error messages to verify accuracy.
func (s *AccessStoreTxErrorMessagesSuite) verifyTxErrorMessage(txResults []*sdk.TransactionResult, errMsgResults []*flow.TransactionResultErrorMessage) {
	s.Require().Equal(len(txResults), len(errMsgResults))

	for i, txResult := range txResults {
		expectedTxResultErrorMessage := txResult.Error.Error()
		expectedTxID := convert.IDFromSDK(txResult.TransactionID)

		s.Require().Equal(expectedTxID, errMsgResults[i].TransactionID)
		s.Require().Equal(expectedTxResultErrorMessage, errMsgResults[i].ErrorMessage)
	}

}
