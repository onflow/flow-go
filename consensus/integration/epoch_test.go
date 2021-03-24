package integration_test

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// should be able to reach consensus when identity table contains nodes with 0 weight.
func TestUnweightedNode(t *testing.T) {

	stopper := NewStopper(2, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	// convert to encodable form to add an un-staked consensus node
	enc := rootSnapshot.Encodable()

	// add a consensus to next epoch (it will have 0 weight)
	unweightedIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nextEpochIdentities := unittest.CompleteIdentitySet(unweightedIdentity)

	currEpoch := enc.Epochs.Current
	enc.Epochs.Next = &inmem.EncodableEpoch{
		Counter:           currEpoch.Counter,
		FirstView:         currEpoch.FinalView + 1,
		FinalView:         currEpoch.FinalView + 1 + 10000,
		RandomSource:      unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		InitialIdentities: nextEpochIdentities,
		Clustering:        unittest.ClusterList(1, nextEpochIdentities),
	}
	enc.LatestSeal.ResultID = enc.LatestResult.ID()
	enc.Phase = flow.EpochPhaseCommitted

	// convert back to protocol state snapshot
	rootSnapshot = inmem.SnapshotFromEncodable(enc)

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

	stopper := NewStopper(2, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	// convert to encodable form to add an un-staked consensus node
	enc := rootSnapshot.Encodable()

	// same identities -> same consensus committee in next epoch
	nextEpochIdentities := enc.Identities

	currEpoch := enc.Epochs.Current
	enc.Epochs.Next = &inmem.EncodableEpoch{
		Counter:           currEpoch.Counter,
		FirstView:         currEpoch.FinalView + 1,
		FinalView:         currEpoch.FinalView + 1 + 10000,
		RandomSource:      unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		InitialIdentities: nextEpochIdentities,
		Clustering:        unittest.ClusterList(1, nextEpochIdentities),
	}
	enc.LatestSeal.ResultID = enc.LatestResult.ID()
	enc.Phase = flow.EpochPhaseCommitted

	// convert back to protocol state snapshot
	rootSnapshot = inmem.SnapshotFromEncodable(enc)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where the identity table changes
// but the new epoch overlaps with the previous epoch.
func TestEpochTransition_IdentitiesOverlap(t *testing.T) {}

// test consensus across an epoch boundary, where the identity table in the new
// epoch is disjoint from the identity table in the first epoch.
func TestEpochTransition_IdentitiesDisjoint(t *testing.T) {}
