package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/utils/unittest"
)

// should be able to reach consensus when identity table contains nodes which are joining in next epoch.
func TestUnweightedNode(t *testing.T) {
	// stop after building 2 blocks to ensure we can tolerate 0-weight (joining next
	// epoch) identities, but don't cross an epoch boundary
	stopper := NewStopper(2, 0)
	participantsData := createConsensusIdentities(t, 3)
	rootSnapshot := createRootSnapshot(t, participantsData)
	consensusParticipants := NewConsensusParticipants(participantsData)

	// add a consensus node to next epoch (it will have `flow.EpochParticipationStatusJoining` status in the current epoch)
	nextEpochParticipantsData := createConsensusIdentities(t, 1)
	// epoch 2 identities includes:
	// * same collection node from epoch 1, so cluster QCs are consistent
	// * 1 new consensus node, joining at epoch 2
	// * random nodes with other roles
	currentEpochCollectionNodes, err := rootSnapshot.Identities(filter.HasRole[flow.Identity](flow.RoleCollection))
	require.NoError(t, err)
	nextEpochIdentities := unittest.CompleteIdentitySet(
		append(
			currentEpochCollectionNodes,
			nextEpochParticipantsData.Identities()...)...,
	)
	rootSnapshot = withNextEpoch(t, rootSnapshot, nextEpochIdentities, nextEpochParticipantsData, consensusParticipants, 10_000)

	nodes, hub, runFor := createNodes(t, consensusParticipants, rootSnapshot, stopper)

	hub.WithFilter(blockNothing)

	runFor(60 * time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where both epochs have the same identity table.
func TestStaticEpochTransition(t *testing.T) {
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	participantsData := createConsensusIdentities(t, 3)
	rootSnapshot := createRootSnapshot(t, participantsData)
	consensusParticipants := NewConsensusParticipants(participantsData)

	firstEpochCounter, err := rootSnapshot.Epochs().Current().Counter()
	require.NoError(t, err)

	// set up next epoch beginning in 4 views, with same identities as first epoch
	nextEpochIdentities, err := rootSnapshot.Identities(filter.Any)
	require.NoError(t, err)
	rootSnapshot = withNextEpoch(t, rootSnapshot, nextEpochIdentities, participantsData, consensusParticipants, 4)

	nodes, hub, runFor := createNodes(t, consensusParticipants, rootSnapshot, stopper)

	hub.WithFilter(blockNothing)

	runFor(30 * time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	// confirm that we have transitioned to the new epoch
	pstate := nodes[0].state
	afterCounter, err := pstate.Final().Epochs().Current().Counter()
	require.NoError(t, err)
	assert.Equal(t, firstEpochCounter+1, afterCounter)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where the identity table changes
// but the new epoch overlaps with the previous epoch.
func TestEpochTransition_IdentitiesOverlap(t *testing.T) {
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	privateNodeInfos := createPrivateNodeIdentities(4)
	firstEpochConsensusParticipants := completeConsensusIdentities(t, privateNodeInfos[:3])
	rootSnapshot := createRootSnapshot(t, firstEpochConsensusParticipants)
	consensusParticipants := NewConsensusParticipants(firstEpochConsensusParticipants)

	firstEpochCounter, err := rootSnapshot.Epochs().Current().Counter()
	require.NoError(t, err)

	// set up next epoch with 1 new consensus nodes and 2 consensus nodes from first epoch
	// 1 consensus node is removed after the first epoch
	firstEpochIdentities, err := rootSnapshot.Identities(filter.Any)
	require.NoError(t, err)

	removedIdentity := privateNodeInfos[0].Identity()
	newIdentity := privateNodeInfos[3].Identity()
	nextEpochIdentities := append(
		firstEpochIdentities.Filter(filter.Not(filter.HasNodeID[flow.Identity](removedIdentity.NodeID))),
		newIdentity,
	)

	// generate new identities for next epoch, it will generate new DKG keys for random beacon participants
	nextEpochParticipantData := completeConsensusIdentities(t, privateNodeInfos[1:])
	rootSnapshot = withNextEpoch(t, rootSnapshot, nextEpochIdentities, nextEpochParticipantData, consensusParticipants, 4)

	nodes, hub, runFor := createNodes(t, consensusParticipants, rootSnapshot, stopper)

	hub.WithFilter(blockNothing)

	runFor(30 * time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	// confirm that we have transitioned to the new epoch
	pstate := nodes[0].state
	afterCounter, err := pstate.Final().Epochs().Current().Counter()
	require.NoError(t, err)
	assert.Equal(t, firstEpochCounter+1, afterCounter)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where the identity table in the new
// epoch is disjoint from the identity table in the first epoch.
func TestEpochTransition_IdentitiesDisjoint(t *testing.T) {
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	firstEpochConsensusParticipants := createConsensusIdentities(t, 3)
	rootSnapshot := createRootSnapshot(t, firstEpochConsensusParticipants)
	consensusParticipants := NewConsensusParticipants(firstEpochConsensusParticipants)

	firstEpochCounter, err := rootSnapshot.Epochs().Current().Counter()
	require.NoError(t, err)

	// prepare a next epoch with a completely different consensus committee
	// (no overlapping consensus nodes)
	firstEpochIdentities, err := rootSnapshot.Identities(filter.Any)
	require.NoError(t, err)

	nextEpochParticipantData := createConsensusIdentities(t, 3)
	nextEpochIdentities := append(
		firstEpochIdentities.Filter(filter.Not(filter.HasRole[flow.Identity](flow.RoleConsensus))), // remove all consensus nodes
		nextEpochParticipantData.Identities()...,                                                   // add new consensus nodes
	)

	rootSnapshot = withNextEpoch(t, rootSnapshot, nextEpochIdentities, nextEpochParticipantData, consensusParticipants, 4)

	nodes, hub, runFor := createNodes(t, consensusParticipants, rootSnapshot, stopper)

	hub.WithFilter(blockNothing)

	runFor(60 * time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	// confirm that we have transitioned to the new epoch
	pstate := nodes[0].state
	afterCounter, err := pstate.Final().Epochs().Current().Counter()
	require.NoError(t, err)
	assert.Equal(t, firstEpochCounter+1, afterCounter)

	cleanupNodes(nodes)
}

// withNextEpoch adds a valid next epoch with the given identities to the input
// snapshot. Also sets the length of the first (current) epoch to curEpochViews.
//
// We make the first (current) epoch start in committed phase so we can transition
// to the next epoch upon reaching the appropriate view without any further changes
// to the protocol state.
func withNextEpoch(t *testing.T, snapshot *inmem.Snapshot, nextEpochIdentities flow.IdentityList, nextEpochParticipantData *run.ParticipantData, participantsCache *ConsensusParticipants, curEpochViews uint64) *inmem.Snapshot {
	nextEpochIdentities = nextEpochIdentities.Sort(flow.Canonical[flow.Identity])

	// convert to encodable representation for simple modification
	encodableSnapshot := snapshot.Encodable()

	rootProtocolState := encodableSnapshot.SealingSegment.LatestProtocolStateEntry()
	epochProtocolState := rootProtocolState.EpochEntry
	currEpochSetup := epochProtocolState.CurrentEpochSetup
	currEpochCommit := epochProtocolState.CurrentEpochCommit

	currEpochSetup.FinalView = currEpochSetup.FirstView + curEpochViews - 1

	nextEpochSetup := &flow.EpochSetup{
		Counter:      currEpochSetup.Counter + 1,
		FirstView:    currEpochSetup.FinalView + 1,
		FinalView:    currEpochSetup.FinalView + 1 + 10000,
		RandomSource: unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		Participants: nextEpochIdentities.ToSkeleton(),
		Assignments:  unittest.ClusterAssignment(1, nextEpochIdentities.ToSkeleton()),
	}
	nextEpochCommit := &flow.EpochCommit{
		Counter: nextEpochSetup.Counter,
		// TODO do we need these values?
		ClusterQCs:         currEpochCommit.ClusterQCs,
		DKGParticipantKeys: nextEpochParticipantData.PublicKeys(),
		DKGGroupKey:        nextEpochParticipantData.GroupKey,
	}
	epochProtocolState.NextEpochSetup = nextEpochSetup
	epochProtocolState.NextEpochCommit = nextEpochCommit
	epochProtocolState.NextEpoch = &flow.EpochStateContainer{
		SetupID:          nextEpochSetup.ID(),
		CommitID:         nextEpochCommit.ID(),
		ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(nextEpochIdentities),
	}

	rootKVStore := kvstore.NewDefaultKVStore(epochProtocolState.ID())
	protocolVersion, encodedKVStore, err := rootKVStore.VersionedEncode()
	require.NoError(t, err)
	encodableSnapshot.SealingSegment.ProtocolStateEntries = map[flow.Identifier]*flow.ProtocolStateEntryWrapper{
		rootKVStore.ID(): {
			KVStore: flow.PSKeyValueStoreData{
				Version: protocolVersion,
				Data:    encodedKVStore,
			},
			EpochEntry: epochProtocolState,
		},
	}
	encodableSnapshot.SealingSegment.Blocks[0].Payload.ProtocolStateID = rootKVStore.ID()

	participantsCache.Update(nextEpochSetup.Counter, nextEpochParticipantData)

	// TODO necessary?
	encodableSnapshot.LatestSeal.ResultID = encodableSnapshot.LatestResult.ID()

	return inmem.SnapshotFromEncodable(encodableSnapshot)
}
