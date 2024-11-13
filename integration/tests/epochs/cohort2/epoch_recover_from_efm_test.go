package cohort2

import (
	"encoding/hex"
	"fmt"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/rs/zerolog"

	"strings"
	"testing"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

func TestRecoverEpoch(t *testing.T) {
	suite.Run(t, new(RecoverEpochSuite))
}

// Suite encapsulates common functionality for epoch integration tests.
type RecoverEpochSuite struct {
	epochs.BaseSuite
}

func (s *RecoverEpochSuite) SetupTest() {
	// use a shorter staking auction because we don't have staking operations in this case
	s.StakingAuctionLen = 2
	// to manually trigger EFM we assign very short dkg phase len ensuring the dkg will fail
	s.DKGPhaseLen = 30
	s.EpochLen = 150
	s.FinalizationSafetyThreshold = 20
	s.NumOfCollectionClusters = 1
	// we need to use 3 consensus nodes to be able to eject a single node from the consensus committee
	// and still have a DKG committee which meets the protocol.RandomBeaconSafetyThreshold
	s.NumOfConsensusNodes = 3

	// run the generic setup, which starts up the network
	s.BaseSuite.SetupTest()
}

// getNodeInfoDirs returns the internal node private info dir and the node config dir from a container with the specified role.
func (s *RecoverEpochSuite) getNodeInfoDirs(role flow.Role) (string, string) {
	bootstrapPath := s.GetContainersByRole(role)[0].BootstrapPath()
	internalNodePrivInfoDir := fmt.Sprintf("%s/%s", bootstrapPath, bootstrap.DirPrivateRoot)
	nodeConfigJson := fmt.Sprintf("%s/%s", bootstrapPath, bootstrap.PathNodeInfosPub)
	return internalNodePrivInfoDir, nodeConfigJson
}

// executeEFMRecoverTXArgsCMD executes the efm-recover-tx-args CLI command to generate EpochRecover transaction arguments.
// Args:
//
//	collectionClusters: the number of collector clusters.
//	numViewsInEpoch: the number of views in the recovery epoch.
//	numViewsInStakingAuction: the number of views in the staking auction of the recovery epoch.
//	epochCounter: the epoch counter.
//	targetDuration: the target duration for the recover epoch.
//	targetEndTime: the target end time for the recover epoch.
//
// Returns:
//
//	[]cadence.Value: the transaction arguments.
func (s *RecoverEpochSuite) executeEFMRecoverTXArgsCMD(
	collectionClusters int,
	numViewsInEpoch,
	numViewsInStakingAuction,
	recoveryEpochCounter,
	targetDuration uint64,
	unsafeAllowOverWrite bool) []cadence.Value {
	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	snapshot := s.GetLatestProtocolSnapshot(s.Ctx)
	txArgs, err := run.GenerateRecoverEpochTxArgs(
		s.Log,
		internalNodePrivInfoDir,
		nodeConfigJson,
		collectionClusters,
		recoveryEpochCounter,
		flow.Localnet,
		numViewsInStakingAuction,
		numViewsInEpoch,
		targetDuration,
		unsafeAllowOverWrite,
		snapshot,
	)
	require.NoError(s.T(), err)
	return txArgs
}

// recoverEpoch submits the recover epoch transaction to the network.
func (s *RecoverEpochSuite) recoverEpoch(env templates.Environment, args []cadence.Value) *sdk.TransactionResult {
	latestBlockID, err := s.Client.GetLatestBlockID(s.Ctx)
	require.NoError(s.T(), err)

	tx, err := utils.MakeRecoverEpochTx(
		env,
		s.Client.Account(),
		0,
		sdk.Identifier(latestBlockID),
		args,
	)
	require.NoError(s.T(), err)

	err = s.Client.SignAndSendTransaction(s.Ctx, tx)
	require.NoError(s.T(), err)
	result, err := s.Client.WaitForSealed(s.Ctx, tx.ID())
	require.NoError(s.T(), err)
	s.Client.Account().Keys[0].SequenceNumber++
	require.NoError(s.T(), result.Error)

	return result
}

// TestRecoverEpoch ensures that the recover epoch governance transaction flow works as expected, and a network that
// enters Epoch Fallback Mode can successfully recover.
// For this specific scenario, we are testing a scenario where the consensus committee is equal to the DKG committee, i.e.,
// no changes to the identity table between epoch start and submitting the recover epoch transaction were made.
// This test will do the following:
// 1. Triggers EFM by turning off the sole collection node before the end of the DKG forcing the DKG to fail.
// 2. Generates epoch recover transaction args using the epoch efm-recover-tx-args.
// 3. Submit recover epoch transaction.
// 4. Ensure expected EpochRecover event is emitted.
// 5. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
func (s *RecoverEpochSuite) TestRecoverEpoch() {
	// 1. Manually trigger EFM
	// wait until the epoch setup phase to force network into EFM
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 10*time.Second, 500*time.Millisecond)

	// pause the collection node to trigger EFM by failing DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	require.NoError(s.T(), ln.Pause())
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view form the latest snapshot
	epoch1FinalView, err := s.Net.BootstrapSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// Wait for at least the first view past the current epoch's original FinalView to be finalized.
	// At this point we can observe that an extension has been added to the current epoch, indicating EFM.
	s.TimedLogf("waiting for epoch transition (finalized view %d)", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", epoch1FinalView+1)

	// assert transition to second epoch did not happen
	// if counter is still 0, epoch emergency fallback was triggered as expected
	s.AssertInEpoch(s.Ctx, 0)

	// 2. Generate transaction arguments for epoch recover transaction.
	collectionClusters := s.NumOfCollectionClusters
	numViewsInRecoveryEpoch := s.EpochLen
	numViewsInStakingAuction := s.StakingAuctionLen
	epochCounter := uint64(1)

	txArgs := s.executeEFMRecoverTXArgsCMD(
		collectionClusters,
		numViewsInRecoveryEpoch,
		numViewsInStakingAuction,
		epochCounter,
		// cruise control is disabled for integration tests
		// targetDuration and targetEndTime will be ignored
		3000,
		// unsafeAllowOverWrite set to false, initialize new epoch
		false,
	)

	// 3. Submit recover epoch transaction to the network.
	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)
	require.NoError(s.T(), result.Error)
	require.Equal(s.T(), result.Status, sdk.TransactionStatusSealed)

	// 4. Ensure expected EpochRecover event is emitted.
	eventType := ""
	for _, evt := range result.Events {
		if strings.Contains(evt.Type, "FlowEpoch.EpochRecover") {
			eventType = evt.Type
			break
		}
	}
	require.NotEmpty(s.T(), eventType, "expected FlowEpoch.EpochRecover event type")
	events, err := s.Client.GetEventsForBlockIDs(s.Ctx, eventType, []sdk.Identifier{result.BlockID})
	require.NoError(s.T(), err)
	require.Equal(s.T(), events[0].Events[0].Type, eventType)

	// 5. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
	startViewOfNextEpoch := uint64(txArgs[1].(cadence.UInt64))
	s.TimedLogf("waiting to transition into recovery epoch (finalized view %d)", startViewOfNextEpoch)
	s.AwaitFinalizedView(s.Ctx, startViewOfNextEpoch, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized first view of recovery epoch %d", startViewOfNextEpoch)

	s.AssertInEpoch(s.Ctx, 1)
}

// TestRecoverEpochNodeEjected ensures that the recover epoch governance transaction flow works as expected, and a network that
// enters Epoch Fallback Mode can successfully recover.
// For this specific scenario, we are testing a scenario where the consensus committee is a subset of the DKG committee, i.e.,
// a node was ejected between epoch start and submitting the recover epoch transaction.
// This test will do the following:
// 1. Triggers EFM by turning off the sole collection node before the end of the DKG forcing the DKG to fail.
// 2. Generates epoch recover transaction args using the epoch efm-recover-tx-args.
// 3. Eject consensus node by modifying the snapshot before generating the recover epoch transaction args.
// 4. Submit recover epoch transaction.
// 5. Ensure expected EpochRecover event is emitted.
// 6. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
func (s *RecoverEpochSuite) TestRecoverEpochNodeEjected() {
	// 1. Manually trigger EFM
	// wait until the epoch setup phase to force network into EFM
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 10*time.Second, 500*time.Millisecond)

	// pause the collection node to trigger EFM by failing DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	require.NoError(s.T(), ln.Pause())
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view form the latest snapshot
	epoch1FinalView, err := s.Net.BootstrapSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// Wait for at least the first view past the current epoch's original FinalView to be finalized.
	// At this point we can observe that an extension has been added to the current epoch, indicating EFM.
	s.TimedLogf("waiting for epoch transition (finalized view %d)", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", epoch1FinalView+1)

	// assert transition to second epoch did not happen
	// if counter is still 0, epoch emergency fallback was triggered as expected
	s.AssertInEpoch(s.Ctx, 0)

	// 2. Generate transaction arguments for epoch recover transaction.
	collectionClusters := s.NumOfCollectionClusters
	recoveryEpochCounter := uint64(1)

	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	snapshot := s.GetLatestProtocolSnapshot(s.Ctx).Encodable()
	// 3. Eject consensus node by modifying the snapshot before generating the recover epoch transaction args.
	snapshot.SealingSegment.LatestProtocolStateEntry().EpochEntry.CurrentEpochIdentityTable.
		Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))[0].EpochParticipationStatus = flow.EpochParticipationStatusEjected

	txArgs, err := run.GenerateRecoverEpochTxArgs(
		s.Log,
		internalNodePrivInfoDir,
		nodeConfigJson,
		collectionClusters,
		recoveryEpochCounter,
		flow.Localnet,
		s.StakingAuctionLen,
		s.EpochLen,
		3000,
		false,
		inmem.SnapshotFromEncodable(snapshot),
	)
	require.NoError(s.T(), err)

	// 4. Submit recover epoch transaction to the network.
	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)
	require.NoError(s.T(), result.Error)
	require.Equal(s.T(), result.Status, sdk.TransactionStatusSealed)

	// 5. Ensure expected EpochRecover event is emitted.
	eventType := ""
	for _, evt := range result.Events {
		if strings.Contains(evt.Type, "FlowEpoch.EpochRecover") {
			eventType = evt.Type
			break
		}
	}
	require.NotEmpty(s.T(), eventType, "expected FlowEpoch.EpochRecover event type")
	events, err := s.Client.GetEventsForBlockIDs(s.Ctx, eventType, []sdk.Identifier{result.BlockID})
	require.NoError(s.T(), err)
	require.Equal(s.T(), events[0].Events[0].Type, eventType)

	// 6. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
	startViewOfNextEpoch := uint64(txArgs[1].(cadence.UInt64))
	s.TimedLogf("waiting to transition into recovery epoch (finalized view %d)", startViewOfNextEpoch)
	s.AwaitFinalizedView(s.Ctx, startViewOfNextEpoch, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized first view of recovery epoch %d", startViewOfNextEpoch)

	s.AssertInEpoch(s.Ctx, 1)
}

// TestRecoverEpochNodeEjected ensures that the recover epoch governance transaction flow works as expected, and a network that
// enters Epoch Fallback Mode can successfully recover.
// For this specific scenario, we are testing a scenario where the consensus committee is a subset of the DKG committee, i.e.,
// a node was ejected between epoch start and submitting the recover epoch transaction.
// This test will do the following:
// 1. Triggers EFM by turning off the sole collection node before the end of the DKG forcing the DKG to fail.
// 2. Generates epoch recover transaction args using the epoch efm-recover-tx-args.
// 3. Eject consensus node by modifying the snapshot before generating the recover epoch transaction args.
// 4. Submit recover epoch transaction.
// 5. Ensure expected EpochRecover event is emitted.
// 6. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
func (s *RecoverEpochSuite) TestRecoverEpochEjectNodeDifferentDKG() {
	// 1. Manually trigger EFM
	// wait until the epoch setup phase to force network into EFM
	s.AwaitEpochPhase(s.Ctx, 0, flow.EpochPhaseSetup, 10*time.Second, 500*time.Millisecond)

	// pause the collection node to trigger EFM by failing DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	require.NoError(s.T(), ln.Pause())
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view form the latest snapshot
	epoch1FinalView, err := s.Net.BootstrapSnapshot.Epochs().Current().FinalView()
	require.NoError(s.T(), err)

	// Wait for at least the first view past the current epoch's original FinalView to be finalized.
	// At this point we can observe that an extension has been added to the current epoch, indicating EFM.
	s.TimedLogf("waiting for epoch transition (finalized view %d)", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", epoch1FinalView+1)

	// assert transition to second epoch did not happen
	// if counter is still 0, epoch emergency fallback was triggered as expected
	s.AssertInEpoch(s.Ctx, 0)

	// 2. Generate transaction arguments for epoch recover transaction.
	collectionClusters := s.NumOfCollectionClusters
	recoveryEpochCounter := uint64(1)

	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	snapshot := s.GetLatestProtocolSnapshot(s.Ctx).Encodable()
	// 3. Eject consensus node by modifying the snapshot before generating the recover epoch transaction args.
	snapshot.SealingSegment.LatestProtocolStateEntry().EpochEntry.CurrentEpochIdentityTable.
		Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))[0].EpochParticipationStatus = flow.EpochParticipationStatusEjected

	txArgs, err := run.GenerateRecoverEpochTxArgs(
		s.Log,
		internalNodePrivInfoDir,
		nodeConfigJson,
		collectionClusters,
		recoveryEpochCounter,
		flow.Localnet,
		s.StakingAuctionLen,
		s.EpochLen,
		3000,
		false,
		inmem.SnapshotFromEncodable(snapshot),
	)
	require.NoError(s.T(), err)

	// 4. Submit recover epoch transaction to the network.
	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)
	require.NoError(s.T(), result.Error)
	require.Equal(s.T(), result.Status, sdk.TransactionStatusSealed)

	// 5. Ensure expected EpochRecover event is emitted.
	eventType := ""
	for _, evt := range result.Events {
		if strings.Contains(evt.Type, "FlowEpoch.EpochRecover") {
			eventType = evt.Type
			break
		}
	}
	require.NotEmpty(s.T(), eventType, "expected FlowEpoch.EpochRecover event type")
	events, err := s.Client.GetEventsForBlockIDs(s.Ctx, eventType, []sdk.Identifier{result.BlockID})
	require.NoError(s.T(), err)
	require.Equal(s.T(), events[0].Events[0].Type, eventType)

	// 6. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
	startViewOfNextEpoch := uint64(txArgs[1].(cadence.UInt64))
	s.TimedLogf("waiting to transition into recovery epoch (finalized view %d)", startViewOfNextEpoch)
	s.AwaitFinalizedView(s.Ctx, startViewOfNextEpoch, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized first view of recovery epoch %d", startViewOfNextEpoch)

	s.AssertInEpoch(s.Ctx, 1)
}

// generateRecoverTxArgsWithCustomDKG generates the required transaction arguments for the `recoverEpoch` transaction.
// No errors are expected during normal operation.
func GenerateRecoverTxArgsWithDKG(log zerolog.Logger,
	internalNodes []bootstrap.NodeInfo,
	collectionClusters int,
	recoveryEpochCounter uint64,
	rootChainID flow.ChainID,
	numViewsInStakingAuction uint64,
	numViewsInEpoch uint64,
	targetDuration uint64,
	unsafeAllowOverWrite bool,
	dkgIndexMap flow.DKGIndexMap,
	dkgParticipantKeys []crypto.PublicKey,
	dkgGroupKey crypto.PublicKey,
	snapshot *inmem.Snapshot,
) ([]cadence.Value, error) {
	epoch := snapshot.Epochs().Current()

	currentEpochIdentities, err := snapshot.Identities(filter.IsValidProtocolParticipant)
	if err != nil {
		return nil, fmt.Errorf("failed to get  valid protocol participants from snapshot: %w", err)
	}
	// We need canonical ordering here; sanity check to enforce this:
	if !currentEpochIdentities.Sorted(flow.Canonical[flow.Identity]) {
		return nil, fmt.Errorf("identies from snapshot not in canonical order")
	}

	// separate collector nodes by internal and partner nodes
	collectors := currentEpochIdentities.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	internalCollectors := make(flow.IdentityList, 0)
	partnerCollectors := make(flow.IdentityList, 0)

	internalNodesMap := make(map[flow.Identifier]struct{})
	for _, node := range internalNodes {
		internalNodesMap[node.NodeID] = struct{}{}
	}

	for _, collector := range collectors {
		if _, ok := internalNodesMap[collector.NodeID]; ok {
			internalCollectors = append(internalCollectors, collector)
		} else {
			partnerCollectors = append(partnerCollectors, collector)
		}
	}

	assignments, clusters, err := common.ConstructClusterAssignment(log, partnerCollectors, internalCollectors, collectionClusters)
	if err != nil {
		return nil, fmt.Errorf("unable to generate cluster assignment: %w", err)
	}

	clusterBlocks := run.GenerateRootClusterBlocks(recoveryEpochCounter, clusters)
	clusterQCs := run.ConstructRootQCsForClusters(log, clusters, internalNodes, clusterBlocks)

	// NOTE: The RecoveryEpoch will re-use the last successful DKG output. This means that the random beacon committee can be
	// different from the consensus committee. This could happen if the node was ejected from the consensus committee, but it still has to be
	// included in the DKG committee since the threshold signature scheme operates on pre-defined number of participants and cannot be changed.
	dkgGroupKeyCdc, cdcErr := cadence.NewString(hex.EncodeToString(dkgGroupKey.Encode()))
	if cdcErr != nil {
		return nil, fmt.Errorf("failed to get dkg group key cadence string: %w", cdcErr)
	}

	// copy DKG index map from the current epoch
	dkgIndexMapPairs := make([]cadence.KeyValuePair, 0)
	for nodeID, index := range dkgIndexMap {
		dkgIndexMapPairs = append(dkgIndexMapPairs, cadence.KeyValuePair{
			Key:   cadence.String(nodeID.String()),
			Value: cadence.NewInt(index),
		})
	}
	// copy DKG public keys from the current epoch
	dkgPubKeys := make([]cadence.Value, 0)
	for _, dkgPubKey := range dkgParticipantKeys {
		dkgPubKeyCdc, cdcErr := cadence.NewString(hex.EncodeToString(dkgPubKey.Encode()))
		if cdcErr != nil {
			return nil, fmt.Errorf("failed to get dkg pub key cadence string for node: %w", cdcErr)
		}
		dkgPubKeys = append(dkgPubKeys, dkgPubKeyCdc)
	}
	// fill node IDs
	nodeIds := make([]cadence.Value, 0)
	for _, id := range currentEpochIdentities {
		nodeIdCdc, err := cadence.NewString(id.GetNodeID().String())
		if err != nil {
			return nil, fmt.Errorf("failed to convert node ID to cadence string %s: %w", id.GetNodeID(), err)
		}
		nodeIds = append(nodeIds, nodeIdCdc)
	}

	clusterQCAddress := systemcontracts.SystemContractsForChain(rootChainID).ClusterQC.Address.String()
	qcVoteData, err := common.ConvertClusterQcsCdc(clusterQCs, clusters, clusterQCAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cluster qcs to cadence type")
	}
	currEpochFinalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("failed to get final view of current epoch")
	}
	currEpochTargetEndTime, err := epoch.TargetEndTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get target end time of current epoch")
	}

	args := []cadence.Value{
		// recovery epoch counter
		cadence.NewUInt64(recoveryEpochCounter),
		// epoch start view
		cadence.NewUInt64(currEpochFinalView + 1),
		// staking phase end view
		cadence.NewUInt64(currEpochFinalView + numViewsInStakingAuction),
		// epoch end view
		cadence.NewUInt64(currEpochFinalView + numViewsInEpoch),
		// target duration
		cadence.NewUInt64(targetDuration),
		// target end time
		cadence.NewUInt64(currEpochTargetEndTime),
		// clusters,
		common.ConvertClusterAssignmentsCdc(assignments),
		// qcVoteData
		cadence.NewArray(qcVoteData),
		// dkg pub keys
		cadence.NewArray(dkgPubKeys),
		// dkg group key,
		dkgGroupKeyCdc,
		// dkg index map
		cadence.NewDictionary(dkgIndexMapPairs),
		// node ids
		cadence.NewArray(nodeIds),
		// recover the network by initializing a new recover epoch which will increment the smart contract epoch counter
		// or overwrite the epoch metadata for the current epoch
		cadence.NewBool(unsafeAllowOverWrite),
	}

	return args, nil
}
