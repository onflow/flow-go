package cohort3

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

	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
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

	txBlockID := convert.IDFromSDK(txResult.BlockID)
	txID := convert.IDFromSDK(txResult.TransactionID)
	expectedTxResultErrorMessage := txResult.Error.Error()

	accessContainerName := "access_2"

	// Wait until execution receipts are handled, transaction error messages are stored.
	s.Eventually(func() bool {
		value, err := s.getMaxReceiptHeight(accessContainerName)
		return err == nil && value > txResult.BlockHeight
	}, 60*time.Second, 1*time.Second)

	// Stop the network containers before checking the results.
	s.net.StopContainers()

	// Get the access node and open the protocol DB.
	accessNode := s.net.ContainerByName(accessContainerName)
	// setup storage objects needed to get the execution data id
	anDB, err := accessNode.DB()
	require.NoError(s.T(), err, "could not open db")

	metrics := metrics.NewNoopCollector()
	anTxErrorMessages := badger.NewTransactionResultErrorMessages(metrics, anDB, badger.DefaultCacheSize)

	// Fetch the stored error message by block ID and transaction ID.
	errMsgResult, err := anTxErrorMessages.ByBlockIDTransactionID(txBlockID, txID)
	s.Require().NoError(err)

	// Verify that the error message retrieved matches the expected values.
	s.Require().Equal(txID, errMsgResult.TransactionID)
	s.Require().Equal(expectedTxResultErrorMessage, errMsgResult.ErrorMessage)
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
