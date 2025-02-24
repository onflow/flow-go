package cohort2

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/integration/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
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
	// we need to use 4 consensus nodes to be able to eject a single node and still have a super-majority and
	// have a Random Beacon committee which meets the protocol.RandomBeaconSafetyThreshold.
	s.NumOfConsensusNodes = 4

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
	recoveryEpochTargetDuration uint64,
	unsafeAllowOverWrite bool,
) []cadence.Value {
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
		recoveryEpochTargetDuration,
		unsafeAllowOverWrite,
		nil,
		nil,
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

// TestRecoverEpoch ensures that the recover epoch governance transaction flow works as expected, i.e.
// a network that entered Epoch Fallback Mode successfully recovers.
// For this specific scenario, we are testing a scenario where the consensus committee is equal to the Random Beacon committee, i.e.,
// no changes to the identity table between epoch start and submitting the recover epoch transaction were made.
// This test will do the following:
// 1. Triggers EFM by turning off the sole collection node before the end of the DKG forcing the DKG to fail.
// 2. Generates epoch recover transaction args using the epoch efm-recover-tx-args.
// 3. Submit recover epoch transaction.
// 4. Ensure expected EpochRecover event is emitted.
// 5. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
func (s *RecoverEpochSuite) TestRecoverEpoch() {
	// 1. Manually trigger EFM
	// pause the collection node to trigger EFM by failing DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	require.NoError(s.T(), ln.Pause())
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view from the latest snapshot
	epoch1, err := s.Net.BootstrapSnapshot.Epochs().Current()
	require.NoError(s.T(), err)
	epoch1FinalView := epoch1.FinalView()

	// Wait for at least the first view past the current epoch's original FinalView to be finalized.
	s.TimedLogf("waiting for epoch transition (finalized view %d)", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", epoch1FinalView+1)

	// assert that we are in EFM
	snapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)
	epochPhase, err := snapshot.EpochPhase()
	require.NoError(s.T(), err)
	require.Equal(s.T(), flow.EpochPhaseFallback, epochPhase, "network must enter EFM by this point")

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
// For this specific scenario, we are testing a scenario where the consensus committee is a subset of the Random Beacon committee, i.e.,
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

	// pause the collection node to trigger EFM by failing DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	require.NoError(s.T(), ln.Pause())
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view from the latest snapshot
	epoch1, err := s.Net.BootstrapSnapshot.Epochs().Current()
	require.NoError(s.T(), err)
	epoch1FinalView := epoch1.FinalView()

	// Wait for at least the first view past the current epoch's original FinalView to be finalized.
	s.TimedLogf("waiting for epoch transition (finalized view %d)", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", epoch1FinalView+1)

	// assert that we are in EFM
	snapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)
	epochPhase, err := snapshot.EpochPhase()
	require.NoError(s.T(), err)
	require.Equal(s.T(), flow.EpochPhaseFallback, epochPhase, "network must enter EFM by this point")

	// 2. Generate transaction arguments for epoch recover transaction.
	collectionClusters := s.NumOfCollectionClusters
	recoveryEpochCounter := uint64(1)

	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	// 3. Eject consensus node by modifying the snapshot before generating the recover epoch transaction args.
	snapshot.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.CurrentEpochIdentityTable.
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
		nil,
		nil,
		snapshot,
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

// TestRecoverEpochEjectNodeDifferentDKG ensures that the recover epoch governance transaction flow works as expected, and a network that
// enters Epoch Fallback Mode can successfully recover.
// Here, we are testing a scenario where the consensus committee 𝒞 and Random Beacon committee 𝒟 form a symmetric difference with
// cardinality 1. Formally, |𝒞 ∖ 𝒟| = 1 and |𝒟 \ 𝒞| = 1. In other words, there is a node which is part of the consensus committee but not
// part of the Random Beacon committee and another node which is part of the Random Beacon committee but not part of the consensus committee.
// We remove the first consensus node from the Consensus Committee, and the last consensus node from the Random Beacon Committee. For example,
// if the original consensus set is {A, B, C, D} then:
//   - the post-recovery consensus committee is {B, C, D}
//   - the post-recovery random beacon committee is {A, B, C}
//
// This test will do the following:
//  1. Triggers EFM by turning off the sole collection node before the end of the DKG forcing the DKG to fail.
//  2. Eject the first consensus node by modifying the epoch snapshot.
//  3. Drop the last consensus node from the Random Beacon committee. This hack works only for threshold systems with an even number of participants,
//     without changing the threshold - hence we need to start this test with 4 consensus nodes.
//  4. Generates epoch recover transaction args using the tooling [run.GenerateRecoverTxArgsWithDKG] provided for the governance committee.
//  5. Submit recover epoch transaction.
//  6. Ensure expected EpochRecover event is emitted.
//  7. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
func (s *RecoverEpochSuite) TestRecoverEpochEjectNodeDifferentDKG() {
	//  1. Triggers EFM by turning off the sole collection node before the end of the DKG forcing the DKG to fail.

	// pause the collection node to trigger EFM by failing DKG
	ln := s.GetContainersByRole(flow.RoleCollection)[0]
	require.NoError(s.T(), ln.Pause())
	s.AwaitFinalizedView(s.Ctx, s.GetDKGEndView(), 2*time.Minute, 500*time.Millisecond)
	// start the paused collection node now that we are in EFM
	require.NoError(s.T(), ln.Start())

	// get final view from the latest snapshot
	epoch1, err := s.Net.BootstrapSnapshot.Epochs().Current()
	require.NoError(s.T(), err)
	epoch1FinalView := epoch1.FinalView()

	// Wait for at least the first view past the current epoch's original FinalView to be finalized.
	s.TimedLogf("waiting for epoch transition (finalized view %d)", epoch1FinalView+1)
	s.AwaitFinalizedView(s.Ctx, epoch1FinalView+1, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized view %d", epoch1FinalView+1)

	// assert that we are in EFM
	snapshot, err := s.Client.GetLatestProtocolSnapshot(s.Ctx)
	require.NoError(s.T(), err)
	epochPhase, err := snapshot.EpochPhase()
	require.NoError(s.T(), err)
	require.Equal(s.T(), flow.EpochPhaseFallback, epochPhase, "network must enter EFM by this point")

	// 2. Eject the FIRST consensus node by modifying the snapshot before generating the recover epoch transaction args.
	// By ejecting a node from the consensus committee but keeping it in the Random Beacon committee, we ensure that the there is a node
	// which is not part of the consensus committee but is part of the Random Beacon committee.
	currentIdentityTable := snapshot.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.CurrentEpochIdentityTable
	ejectedIdentity := currentIdentityTable.Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))[0]
	ejectedIdentity.EpochParticipationStatus = flow.EpochParticipationStatusEjected // writes through to `currentIdentityTable`

	// 3. Modify DKG data by removing the last node of the consensus committee from DKG committee. This way we ensure that consensus
	// committee has a node which is not part of the Random Beacon committee. For threshold committees of *even size*, we can remove a
	// single node without changing the threshold (see [ref. 1] for details). In other words, we can just pretend that there was originally
	// one node less in the DKG, while the same number of signatures (threshold +1) are sufficient to construct a group signature.
	//
	// [ref. 1] function `RandomBeaconThreshold` for computing the threshold in package module/signature; note
	//          that for reconstructing the group sig, _strictly more_ than `threshold` sig shares are required.
	randomBeaconParticipants := currentIdentityTable.Filter(filter.HasRole[flow.Identity](flow.RoleConsensus))
	nConsensusNodes := len(randomBeaconParticipants) - 1

	//  4. Generates epoch recover transaction args using the tooling [run.GenerateRecoverTxArgsWithDKG] provided for the governance committee.
	recoveryDkgIndexMap := make(flow.DKGIndexMap, nConsensusNodes)
	for i, participant := range randomBeaconParticipants[:nConsensusNodes] {
		recoveryDkgIndexMap[participant.NodeID] = i
	}

	epochProtocolState, err := snapshot.EpochProtocolState()
	require.NoError(s.T(), err)
	dkg, err := epochProtocolState.DKG()
	require.NoError(s.T(), err)
	recoveryThresholdKeyShares := dkg.KeyShares()[:nConsensusNodes]
	recoveryThresholdGroupKey := dkg.GroupKey()

	// read internal node info from one of the consensus nodes
	internalNodePrivInfoDir, nodeConfigJson := s.getNodeInfoDirs(flow.RoleConsensus)
	internalNodes, err := common.ReadFullInternalNodeInfos(unittest.Logger(), internalNodePrivInfoDir, nodeConfigJson)
	require.NoError(s.T(), err)

	// At this point we have a node which is part of the consensus committee but not part of the Random Beacon committee and
	// another node which is part of the Random Beacon committee but not part of the consensus committee.
	collectionClusters := s.NumOfCollectionClusters
	recoveryEpochCounter := uint64(1)
	txArgs, err := run.GenerateRecoverTxArgsWithDKG(
		s.Log,
		internalNodes,
		collectionClusters,
		recoveryEpochCounter,
		flow.Localnet,
		s.StakingAuctionLen,
		s.EpochLen,
		3000,
		false,
		recoveryDkgIndexMap,
		recoveryThresholdKeyShares,
		recoveryThresholdGroupKey,
		nil,
		nil,
		snapshot,
	)
	require.NoError(s.T(), err)

	// 5. Submit recover epoch transaction to the network.
	env := utils.LocalnetEnv()
	result := s.recoverEpoch(env, txArgs)
	require.NoError(s.T(), result.Error)
	require.Equal(s.T(), result.Status, sdk.TransactionStatusSealed)

	// 6. Ensure expected EpochRecover event is emitted.
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

	// 7. Ensure the network transitions into the recovery epoch and finalizes the first view of the recovery epoch.
	startViewOfNextEpoch := uint64(txArgs[1].(cadence.UInt64))
	s.TimedLogf("waiting to transition into recovery epoch (finalized view %d)", startViewOfNextEpoch)
	s.AwaitFinalizedView(s.Ctx, startViewOfNextEpoch, 2*time.Minute, 500*time.Millisecond)
	s.TimedLogf("observed finalized first view of recovery epoch %d", startViewOfNextEpoch)

	s.AssertInEpoch(s.Ctx, 1)
}
