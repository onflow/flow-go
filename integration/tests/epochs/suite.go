package epochs

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/state/protocol/inmem"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const waitTimeout = 2 * time.Minute

// nodeUpdateValidation func that will be used to validate the health of the network
// after an identity table change during an epoch transition. This is used in
// tandem with runTestEpochJoinAndLeave.
//
// NOTE: rootSnapshot must be the snapshot that the node (info) was bootstrapped with.
type nodeUpdateValidation func(ctx context.Context, env templates.Environment, rootSnapshot *inmem.Snapshot, info *StakedNodeOperationInfo)

// Suite encapsulates common functionality for epoch integration tests.
type Suite struct {
	suite.Suite
	log zerolog.Logger
	common.TestnetStateTracker
	ctx         context.Context
	cancel      context.CancelFunc
	net         *testnet.FlowNetwork
	nodeConfigs []testnet.NodeConfig
	ghostID     flow.Identifier
	client      *testnet.Client

	// Epoch config (lengths in views)
	StakingAuctionLen uint64
	DKGPhaseLen       uint64
	EpochLen          uint64
}

// SetupTest is run automatically by the testing framework before each test case.
func (s *Suite) SetupTest() {
	// ensure epoch lengths are set correctly
	require.Greater(s.T(), s.EpochLen, s.StakingAuctionLen+s.DKGPhaseLen*3)

	s.ctx, s.cancel = context.WithCancel(context.Background())
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "suite.go").
		Str("testcase", s.T().Name()).
		Logger()
	s.log = logger
	s.log.Info().Msgf("================> SetupTest")
	defer func() {
		s.log.Info().Msgf("================> Finish SetupTest")
	}()

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	// a ghost node masquerading as an access node
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost())

	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.WarnLevel)),
		ghostNode,
	}

	netConf := testnet.NewNetworkConfigWithEpochConfig("epochs-tests", confs, s.StakingAuctionLen, s.DKGPhaseLen, s.EpochLen)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConf)

	// start the network

	s.net.Start(s.ctx)

	// start tracking blocks
	s.Track(s.T(), s.ctx, s.Ghost())

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
	s.log.Info().Msgf("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}

// StakedNodeOperationInfo struct contains all the node information needed to
// start a node after it is onboarded (staked and registered).
type StakedNodeOperationInfo struct {
	NodeID                  flow.Identifier
	Role                    flow.Role
	StakingAccountAddress   sdk.Address
	FullAccountKey          *sdk.AccountKey
	StakingAccountKey       sdkcrypto.PrivateKey
	NetworkingKey           sdkcrypto.PrivateKey
	StakingKey              sdkcrypto.PrivateKey
	MachineAccountAddress   sdk.Address
	MachineAccountKey       sdkcrypto.PrivateKey
	MachineAccountPublicKey flow.AccountPublicKey
	ContainerName           string
}

// StakeNode will generate initial keys needed for a SN/LN node and onboard this node using the following steps:
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

	containerName := s.getTestContainerName(role)

	// register node using staking collection
	result, machineAccountAddr, err := s.SubmitStakingCollectionRegisterNodeTx(
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

	result = s.SubmitSetApprovedListTx(ctx, env, append(s.net.Identities().NodeIDs(), nodeID)...)
	require.NoError(s.T(), result.Error)

	// ensure we are still in staking auction
	s.assertInPhase(ctx, flow.EpochPhaseStaking)

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
		MachineAccountAddress:   machineAccountAddr,
		ContainerName:           containerName,
	}
}

// WaitForPhase waits for epoch phase and will timeout after 2 minutes
func (s *Suite) WaitForPhase(ctx context.Context, phase flow.EpochPhase) {
	condition := func() bool {
		snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
		require.NoError(s.T(), err)

		currentPhase, err := snapshot.Phase()
		require.NoError(s.T(), err)

		return currentPhase == phase
	}
	require.Eventually(s.T(),
		condition,
		waitTimeout,
		100*time.Millisecond,
		fmt.Sprintf("did not reach epoch phase (%s) within %v seconds", phase, waitTimeout))
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
	s.client.Account().Keys[0].SequenceNumber++

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

	payerAccount.Keys[0].SequenceNumber++
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
	stakingAccount.Keys[0].SequenceNumber++

	return result, nil
}

// SubmitStakingCollectionRegisterNodeTx submits tx that calls StakingCollection.registerNode
func (s *Suite) SubmitStakingCollectionRegisterNodeTx(
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
) (*sdk.TransactionResult, sdk.Address, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	signer := sdkcrypto.NewInMemorySigner(accountKey, sdkcrypto.SHA2_256)

	registerNodeTx, err := utils.MakeStakingCollectionRegisterNodeTx(
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
	stakingAccount.Keys[0].SequenceNumber++

	if role == flow.RoleCollection || role == flow.RoleConsensus {
		var machineAccountAddr sdk.Address
		for _, event := range result.Events {
			if event.Type == sdk.EventAccountCreated { // assume only one account created (safe because we control the transaction)
				accountCreatedEvent := sdk.AccountCreatedEvent(event)
				machineAccountAddr = accountCreatedEvent.Address()
				break
			}
		}

		require.NotZerof(s.T(), machineAccountAddr, "failed to create the machine account: %s", machineAccountAddr)
		return result, machineAccountAddr, nil
	}

	return result, sdk.Address{}, nil
}

// SubmitStakingCollectionCloseStakeTx submits tx that calls StakingCollection.closeStake
func (s *Suite) SubmitStakingCollectionCloseStakeTx(
	ctx context.Context,
	env templates.Environment,
	accountKey sdkcrypto.PrivateKey,
	stakingAccount *sdk.Account,
	nodeID flow.Identifier,
) (*sdk.TransactionResult, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	signer := sdkcrypto.NewInMemorySigner(accountKey, sdkcrypto.SHA2_256)

	closeStakeTx, err := utils.MakeStakingCollectionCloseStakeTx(
		env,
		stakingAccount,
		0,
		signer,
		s.client.SDKServiceAddress(),
		sdk.Identifier(latestBlockID),
		nodeID,
	)
	require.NoError(s.T(), err)

	err = s.client.SignAndSendTransaction(ctx, closeStakeTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, closeStakeTx.ID())
	require.NoError(s.T(), err)
	stakingAccount.Keys[0].SequenceNumber++
	return result, nil
}

func (s *Suite) removeNodeFromProtocol(ctx context.Context, env templates.Environment, nodeID flow.Identifier) {
	result, err := s.submitAdminRemoveNodeTx(ctx, env, nodeID)
	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)

	// ensure we submit transaction while in staking phase
	s.assertInPhase(ctx, flow.EpochPhaseStaking)
}

// submitAdminRemoveNodeTx will submit the admin remove node transaction
func (s *Suite) submitAdminRemoveNodeTx(ctx context.Context,
	env templates.Environment,
	nodeID flow.Identifier,
) (*sdk.TransactionResult, error) {
	latestBlockID, err := s.client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	closeStakeTx, err := utils.MakeAdminRemoveNodeTx(
		env,
		s.client.Account(),
		0,
		sdk.Identifier(latestBlockID),
		nodeID,
	)
	require.NoError(s.T(), err)

	err = s.client.SignAndSendTransaction(ctx, closeStakeTx)
	require.NoError(s.T(), err)

	result, err := s.client.WaitForSealed(ctx, closeStakeTx.ID())
	require.NoError(s.T(), err)
	s.client.Account().Keys[0].SequenceNumber++
	return result, nil
}

func (s *Suite) ExecuteGetProposedTableScript(ctx context.Context, env templates.Environment, nodeID flow.Identifier) cadence.Value {
	v, err := s.client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	return v
}

// SubmitSetApprovedListTx adds a node the the approved node list, this must be done when a node joins the protocol during the epoch staking phase
func (s *Suite) SubmitSetApprovedListTx(ctx context.Context, env templates.Environment, identities ...flow.Identifier) *sdk.TransactionResult {
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
	s.client.Account().Keys[0].SequenceNumber++

	return result
}

// ExecuteReadApprovedNodesScript executes the return proposal table script and returns a list of approved nodes
func (s *Suite) ExecuteReadApprovedNodesScript(ctx context.Context, env templates.Environment) cadence.Value {
	v, err := s.client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)

	return v
}

// getTestContainerName returns a name for a test container in the form of ${role}_${nodeID}_test
func (s *Suite) getTestContainerName(role flow.Role) string {
	i := len(s.net.ContainersByRole(role)) + 1
	return fmt.Sprintf("%s_test_%d", role, i)
}

// assertNodeApprovedAndProposed executes the read approved nodes list and get proposed table scripts
// and checks that the info.NodeID is in both list
func (s *Suite) assertNodeApprovedAndProposed(ctx context.Context, env templates.Environment, info *StakedNodeOperationInfo) {
	// ensure node ID in approved list
	approvedNodes := s.ExecuteReadApprovedNodesScript(ctx, env)
	require.Containsf(s.T(), approvedNodes.(cadence.Array).Values, cadence.String(info.NodeID.String()), "expected new node to be in approved nodes list: %x", info.NodeID)

	// check if node is in proposed table
	proposedTable := s.ExecuteGetProposedTableScript(ctx, env, info.NodeID)
	require.Containsf(s.T(), proposedTable.(cadence.Array).Values, cadence.String(info.NodeID.String()), "expected new node to be in proposed table: %x", info.NodeID)
}

// assertNodeNotApprovedOrProposed executes the read approved nodes list and get proposed table scripts
// and checks that the info.NodeID is not included in either - this means the node would be excluded from future epochs
func (s *Suite) assertNodeNotApprovedOrProposed(ctx context.Context, env templates.Environment, nodeID flow.Identifier) {
	// ensure node ID not in approved list
	approvedNodes := s.ExecuteReadApprovedNodesScript(ctx, env)
	require.NotContainsf(s.T(), approvedNodes.(cadence.Array).Values, cadence.String(nodeID.String()), "expected new node to not be in approved nodes list: %x", nodeID)

	// check if node is not in proposed table
	proposedTable := s.ExecuteGetProposedTableScript(ctx, env, nodeID)
	require.NotContainsf(s.T(), proposedTable.(cadence.Array).Values, cadence.String(nodeID.String()), "expected new node to not be in proposed table: %x", nodeID)
}

// newTestContainerOnNetwork configures a new container on the suites network
func (s *Suite) newTestContainerOnNetwork(role flow.Role, info *StakedNodeOperationInfo) *testnet.Container {
	containerConfigs := []func(config *testnet.NodeConfig){
		testnet.WithLogLevel(zerolog.WarnLevel),
		testnet.WithID(info.NodeID),
	}

	nodeConfig := testnet.NewNodeConfig(role, containerConfigs...)
	testContainerConfig := testnet.NewContainerConfig(info.ContainerName, nodeConfig, info.NetworkingKey, info.StakingKey)
	err := testContainerConfig.WriteKeyFiles(s.net.BootstrapDir, info.MachineAccountAddress, encodable.MachineAccountPrivKey{PrivateKey: info.MachineAccountKey}, role)
	require.NoError(s.T(), err)

	//add our container to the network
	err = s.net.AddNode(s.T(), s.net.BootstrapDir, testContainerConfig)
	require.NoError(s.T(), err, "failed to add container to network")

	// if node is of LN/SN role type add additional flags to node container for secure GRPC connection
	if role == flow.RoleConsensus || role == flow.RoleCollection {
		// ghost containers don't participate in the network skip any SN/LN ghost containers
		nodeContainer := s.net.ContainerByID(testContainerConfig.NodeID)
		nodeContainer.AddFlag("insecure-access-api", "false")

		accessNodeIDS := make([]string, 0)
		for _, c := range s.net.ContainersByRole(flow.RoleAccess) {
			if c.Config.Role == flow.RoleAccess && !c.Config.Ghost {
				accessNodeIDS = append(accessNodeIDS, c.Config.NodeID.String())
			}
		}
		nodeContainer.AddFlag("access-node-ids", strings.Join(accessNodeIDS, ","))
	}

	return s.net.ContainerByID(info.NodeID)
}

// StakeNewNode will stake a new node, and create the corresponding docker container for that node
func (s *Suite) StakeNewNode(ctx context.Context, env templates.Environment, role flow.Role) (*StakedNodeOperationInfo, *testnet.Container) {
	// stake our new node
	info := s.StakeNode(ctx, env, role)

	// make sure our node is in the approved nodes list and the proposed nodes table
	s.assertNodeApprovedAndProposed(ctx, env, info)

	// add a new container to the network with the info used to stake our node
	testContainer := s.newTestContainerOnNetwork(role, info)

	return info, testContainer
}

// getContainerToReplace return a container from the network, make sure the container is not a ghost
func (s *Suite) getContainerToReplace(role flow.Role) *testnet.Container {
	nodes := s.net.ContainersByRole(role)
	require.True(s.T(), len(nodes) > 0)

	for _, c := range nodes {
		if !c.Config.Ghost {
			return c
		}
	}

	return nil
}

// assertInPhase checks if we are in the phase provided and returns the current view
func (s *Suite) assertInPhase(ctx context.Context, expectedPhase flow.EpochPhase) {
	snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	actualPhase, err := snapshot.Phase()
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedPhase, actualPhase)
}

// assertEpochCounter requires actual epoch counter is equal to counter provided
func (s *Suite) assertEpochCounter(ctx context.Context, expectedCounter uint64) {
	snapshot, err := s.client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	actualCounter, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.T(), err)
	require.Equalf(s.T(), expectedCounter, actualCounter, "expected to be in epoch %d got %d", expectedCounter, actualCounter)
}

// assertQCVotingSuccessful asserts that the QC has completed successfully
func (s *Suite) assertQCVotingSuccessful(ctx context.Context, env templates.Environment) {
	v, err := s.client.ExecuteScriptBytes(ctx, templates.GenerateGetVotingCompletedScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	require.Truef(s.T(), bool(v.(cadence.Bool)), "expected qc voting to have completed successfully")
}

// assertDKGSuccessful asserts that the DKG has completed successfully
func (s *Suite) assertDKGSuccessful(ctx context.Context, env templates.Environment) {
	v, err := s.client.ExecuteScriptBytes(ctx, templates.GenerateGetDKGCompletedScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	require.Truef(s.T(), bool(v.(cadence.Bool)), "expected dkg to have completed successfully")
}

// assertLatestFinalizedBlockHeightHigher will assert that the difference between snapshot height and latest finalized height
// is greater than numOfBlocks.
func (s *Suite) assertLatestFinalizedBlockHeightHigher(ctx context.Context, snapshot *inmem.Snapshot, numOfBlocks uint64) {
	bootstrapHead, err := snapshot.Head()
	require.NoError(s.T(), err)

	header, err := s.client.GetLatestSealedBlockHeader(ctx)
	require.NoError(s.T(), err)

	// head should now be at-least numOfBlocks blocks higher from when we started
	require.True(s.T(), header.Height-bootstrapHead.Height >= numOfBlocks, fmt.Sprintf("expected head.Height %d to be higher than head from the snapshot the node was bootstraped with bootstrapHead.Height %d.", header.Height, bootstrapHead.Height))
}

// submitSmokeTestTransaction will submit a create account transaction to smoke test network
// This ensures a single transaction can be sealed by the network.
func (s *Suite) submitSmokeTestTransaction(ctx context.Context) {
	fullAccountKey := sdk.NewAccountKey().
		SetPublicKey(unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLenECDSAP256).PublicKey()).
		SetHashAlgo(sdkcrypto.SHA2_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	// createAccount will submit a create account transaction and wait for it to be sealed
	_, err := s.createAccount(
		ctx,
		fullAccountKey,
		s.client.Account(),
		s.client.SDKServiceAddress(),
	)
	require.NoError(s.T(), err)
}

// assertNetworkHealthyAfterANChange after an access node is removed or added to the network
// this func can be used to perform sanity.
// 1. Check that there is no problem connecting directly to the AN provided and retrieve a protocol snapshot
// 2. Check that the chain moved atleast 20 blocks from when the node was bootstrapped by comparing
// head of the rootSnapshot with the head of the snapshot we retrieved directly from the AN
// 3. Check that we can execute a script on the AN
func (s *Suite) assertNetworkHealthyAfterANChange(ctx context.Context, env templates.Environment, rootSnapshot *inmem.Snapshot, info *StakedNodeOperationInfo) {

	// get snapshot directly from new AN and compare head with head from the
	// snapshot that was used to bootstrap the node
	clientAddr := fmt.Sprintf(":%s", s.net.AccessPortsByContainerName[info.ContainerName])
	client, err := testnet.NewClient(clientAddr, s.net.Root().Header.ChainID.Chain())
	require.NoError(s.T(), err)

	// overwrite client to point to the new AN (since we have stopped the initial AN at this point)
	s.client = client
	// assert atleast 20 blocks have been finalized since the node replacement
	s.assertLatestFinalizedBlockHeightHigher(ctx, rootSnapshot, 20)

	// execute script directly on new AN to ensure it's functional
	proposedTable, err := client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	require.Contains(s.T(), proposedTable.(cadence.Array).Values, cadence.String(info.NodeID.String()), "expected node ID to be present in proposed table returned by new AN.")
}

// assertNetworkHealthyAfterVNChange after an verification node is removed or added to the network
// this func can be used to perform sanity.
// 1. Ensure sealing continues by comparing latest sealed block from the root snapshot to the current latest sealed block
func (s *Suite) assertNetworkHealthyAfterVNChange(ctx context.Context, _ templates.Environment, rootSnapshot *inmem.Snapshot, _ *StakedNodeOperationInfo) {
	// assert at least 20 blocks have been finalized since the node replacement
	s.assertLatestFinalizedBlockHeightHigher(ctx, rootSnapshot, 20)
}

// assertNetworkHealthyAfterLNChange after an collection node is removed or added to the network
// this func can be used to perform sanity.
// 1. Submit transaction to network that will target the newly staked LN by making sure the reference block ID
// is after the first epoch.
func (s *Suite) assertNetworkHealthyAfterLNChange(ctx context.Context, _ templates.Environment, _ *inmem.Snapshot, _ *StakedNodeOperationInfo) {
	// At this point we have reached epoch 1 and our new LN node should be the only LN node in the network.
	// To validate the LN joined the network successfully and is processing transactions we submit a
	// create account transaction and assert there are no errors.
	s.submitSmokeTestTransaction(ctx)
}

// assertNetworkHealthyAfterSNChange after replacing a consensus node in the test and waiting until
// the epoch transition we should observe blocks finalizing and we should be able to submit a transaction
// that will indicate overall network health
// 1. Submit transaction to network
func (s *Suite) assertNetworkHealthyAfterSNChange(ctx context.Context, _ templates.Environment, _ *inmem.Snapshot, _ *StakedNodeOperationInfo) {
	// At this point we can assure that our SN node is participating in finalization and sealing because
	// there are only 2 SN nodes in the network now we will submit a transaction to the
	// network to ensure the network is overall healthy.
	s.submitSmokeTestTransaction(ctx)
}
