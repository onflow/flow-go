package access

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccessCircuitBreaker(t *testing.T) {
	suite.Run(t, new(AccessCircuitBreakerSuite))
}

type AccessCircuitBreakerSuite struct {
	suite.Suite

	log zerolog.Logger

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

var requestTimeout = 3 * time.Second
var cbRestoreTimeout = 6 * time.Second

func (s *AccessCircuitBreakerSuite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

func (s *AccessCircuitBreakerSuite) SetupTest() {
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	// need one access node with enabled circuit breaker
	nodeConfigs := []testnet.NodeConfig{
		testnet.NewNodeConfig(
			flow.RoleAccess,
			testnet.WithLogLevel(zerolog.InfoLevel),
			testnet.WithAdditionalFlag("--circuit-breaker-enabled=true"),
			testnet.WithAdditionalFlag(fmt.Sprintf("--circuit-breaker-restore-timeout=%s", cbRestoreTimeout.String())),
			testnet.WithAdditionalFlag("--circuit-breaker-max-failures=1"),
			testnet.WithAdditionalFlag(fmt.Sprintf("--collection-client-timeout=%s", requestTimeout.String())),
		),
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

// TestCircuitBreaker tests the behavior of the circuit breaker. It verifies the circuit breaker's ability to open,
// prevent further requests, and restore after a timeout. It is done in a few steps:
// 1. Get the collection node and disconnect it from the network.
// 2. Try to send a transaction multiple times to observe the decrease in waiting time for a failed response.
// 3. Connect the collection node to the network and wait for the circuit breaker restore time.
// 4. Successfully send a transaction.
func (s *AccessCircuitBreakerSuite) TestCircuitBreaker() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// 1. Get the collection node
	collectionContainer := s.net.ContainerByName("collection_1")

	// 2. Get the Access Node container and client
	accessContainer := s.net.ContainerByName(testnet.PrimaryAN)

	// Check if access node was created with circuit breaker flags
	require.True(s.T(), accessContainer.IsFlagSet("circuit-breaker-enabled"))
	require.True(s.T(), accessContainer.IsFlagSet("circuit-breaker-restore-timeout"))
	require.True(s.T(), accessContainer.IsFlagSet("circuit-breaker-max-failures"))

	accessClient, err := accessContainer.TestnetClient()
	assert.NoError(s.T(), err, "failed to get access node client")

	latestBlockID, err := accessClient.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	// Create a new account to deploy Counter to
	accountPrivateKey := lib.RandomPrivateKey()

	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(accessClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx, err := templates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]templates.Contract{
			{
				Name:   lib.CounterContract.Name,
				Source: lib.CounterContract.ToCadence(),
			},
		}, serviceAddress)
	require.NoError(s.T(), err)

	createAccountTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, accessClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		SetGasLimit(9999)

	// Sign the transaction
	childCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	signedTx, err := accessClient.SignTransaction(createAccountTx)
	require.NoError(s.T(), err)
	cancel()

	// 3. Disconnect the collection node from the network to activate the Circuit Breaker
	err = collectionContainer.Disconnect()
	require.NoError(s.T(), err, "failed to pause connection node")

	// 4. Send a couple of transactions to test if the circuit breaker opens correctly
	sendTransaction := func(ctx context.Context, tx *sdk.Transaction) (time.Duration, error) {
		childCtx, cancel = context.WithTimeout(ctx, time.Second*10)
		start := time.Now()
		err := accessClient.SendTransaction(childCtx, tx)
		duration := time.Since(start)
		defer cancel()

		return duration, err
	}

	// Try to send the transaction for the first time. It should wait at least the timeout time and return Unavailable error
	duration, err := sendTransaction(ctx, signedTx)
	assert.Equal(s.T(), codes.Unavailable, status.Code(err))
	assert.GreaterOrEqual(s.T(), duration, requestTimeout)

	// Try to send the transaction for the second time. It should wait less than a second because the circuit breaker
	// is configured to break after the first failure
	duration, err = sendTransaction(ctx, signedTx)
	assert.Equal(s.T(), codes.Unknown, status.Code(err))
	assert.Greater(s.T(), time.Second, duration)

	// Reconnect the collection node
	err = collectionContainer.Connect()
	require.NoError(s.T(), err, "failed to start collection node")

	// Wait for the circuit breaker to restore
	time.Sleep(cbRestoreTimeout)

	// Try to send the transaction for the third time. The transaction should be sent successfully
	_, err = sendTransaction(ctx, signedTx)
	require.NoError(s.T(), err, "transaction should be sent")
}
