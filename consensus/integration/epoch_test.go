package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/mapfunc"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// should be able to reach consensus when identity table contains nodes with 0 weight.
func TestUnweightedNode(t *testing.T) {
	t.Skip("event loop needs an event handler, which will be replaced later in V2")

	stopper := NewStopper(2, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	// add a consensus node to next epoch (it will have 0 weight)
	unweightedIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nextEpochIdentities := unittest.CompleteIdentitySet(unweightedIdentity)
	rootSnapshot = withNextEpoch(rootSnapshot, nextEpochIdentities, 10_000)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where both epochs have the same identity table.
func TestStaticEpochTransition(t *testing.T) {
	t.Skip("event loop needs an event handler, which will be replaced later in V2")

	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	firstEpochCounter, err := rootSnapshot.Epochs().Current().Counter()
	require.NoError(t, err)

	// set up next epoch beginning in 4 views, with same identities as first epoch
	nextEpochIdentities, err := rootSnapshot.Identities(filter.Any)
	require.NoError(t, err)
	rootSnapshot = withNextEpoch(rootSnapshot, nextEpochIdentities, 4)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

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
	t.Skip("event loop needs an event handler, which will be replaced later in V2")
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	firstEpochCounter, err := rootSnapshot.Epochs().Current().Counter()
	require.NoError(t, err)

	// set up next epoch with 1 new consensus nodes and 2 consensus nodes from first epoch
	// 1 consensus node is removed after the first epoch
	firstEpochIdentities, err := rootSnapshot.Identities(filter.Any)
	require.NoError(t, err)

	removedIdentity := firstEpochIdentities.Filter(filter.HasRole(flow.RoleConsensus)).Sample(1)[0]
	newIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nextEpochIdentities := append(
		firstEpochIdentities.Filter(filter.Not(filter.HasNodeID(removedIdentity.NodeID))),
		newIdentity,
	)
	rootSnapshot = withNextEpoch(rootSnapshot, nextEpochIdentities, 4)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

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
	t.Skip("event loop needs an event handler, which will be replaced later in V2")
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	firstEpochCounter, err := rootSnapshot.Epochs().Current().Counter()
	require.NoError(t, err)

	// prepare a next epoch with a completely different consensus committee
	// (no overlapping consensus nodes)
	firstEpochIdentities, err := rootSnapshot.Identities(filter.Any)
	require.NoError(t, err)

	newIdentities := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
	nextEpochIdentities := append(
		firstEpochIdentities.Filter(filter.Not(filter.HasRole(flow.RoleConsensus))), // remove all consensus nodes
		newIdentities..., // add new consensus nodes
	)
	rootSnapshot = withNextEpoch(rootSnapshot, nextEpochIdentities, 4)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

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
func withNextEpoch(snapshot *inmem.Snapshot, nextEpochIdentities flow.IdentityList, curEpochViews uint64) *inmem.Snapshot {

	// convert to encodable representation for simple modification
	encodableSnapshot := snapshot.Encodable()

	nextEpochIdentities = nextEpochIdentities.Sort(order.Canonical)

	currEpoch := &encodableSnapshot.Epochs.Current                // take pointer so assignments apply
	currEpoch.FinalView = currEpoch.FirstView + curEpochViews - 1 // first epoch lasts curEpochViews
	encodableSnapshot.Epochs.Next = &inmem.EncodableEpoch{
		Counter:           currEpoch.Counter + 1,
		FirstView:         currEpoch.FinalView + 1,
		FinalView:         currEpoch.FinalView + 1 + 10000,
		RandomSource:      unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		InitialIdentities: nextEpochIdentities,
		// must include info corresponding to EpochCommit event, since we are
		// starting in committed phase
		Clustering: unittest.ClusterList(1, nextEpochIdentities),
		Clusters:   currEpoch.Clusters,
		DKG: &inmem.EncodableDKG{
			GroupKey: encodable.RandomBeaconPubKey{
				PublicKey: unittest.KeyFixture(crypto.BLSBLS12381).PublicKey(),
			},
			Participants: unittest.DKGParticipantLookup(nextEpochIdentities),
		},
	}

	// we must start the current epoch in committed phase so we can transition to the next epoch
	encodableSnapshot.Phase = flow.EpochPhaseCommitted
	encodableSnapshot.LatestSeal.ResultID = encodableSnapshot.LatestResult.ID()

	// set identities for root snapshot to include next epoch identities,
	// since we are in committed phase
	encodableSnapshot.Identities = append(
		// all the current epoch identities
		encodableSnapshot.Identities,
		// and all the NEW identities in next epoch, with 0 weight
		nextEpochIdentities.
			Filter(filter.Not(filter.In(encodableSnapshot.Identities))).
			Map(mapfunc.WithStake(0))...,
	).Sort(order.Canonical)

	return inmem.SnapshotFromEncodable(encodableSnapshot)
}
