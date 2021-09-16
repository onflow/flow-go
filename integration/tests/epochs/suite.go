package epochs

import (
	"context"
	"fmt"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/integration/utils"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	common.TestnetStateTracker
	cancel      context.CancelFunc
	net         *testnet.FlowNetwork
	nodeConfigs []testnet.NodeConfig
	ghostID     flow.Identifier
	client      *testnet.Client
}

func (s *Suite) SetupTest() {

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.InfoLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.InfoLevel),
	}

	// a ghost node masquerading as a consensus node
	s.ghostID = unittest.IdentifierFixture()
	ghostConNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.DebugLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost())

	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithDebugImage(false)),
		testnet.NewNodeConfig(flow.RoleAccess),
		ghostConNode,
	}

	netConf := testnet.NewNetworkConfig("epochs tests", confs)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConf)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())

	client, err := testnet.NewClient(
		fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]),
		s.net.Root().Header.ChainID.Chain())
	require.NoError(s.T(), err)

	s.client = client
}

func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *Suite) TearDownTest() {
	s.net.Remove()
	if s.cancel != nil {
		s.cancel()
	}
}

//@TODO add util func to stake a node during integration test
func (s *Suite) StakeNode(role flow.Role) {
	networkingKey, _ := s.generateAccountKeys()

	fullAccountKey := sdk.NewAccountKey().
		SetPublicKey(networkingKey.PublicKey()).
		SetHashAlgo(sdkcrypto.SHA2_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	stakingAccountAddress, err := s.createNewLeaseAccount(fullAccountKey)
	require.NoError(s.T(), err)

	result, err := s.transferLeaseTokens(role, stakingAccountAddress)
	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)

	stakingAccount, err := s.client.GetAccount(stakingAccountAddress)
	require.NoError(s.T(), err)

	_, err = s.createStakingCollection(networkingKey, stakingAccount)
	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)
}

func (s *Suite) generateAccountKeys() (sdkcrypto.PrivateKey, sdkcrypto.PrivateKey) {
	//@TODO generate staking account key
	stakingAccountKey, err := unittest.NetworkingKey()

	//@TODO generate key for machine account
	stakingAccountKey, err := unittest.StakingKey()

	networkingKey, err := unittest.NetworkingKey()
	require.NoError(s.T(), err)

	stakingKey, err := unittest.StakingKey()
	require.NoError(s.T(), err)

	return networkingKey, stakingKey
}

// creates a new lease account, can be used to test staking
func (s *Suite) createNewLeaseAccount(fullAccountKey *sdk.AccountKey) (sdk.Address, error) {
	ctx := context.Background()

	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	makeLeaseAcctTx := utils.MakeCreateLocalnetLeaseAccountWithKey(
		fullAccountKey,
		s.client.Account(),
		0,
		sdk.Identifier(latestBlockID),
	)

	err = s.client.SignAndSendTransaction(ctx, makeLeaseAcctTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, makeLeaseAcctTx.ID())
	require.NoError(s.T(), err)

	stakingAccountAddress, found := s.client.UserAddress(result)
	if !found {
		return sdk.Address{}, fmt.Errorf("failed to stake node, could not create locked token account")
	}

	return stakingAccountAddress, nil
}

// transfers tokens to a lease account from the localnet service account
func (s *Suite) transferLeaseTokens(role flow.Role, to sdk.Address) (*sdk.TransactionResult, error) {
	ctx := context.Background()
	tokenAmount, err := s.client.TokenAmountByRole(role.String())
	require.NoError(s.T(), err)
	fmt.Println(tokenAmount)

	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	transferLeaseTokenTx, err := utils.MakeTransferLeaseToken(
		to,
		s.client.Account(),
		0,
		tokenAmount,
		sdk.Identifier(latestBlockID),
	)

	err = s.client.SignAndSendTransaction(ctx, transferLeaseTokenTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, transferLeaseTokenTx.ID())
	require.NoError(s.T(), err)

	return result, nil
}

// creates a staking collection for the given node
func (s *Suite) createStakingCollection(accountKey sdkcrypto.PrivateKey, stakingAccount *sdk.Account) (*sdk.TransactionResult, error) {
	ctx := context.Background()
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	env := utils.EnvFromNetwork("localnet")

	signer := sdkcrypto.NewInMemorySigner(accountKey, sdkcrypto.SHA2_256)
	require.NoError(s.T(), err)

	createStakingCollectionTx, err := utils.MakeCreateStakingCollectionTx(
		env,
		stakingAccount,
		0,
		signer,
		s.client.SDKServiceAddress(),
		sdk.Identifier(latestBlockID),
	)

	err = s.client.SignAndSendTransaction(ctx, createStakingCollectionTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, createStakingCollectionTx.ID())
	require.NoError(s.T(), err)

	return result, nil
}
