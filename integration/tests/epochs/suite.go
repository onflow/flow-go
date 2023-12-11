// Package epochs contains common functionality for the epoch integration test suite.
// Individual tests exist in sub-directories of this: an, ln, static, sn, vn...
// Each cohort is run as a separate, sequential CI job. Since the epoch tests are long
// and resource-heavy, we split them into several cohorts, which can be run in parallel.
//
// If a new cohort is added in the future, it must be added to:
//   - ci.yml, flaky-test-monitor.yml, bors.toml (ensure new cohort of tests is run)
//   - Makefile (include new cohort in integration-test directive, etc.)
package epochs

import (
	"context"
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

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// nodeUpdateValidation func that will be used to validate the health of the network
// after an identity table change during an epoch transition. This is used in
// tandem with RunTestEpochJoinAndLeave.
// NOTE: The snapshot must reference a block within the second epoch.
type nodeUpdateValidation func(ctx context.Context, env templates.Environment, snapshot *inmem.Snapshot, info *StakedNodeOperationInfo)

// Suite encapsulates common functionality for epoch integration tests.
type Suite struct {
	suite.Suite
	lib.TestnetStateTracker
	cancel  context.CancelFunc
	log     zerolog.Logger
	net     *testnet.FlowNetwork
	ghostID flow.Identifier

	Client *testnet.Client
	Ctx    context.Context

	// Epoch config (lengths in views)
	StakingAuctionLen          uint64
	DKGPhaseLen                uint64
	EpochLen                   uint64
	EpochCommitSafetyThreshold uint64
	// Whether approvals are required for sealing (we only enable for VN tests because
	// requiring approvals requires a longer DKG period to avoid flakiness)
	RequiredSealApprovals uint // defaults to 0 (no approvals required)
	// Consensus Node proposal duration
	ConsensusProposalDuration time.Duration
}

// SetupTest is run automatically by the testing framework before each test case.
func (s *Suite) SetupTest() {
	// If unset, use default value 100ms
	if s.ConsensusProposalDuration == 0 {
		s.ConsensusProposalDuration = time.Millisecond * 100
	}

	minEpochLength := s.StakingAuctionLen + s.DKGPhaseLen*3 + 20
	// ensure epoch lengths are set correctly
	require.Greater(s.T(), s.EpochLen, minEpochLength+s.EpochCommitSafetyThreshold, "epoch too short")

	s.Ctx, s.cancel = context.WithCancel(context.Background())
	s.log = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.log.Info().Msg("================> SetupTest")
	defer func() {
		s.log.Info().Msg("================> Finish SetupTest")
	}()

	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-proposal-duration=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel)}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag(fmt.Sprintf("--cruise-ctl-fallback-proposal-duration=%s", s.ConsensusProposalDuration)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", s.RequiredSealApprovals)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", s.RequiredSealApprovals)),
		testnet.WithLogLevel(zerolog.WarnLevel)}

	// a ghost node masquerading as an access node
	s.ghostID = unittest.IdentifierFixture()
	ghostNode := testnet.NewNodeConfig(
		flow.RoleAccess,
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithID(s.ghostID),
		testnet.AsGhost())

	confs := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.WarnLevel)),
		ghostNode,
	}

	netConf := testnet.NewNetworkConfigWithEpochConfig("epochs-tests", confs, s.StakingAuctionLen, s.DKGPhaseLen, s.EpochLen, s.EpochCommitSafetyThreshold)

	// initialize the network
	s.net = testnet.PrepareFlowNetwork(s.T(), netConf, flow.Localnet)

	// start the network
	s.net.Start(s.Ctx)

	// start tracking blocks
	s.Track(s.T(), s.Ctx, s.Ghost())

	// use AN1 for test-related queries - the AN join/leave test will replace AN2
	client, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	require.NoError(s.T(), err)

	s.Client = client

	// log network info periodically to aid in debugging future flaky tests
	go lib.LogStatusPeriodically(s.T(), s.Ctx, s.log, s.Client, 5*time.Second)
}

func (s *Suite) Ghost() *client.GhostClient {
	client, err := s.net.ContainerByID(s.ghostID).GhostClient()
	require.NoError(s.T(), err, "could not get ghost Client")
	return client
}

// TimedLogf logs the message using t.Log and the suite logger, but prefixes the current time.
// This enables viewing logs inline with Docker logs as well as other test logs.
func (s *Suite) TimedLogf(msg string, args ...interface{}) {
	s.log.Info().Msgf(msg, args...)
	args = append([]interface{}{time.Now().String()}, args...)
	s.T().Logf("%s - "+msg, args...)
}

func (s *Suite) TearDownTest() {
	s.log.Info().Msg("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msg("================> Finish TearDownTest")
}

// StakedNodeOperationInfo struct contains all the node information needed to
// start a node after it is onboarded (staked and registered).
type StakedNodeOperationInfo struct {
	NodeID                flow.Identifier
	Role                  flow.Role
	StakingAccountAddress sdk.Address
	FullAccountKey        *sdk.AccountKey
	StakingAccountKey     sdkcrypto.PrivateKey
	NetworkingKey         sdkcrypto.PrivateKey
	StakingKey            sdkcrypto.PrivateKey
	// machine account info defined only for consensus/collection nodes
	MachineAccountAddress   sdk.Address
	MachineAccountKey       sdkcrypto.PrivateKey
	MachineAccountPublicKey *sdk.AccountKey
	ContainerName           string
}

// StakeNode will generate initial keys needed for a SN/LN node and onboard this node using the following steps:
// 1. Generate keys (networking, staking, machine)
// 2. Create a new account, this will be the staking account
// 3. Transfer token amount for the given role to the staking account
// 4. Add additional funds to staking account for storage
// 5. Create Staking collection for node
// 6. Register node using staking collection object
// 7. Add the node to the approved list
//
// NOTE: assumes staking occurs in first epoch (counter 0)
// NOTE 2: This function performs steps 1-6 in one custom transaction, to reduce
// the time taken by each test case. Individual transactions for each step can be
// found in Git history, for example: 9867056a8b7246655047bc457f9000398f6687c0.
func (s *Suite) StakeNode(ctx context.Context, env templates.Environment, role flow.Role) *StakedNodeOperationInfo {

	stakingAccountKey, networkingKey, stakingKey, machineAccountKey, machineAccountPubKey := s.generateAccountKeys(role)
	nodeID := flow.MakeID(stakingKey.PublicKey().Encode())
	fullStakingAcctKey := sdk.NewAccountKey().
		SetPublicKey(stakingAccountKey.PublicKey()).
		SetHashAlgo(sdkcrypto.SHA2_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	_, stakeAmount, err := s.Client.TokenAmountByRole(role)
	require.NoError(s.T(), err)
	containerName := s.getTestContainerName(role)

	latestBlockID, err := s.Client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	// create and register node
	tx, err := utils.MakeCreateAndSetupNodeTx(
		env,
		s.Client.Account(),
		sdk.Identifier(latestBlockID),
		fullStakingAcctKey,
		fmt.Sprintf("%f", stakeAmount+10.0),
		nodeID,
		role,
		testnet.GetPrivateNodeInfoAddress(containerName),
		strings.TrimPrefix(networkingKey.PublicKey().String(), "0x"),
		strings.TrimPrefix(stakingKey.PublicKey().String(), "0x"),
		machineAccountPubKey,
	)
	require.NoError(s.T(), err)

	err = s.Client.SignAndSendTransaction(ctx, tx)
	require.NoError(s.T(), err)
	result, err := s.Client.WaitForSealed(ctx, tx.ID())
	require.NoError(s.T(), err)
	s.Client.Account().Keys[0].SequenceNumber++
	require.NoError(s.T(), result.Error)

	accounts := s.Client.CreatedAccounts(result)
	stakingAccountAddress := accounts[0]
	var machineAccountAddr sdk.Address
	if role == flow.RoleCollection || role == flow.RoleConsensus {
		machineAccountAddr = accounts[1]
	}

	result = s.SubmitSetApprovedListTx(ctx, env, append(s.net.Identities().NodeIDs(), nodeID)...)
	require.NoError(s.T(), result.Error)

	// ensure we are still in staking auction
	s.AssertInEpochPhase(ctx, 0, flow.EpochPhaseStaking)

	return &StakedNodeOperationInfo{
		NodeID:                  nodeID,
		Role:                    role,
		StakingAccountAddress:   stakingAccountAddress,
		FullAccountKey:          fullStakingAcctKey,
		StakingAccountKey:       stakingAccountKey,
		StakingKey:              stakingKey,
		NetworkingKey:           networkingKey,
		MachineAccountKey:       machineAccountKey,
		MachineAccountPublicKey: machineAccountPubKey,
		MachineAccountAddress:   machineAccountAddr,
		ContainerName:           containerName,
	}
}

// generates initial keys needed to bootstrap account
func (s *Suite) generateAccountKeys(role flow.Role) (
	operatorAccountKey,
	networkingKey,
	stakingKey,
	machineAccountKey crypto.PrivateKey,
	machineAccountPubKey *sdk.AccountKey,
) {
	operatorAccountKey = unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLen)
	networkingKey = unittest.NetworkingPrivKeyFixture()
	stakingKey = unittest.StakingPrivKeyFixture()

	// create a machine account
	if role == flow.RoleConsensus || role == flow.RoleCollection {
		machineAccountKey = unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLen)

		machineAccountPubKey = &sdk.AccountKey{
			PublicKey: machineAccountKey.PublicKey(),
			SigAlgo:   machineAccountKey.PublicKey().Algorithm(),
			HashAlgo:  bootstrap.DefaultMachineAccountHashAlgo,
			Weight:    1000,
		}
	}

	return
}

// removeNodeFromProtocol removes the given node from the protocol.
// NOTE: assumes staking occurs in first epoch (counter 0)
func (s *Suite) removeNodeFromProtocol(ctx context.Context, env templates.Environment, nodeID flow.Identifier) {
	result, err := s.submitAdminRemoveNodeTx(ctx, env, nodeID)
	require.NoError(s.T(), err)
	require.NoError(s.T(), result.Error)

	// ensure we submit transaction while in staking phase
	s.AssertInEpochPhase(ctx, 0, flow.EpochPhaseStaking)
}

// submitAdminRemoveNodeTx will submit the admin remove node transaction
func (s *Suite) submitAdminRemoveNodeTx(ctx context.Context,
	env templates.Environment,
	nodeID flow.Identifier,
) (*sdk.TransactionResult, error) {
	latestBlockID, err := s.Client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	closeStakeTx, err := utils.MakeAdminRemoveNodeTx(
		env,
		s.Client.Account(),
		0,
		sdk.Identifier(latestBlockID),
		nodeID,
	)
	require.NoError(s.T(), err)

	err = s.Client.SignAndSendTransaction(ctx, closeStakeTx)
	require.NoError(s.T(), err)

	result, err := s.Client.WaitForSealed(ctx, closeStakeTx.ID())
	require.NoError(s.T(), err)
	s.Client.Account().Keys[0].SequenceNumber++
	return result, nil
}

func (s *Suite) ExecuteGetProposedTableScript(ctx context.Context, env templates.Environment, nodeID flow.Identifier) cadence.Value {
	v, err := s.Client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	return v
}

// ExecuteGetNodeInfoScript executes a script to get staking info about the given node.
func (s *Suite) ExecuteGetNodeInfoScript(ctx context.Context, env templates.Environment, nodeID flow.Identifier) cadence.Value {
	cdcNodeID, err := cadence.NewString(nodeID.String())
	require.NoError(s.T(), err)
	v, err := s.Client.ExecuteScriptBytes(ctx, templates.GenerateGetNodeInfoScript(env), []cadence.Value{cdcNodeID})
	require.NoError(s.T(), err)
	return v
}

// SubmitSetApprovedListTx adds a node to the approved node list, this must be done when a node joins the protocol during the epoch staking phase
func (s *Suite) SubmitSetApprovedListTx(ctx context.Context, env templates.Environment, identities ...flow.Identifier) *sdk.TransactionResult {
	latestBlockID, err := s.Client.GetLatestBlockID(ctx)
	require.NoError(s.T(), err)

	idTableAddress := sdk.HexToAddress(env.IDTableAddress)
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSetApprovedNodesScript(env)).
		SetGasLimit(9999).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(s.Client.SDKServiceAddress(), 0, s.Client.Account().Keys[0].SequenceNumber).
		SetPayer(s.Client.SDKServiceAddress()).
		AddAuthorizer(idTableAddress)
	err = tx.AddArgument(blueprints.SetStakingAllowlistTxArg(identities))
	require.NoError(s.T(), err)

	err = s.Client.SignAndSendTransaction(ctx, tx)
	require.NoError(s.T(), err)

	result, err := s.Client.WaitForSealed(ctx, tx.ID())
	require.NoError(s.T(), err)
	s.Client.Account().Keys[0].SequenceNumber++

	return result
}

// ExecuteReadApprovedNodesScript executes the return proposal table script and returns a list of approved nodes
func (s *Suite) ExecuteReadApprovedNodesScript(ctx context.Context, env templates.Environment) cadence.Value {
	v, err := s.Client.ExecuteScriptBytes(ctx, templates.GenerateGetApprovedNodesScript(env), []cadence.Value{})
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
	//approvedNodes := s.ExecuteReadApprovedNodesScript(Ctx, env)
	//require.Containsf(s.T(), approvedNodes.(cadence.Array).Values, cadence.String(info.NodeID.String()), "expected new node to be in approved nodes list: %x", info.NodeID)

	// Access Nodes go through a separate selection process, so they do not immediately
	// appear on the proposed table -- skip checking for them here.
	if info.Role == flow.RoleAccess {
		s.T().Logf("skipping checking proposed table for joining Access Node")
		return
	}

	// check if node is in proposed table
	proposedTable := s.ExecuteGetProposedTableScript(ctx, env, info.NodeID)
	require.Containsf(s.T(), proposedTable.(cadence.Array).Values, cadence.String(info.NodeID.String()), "expected new node to be in proposed table: %x", info.NodeID)
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

// AwaitEpochPhase waits for the given phase, in the given epoch.
func (s *Suite) AwaitEpochPhase(ctx context.Context, expectedEpoch uint64, expectedPhase flow.EpochPhase, waitFor, tick time.Duration) {
	var actualEpoch uint64
	var actualPhase flow.EpochPhase
	condition := func() bool {
		snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
		require.NoError(s.T(), err)

		actualEpoch, err = snapshot.Epochs().Current().Counter()
		require.NoError(s.T(), err)
		actualPhase, err = snapshot.Phase()
		require.NoError(s.T(), err)

		return actualEpoch == expectedEpoch && actualPhase == expectedPhase
	}
	require.Eventuallyf(s.T(), condition, waitFor, tick, "did not reach expectedEpoch %d phase %s within %s. Last saw epoch=%d and phase=%s", expectedEpoch, expectedPhase, waitFor, actualEpoch, actualPhase)
}

// AssertInEpochPhase checks if we are in the phase of the given epoch.
func (s *Suite) AssertInEpochPhase(ctx context.Context, expectedEpoch uint64, expectedPhase flow.EpochPhase) {
	snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	actualEpoch, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.T(), err)
	actualPhase, err := snapshot.Phase()
	require.NoError(s.T(), err)
	require.Equal(s.T(), expectedPhase, actualPhase, "not in correct phase")
	require.Equal(s.T(), expectedEpoch, actualEpoch, "not in correct epoch")

	head, err := snapshot.Head()
	require.NoError(s.T(), err)
	s.TimedLogf("asserted in epoch %d, phase %s, finalized height/view: %d/%d", expectedEpoch, expectedPhase, head.Height, head.View)
}

// AssertInEpoch requires actual epoch counter is equal to counter provided.
func (s *Suite) AssertInEpoch(ctx context.Context, expectedEpoch uint64) {
	snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	actualEpoch, err := snapshot.Epochs().Current().Counter()
	require.NoError(s.T(), err)
	require.Equalf(s.T(), expectedEpoch, actualEpoch, "expected to be in epoch %d got %d", expectedEpoch, actualEpoch)
}

// AssertNodeNotParticipantInEpoch asserts that the given node ID does not exist
// in the epoch's identity table.
func (s *Suite) AssertNodeNotParticipantInEpoch(epoch protocol.Epoch, nodeID flow.Identifier) {
	identities, err := epoch.InitialIdentities()
	require.NoError(s.T(), err)
	require.NotContains(s.T(), identities.NodeIDs(), nodeID)
}

// AwaitSealedBlockHeightExceedsSnapshot polls until it observes that the latest
// sealed block height has exceeded the snapshot height by numOfBlocks
// the snapshot height and latest finalized height is greater than numOfBlocks.
func (s *Suite) AwaitSealedBlockHeightExceedsSnapshot(ctx context.Context, snapshot *inmem.Snapshot, threshold uint64, waitFor, tick time.Duration) {
	header, err := snapshot.Head()
	require.NoError(s.T(), err)
	snapshotHeight := header.Height

	require.Eventually(s.T(), func() bool {
		latestSealed := s.getLatestSealedHeader(ctx)
		s.TimedLogf("waiting for sealed block height: %d+%d < %d", snapshotHeight, threshold, latestSealed.Height)
		return snapshotHeight+threshold < latestSealed.Height
	}, waitFor, tick)
}

// AwaitFinalizedView polls until it observes that the latest finalized block has a view
// greater than or equal to the input view. This is used to wait until when an epoch
// transition must have happened.
func (s *Suite) AwaitFinalizedView(ctx context.Context, view uint64, waitFor, tick time.Duration) {
	require.Eventually(s.T(), func() bool {
		sealed := s.getLatestFinalizedHeader(ctx)
		return sealed.View >= view
	}, waitFor, tick)
}

// getLatestSealedHeader retrieves the latest sealed block, as reported in LatestSnapshot.
func (s *Suite) getLatestSealedHeader(ctx context.Context) *flow.Header {
	snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	segment, err := snapshot.SealingSegment()
	require.NoError(s.T(), err)
	sealed := segment.Sealed()
	return sealed.Header
}

// getLatestFinalizedHeader retrieves the latest finalized block, as reported in LatestSnapshot.
func (s *Suite) getLatestFinalizedHeader(ctx context.Context) *flow.Header {
	snapshot, err := s.Client.GetLatestProtocolSnapshot(ctx)
	require.NoError(s.T(), err)
	finalized, err := snapshot.Head()
	require.NoError(s.T(), err)
	return finalized
}

// SubmitSmokeTestTransaction will submit a create account transaction to smoke test network
// This ensures a single transaction can be sealed by the network.
func (s *Suite) SubmitSmokeTestTransaction(ctx context.Context) {
	_, err := utils.CreateFlowAccount(ctx, s.Client)
	require.NoError(s.T(), err)
}

// AssertNetworkHealthyAfterANChange performs a basic network health check after replacing an access node.
//  1. Check that there is no problem connecting directly to the AN provided and retrieve a protocol snapshot
//  2. Check that the chain moved at least 20 blocks from when the node was bootstrapped by comparing
//     head of the rootSnapshot with the head of the snapshot we retrieved directly from the AN
//  3. Check that we can execute a script on the AN
//
// TODO test sending and observing result of a transaction via the new AN (blocked by https://github.com/onflow/flow-go/issues/3642)
func (s *Suite) AssertNetworkHealthyAfterANChange(ctx context.Context, env templates.Environment, snapshotInSecondEpoch *inmem.Snapshot, info *StakedNodeOperationInfo) {

	// get snapshot directly from new AN and compare head with head from the
	// snapshot that was used to bootstrap the node
	client, err := s.net.ContainerByName(info.ContainerName).TestnetClient()
	require.NoError(s.T(), err)

	// overwrite Client to point to the new AN (since we have stopped the initial AN at this point)
	s.Client = client
	// assert atleast 20 blocks have been finalized since the node replacement
	s.AwaitSealedBlockHeightExceedsSnapshot(ctx, snapshotInSecondEpoch, 10, 30*time.Second, time.Millisecond*100)

	// execute script directly on new AN to ensure it's functional
	proposedTable, err := client.ExecuteScriptBytes(ctx, templates.GenerateReturnProposedTableScript(env), []cadence.Value{})
	require.NoError(s.T(), err)
	require.Contains(s.T(), proposedTable.(cadence.Array).Values, cadence.String(info.NodeID.String()), "expected node ID to be present in proposed table returned by new AN.")
}

// AssertNetworkHealthyAfterVNChange performs a basic network health check after replacing a verification node.
//  1. Ensure sealing continues into the second epoch (post-replacement) by observing
//     at least 10 blocks of sealing progress within the epoch
func (s *Suite) AssertNetworkHealthyAfterVNChange(ctx context.Context, _ templates.Environment, snapshotInSecondEpoch *inmem.Snapshot, _ *StakedNodeOperationInfo) {
	s.AwaitSealedBlockHeightExceedsSnapshot(ctx, snapshotInSecondEpoch, 10, 30*time.Second, time.Millisecond*100)
}

// AssertNetworkHealthyAfterLNChange performs a basic network health check after replacing a collection node.
//  1. Submit transaction to network that will target the newly staked LN by making
//     sure the reference block ID is after the first epoch.
func (s *Suite) AssertNetworkHealthyAfterLNChange(ctx context.Context, _ templates.Environment, _ *inmem.Snapshot, _ *StakedNodeOperationInfo) {
	// At this point we have reached the second epoch and our new LN is the only LN in the network.
	// To validate the LN joined the network successfully and is processing transactions we create
	// an account, which submits a transaction and verifies it is sealed.
	s.SubmitSmokeTestTransaction(ctx)
}

// AssertNetworkHealthyAfterSNChange performs a basic network health check after replacing a consensus node.
// The RunTestEpochJoinAndLeave function running prior to this health check already asserts that we successfully:
//  1. enter the second epoch (DKG succeeds; epoch fallback is not triggered)
//  2. seal at least the first block within the second epoch (consensus progresses into second epoch).
//
// The test is configured so that one offline committee member is enough to prevent progress,
// therefore the newly joined consensus node must be participating in consensus.
//
// In addition, here, we submit a transaction and verify that it is sealed.
func (s *Suite) AssertNetworkHealthyAfterSNChange(ctx context.Context, _ templates.Environment, _ *inmem.Snapshot, _ *StakedNodeOperationInfo) {
	s.SubmitSmokeTestTransaction(ctx)
}

// RunTestEpochJoinAndLeave coordinates adding and removing one node with the given
// role during the first epoch, then running the network health validation function
// once the network has successfully transitioned into the second epoch.
//
// This tests:
// * that nodes can stake and join the network at an epoch boundary
// * that nodes can unstake and leave the network at an epoch boundary
// * role-specific network health validation after the swap has completed
func (s *Suite) RunTestEpochJoinAndLeave(role flow.Role, checkNetworkHealth nodeUpdateValidation) {

	env := utils.LocalnetEnv()

	var containerToReplace *testnet.Container

	// replace access_2, avoid replacing access_1 the container used for Client connections
	if role == flow.RoleAccess {
		containerToReplace = s.net.ContainerByName("access_2")
		require.NotNil(s.T(), containerToReplace)
	} else {
		// grab the first container of this node role type, this is the container we will replace
		containerToReplace = s.getContainerToReplace(role)
		require.NotNil(s.T(), containerToReplace)
	}

	// staking our new node and add get the corresponding container for that node
	s.TimedLogf("staking joining node with role %s", role.String())
	info, testContainer := s.StakeNewNode(s.Ctx, env, role)
	s.TimedLogf("successfully staked joining node: %s", info.NodeID)

	// use admin transaction to remove node, this simulates a node leaving the network
	s.TimedLogf("removing node %s with role %s", containerToReplace.Config.NodeID, role.String())
	s.removeNodeFromProtocol(s.Ctx, env, containerToReplace.Config.NodeID)
	s.TimedLogf("successfully removed node: %s", containerToReplace.Config.NodeID)

	// wait for epoch setup phase before we start our container and pause the old container
	s.TimedLogf("waiting for EpochSetup phase of first epoch to begin")
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, time.Minute, 500*time.Millisecond)
	s.TimedLogf("successfully reached EpochSetup phase of first epoch")

	// get the latest snapshot and start new container with it
	rootSnapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)

	header, err := rootSnapshot.Head()
	require.NoError(s.T(), err)
	segment, err := rootSnapshot.SealingSegment()
	require.NoError(s.T(), err)

	s.TimedLogf("retrieved header after entering EpochSetup phase: root_height=%d, root_view=%d, segment_heights=[%d-%d], segment_views=[%d-%d]",
		header.Height, header.View,
		segment.Sealed().Header.Height, segment.Highest().Header.Height,
		segment.Sealed().Header.View, segment.Highest().Header.View)

	testContainer.WriteRootSnapshot(rootSnapshot)
	testContainer.Container.Start(s.Ctx)

	epoch1FinalView, err := rootSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// wait for at least the first block of the next epoch to be sealed before we pause our container to replace
	s.TimedLogf("waiting for epoch transition (finalized view %d) before pausing container", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 4*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d -> pausing container", epoch1FinalView+1)

	// make sure container to replace is not a member of epoch 2
	s.AssertNodeNotParticipantInEpoch(rootSnapshot.Epochs().Next(), containerToReplace.Config.NodeID)

	// assert transition to second epoch happened as expected
	// if counter is still 0, epoch emergency fallback was triggered and we can fail early
	s.AssertInEpoch(s.Ctx, 1)

	err = containerToReplace.Pause()
	require.NoError(s.T(), err)

	// retrieve a snapshot after observing that we have entered the second epoch
	secondEpochSnapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)

	// make sure the network is healthy after adding new node
	checkNetworkHealth(s.Ctx, env, secondEpochSnapshot, info)
}

// DynamicEpochTransitionSuite  is the suite used for epoch transitions tests
// with a dynamic identity table.
type DynamicEpochTransitionSuite struct {
	Suite
}

func (s *DynamicEpochTransitionSuite) SetupTest() {
	// use a longer staking auction length to accommodate staking operations for joining/leaving nodes
	// NOTE: this value is set fairly aggressively to ensure shorter test times.
	// If flakiness due to failure to complete staking operations in time is observed,
	// try increasing (by 10-20 views).
	s.StakingAuctionLen = 50
	s.DKGPhaseLen = 50
	s.EpochLen = 250
	s.EpochCommitSafetyThreshold = 20

	// run the generic setup, which starts up the network
	s.Suite.SetupTest()
}
