package epochs

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/rs/zerolog"
	"strings"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
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
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.DebugLevel),
	}

	// a ghost node masquerading as a consensus node
	s.ghostID = unittest.IdentifierFixture()
	ghostConNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost())

	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
		ghostConNode,
	}

	netConf := testnet.NewNetworkConfigWithEpochConfig("epochs-tests", confs, 200, 50, 380)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConf)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.net.Start(ctx)

	// start tracking blocks
	s.Track(s.T(), ctx, s.Ghost())

	addr := fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort])
	client, err := testnet.NewClient(addr, s.net.Root().Header.ChainID.Chain())
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

// StakedNodeOperationInfo struct contains all the node information needed to start a node after it is onboarded (staked and registered)
type StakedNodeOperationInfo struct {
	NodeID                  flow.Identifier
	Role                    flow.Role
	StakingAccountAddress   sdk.Address
	FullAccountKey          *sdk.AccountKey
	StakingAccountKey       sdkcrypto.PrivateKey
	NetworkingKey           sdkcrypto.PrivateKey
	StakingKey              sdkcrypto.PrivateKey
	MachineAccountAddress   flow.Address
	MachineAccountKey       sdkcrypto.PrivateKey
	MachineAccountPublicKey flow.AccountPublicKey
	ContainerName           string
}

// StakeNode will generate initial keys needed for a SN/LN node and onboard this node using the following steps;
// 1. Generate keys (networking, staking, machine)
// 2. Create a new account, this will be the staking account
// 3. Transfer token amount for the given role to the staking account
// 4. Add additional funds to staking account for storage
// 5. Create Staking collection for node
// 6. Register node using staking collection object
func (s *Suite) StakeNode(ctx context.Context, env templates.Environment, role flow.Role) *StakedNodeOperationInfo {
	stakingAccountKey, networkingKey, stakingKey, machineAccountKey, machineAccountPubKey := s.generateAccountKeys(role)
	nodeID := flow.MakeID(stakingKey.PublicKey().Encode())
	fullAccountKey := sdk.NewAccountKey().
		SetPublicKey(stakingAccountKey.PublicKey()).
		SetHashAlgo(sdkcrypto.SHA2_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	// create staking account
	stakingAccountAddress, err := s.createAccount(
		ctx,
		fullAccountKey,
		s.client.Account(),
		s.client.SDKServiceAddress(),
	)
	require.NoError(s.T(), err)

	_, stakeAmount, err := s.client.TokenAmountByRole(role)
	require.NoError(s.T(), err)

	// fund account with token amount to stake
	result, err := s.fundAccount(ctx, stakingAccountAddress, fmt.Sprintf("%f", stakeAmount+10.0))
	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)

	stakingAccount, err := s.client.GetAccount(stakingAccountAddress)
	require.NoError(s.T(), err)

	// create staking collection
	result, err = s.createStakingCollection(ctx, env, stakingAccountKey, stakingAccount)
	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)

	// if node has a machine account key encode it
	var encMachinePubKey []byte
	if machineAccountKey != nil {
		encMachinePubKey, err = flow.EncodeRuntimeAccountPublicKey(machineAccountPubKey)
		require.NoError(s.T(), err)
	}

	containerName := fmt.Sprintf("%s_test", role)

	// register node using staking collection
	result, err = s.registerNode(
		ctx,
		env,
		stakingAccountKey,
		stakingAccount,
		nodeID,
		role,
		testnet.GetPrivateNodeInfoAddress(containerName),
		strings.TrimPrefix(networkingKey.PublicKey().String(), "0x"),
		strings.TrimPrefix(stakingKey.PublicKey().String(), "0x"),
		fmt.Sprintf("%f", stakeAmount),
		hex.EncodeToString(encMachinePubKey),
	)

	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)

	result = s.SetApprovedNodesScript(ctx, env, append(s.net.Identities().NodeIDs(), nodeID)...)
	require.NoError(s.T(), result.Error)

	return &StakedNodeOperationInfo{
		NodeID:                  nodeID,
		Role:                    role,
		StakingAccountAddress:   stakingAccountAddress,
		FullAccountKey:          fullAccountKey,
		StakingAccountKey:       stakingAccountKey,
		StakingKey:              stakingKey,
		NetworkingKey:           networkingKey,
		MachineAccountKey:       machineAccountKey,
		MachineAccountPublicKey: machineAccountPubKey,
		ContainerName:           containerName,
	}
}

// transfers tokens to receiver from service account
func (s *Suite) fundAccount(ctx context.Context, receiver sdk.Address, tokenAmount string) (*sdk.TransactionResult, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	env := utils.LocalnetEnv()
	transferTx, err := utils.MakeTransferTokenTx(
		env,
		receiver,
		s.client.Account(),
		0,
		tokenAmount,
		sdk.Identifier(latestBlockID),
	)
	require.NoError(s.T(), err)

	err = s.client.SignAndSendTransaction(ctx, transferTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, transferTx.ID())
	require.NoError(s.T(), err)

	return result, nil
}

// generates initial keys needed to bootstrap account
func (s *Suite) generateAccountKeys(role flow.Role) (
	operatorAccountKey,
	networkingKey,
	stakingKey,
	machineAccountKey crypto.PrivateKey,
	machineAccountPubKey flow.AccountPublicKey,
) {
	operatorAccountKey = unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLenECDSAP256)
	networkingKey = unittest.NetworkingPrivKeyFixture()
	stakingKey = unittest.StakingPrivKeyFixture()

	// create a machine account
	if role == flow.RoleConsensus || role == flow.RoleCollection {
		machineAccountKey = unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLenECDSAP256)

		machineAccountPubKey = flow.AccountPublicKey{
			PublicKey: machineAccountKey.PublicKey(),
			SignAlgo:  machineAccountKey.PublicKey().Algorithm(),
			HashAlgo:  bootstrap.DefaultMachineAccountHashAlgo,
			Weight:    1000,
		}
	}

	return
}

// creates a new flow account, can be used to test staking
func (s *Suite) createAccount(ctx context.Context,
	accountKey *sdk.AccountKey,
	payerAccount *sdk.Account,
	payer sdk.Address,
) (sdk.Address, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	addr, err := s.client.CreateAccount(ctx, accountKey, payerAccount, payer, sdk.Identifier(latestBlockID))
	require.NoError(s.T(), err)
	return addr, nil
}

// creates a staking collection for the given node
func (s *Suite) createStakingCollection(ctx context.Context, env templates.Environment, accountKey sdkcrypto.PrivateKey, stakingAccount *sdk.Account) (*sdk.TransactionResult, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	signer := sdkcrypto.NewInMemorySigner(accountKey, sdkcrypto.SHA2_256)

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

// submits register node transaction for staking collection
func (s *Suite) registerNode(
	ctx context.Context,
	env templates.Environment,
	accountKey sdkcrypto.PrivateKey,
	stakingAccount *sdk.Account,
	nodeID flow.Identifier,
	role flow.Role,
	networkingAddress string,
	networkingKey string,
	stakingKey string,
	amount string,
	machineKey string,
) (*sdk.TransactionResult, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	signer := sdkcrypto.NewInMemorySigner(accountKey, sdkcrypto.SHA2_256)

	registerNodeTx, err := utils.MakeCollectionRegisterNodeTx(
		env,
		stakingAccount,
		0,
		signer,
		s.client.SDKServiceAddress(),
		sdk.Identifier(latestBlockID),
		nodeID,
		role,
		networkingAddress,
		networkingKey,
		stakingKey,
		amount,
		machineKey,
	)
	require.NoError(s.T(), err)

	err = s.client.SignAndSendTransaction(ctx, registerNodeTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, registerNodeTx.ID())
	require.NoError(s.T(), err)
	return result, nil
}

func (s *Suite) ExecuteGetProposedTableScript(ctx context.Context, env templates.Environment, nodeID flow.Identifier) cadence.Value {
	v, err := s.client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	return v
}

// SetApprovedNodesScript adds a node the the approved node list, this must be done when a node joins the protocol during the epoch staking phase
func (s *Suite) SetApprovedNodesScript(ctx context.Context, env templates.Environment, identities ...flow.Identifier) *sdk.TransactionResult {
	ids := make([]cadence.Value, 0)
	for _, id := range identities {
		idCDC, err := cadence.NewString(id.String())
		require.NoError(s.T(), err)

		ids = append(ids, idCDC)
	}

	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	idTableAddress := sdk.HexToAddress(env.IDTableAddress)
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSetApprovedNodesScript(env)).
		SetGasLimit(9999).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(s.client.SDKServiceAddress(), 0, s.client.Account().Keys[0].SequenceNumber).
		SetPayer(s.client.SDKServiceAddress()).
		AddAuthorizer(idTableAddress)

	err = tx.AddArgument(cadence.NewArray(ids))
	require.NoError(s.T(), err)

	err = s.client.SignAndSendTransaction(ctx, tx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, tx.ID())
	require.NoError(s.T(), err)

	return result
}

// ExecuteReadApprovedNodesScript executes the return proposal table script and returns a list of approved nodes
func (s *Suite) ExecuteReadApprovedNodesScript(ctx context.Context, env templates.Environment) cadence.Value {
	v, err := s.client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)

	return v
}
