package integration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/mapfunc"
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
	enc.Identities = append(
		enc.Identities,
		nextEpochIdentities.Map(mapfunc.WithStake(0))...,
	)

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

	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	// convert to encodable form to add an un-staked consensus node
	enc := rootSnapshot.Encodable()

	// same identities -> same consensus committee in next epoch
	nextEpochIdentities := enc.Identities

	currEpoch := &enc.Epochs.Current              // take pointer so assignments apply
	currEpoch.FinalView = currEpoch.FirstView + 4 // first epoch lasts 5 views
	enc.Epochs.Next = &inmem.EncodableEpoch{
		Counter:           currEpoch.Counter + 1,
		FirstView:         currEpoch.FinalView + 1,
		FinalView:         currEpoch.FinalView + 1 + 10000,
		RandomSource:      unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		InitialIdentities: nextEpochIdentities,
		Clustering:        unittest.ClusterList(1, nextEpochIdentities),
		Clusters:          currEpoch.Clusters,
		DKG:               currEpoch.DKG,
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

	// confirm that we have transitioned to the new epoch
	pstate := nodes[0].state
	counter, err := pstate.Final().Epochs().Current().Counter()
	require.NoError(t, err)
	assert.Equal(t, enc.Epochs.Next.Counter, counter)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where the identity table changes
// but the new epoch overlaps with the previous epoch.
func TestEpochTransition_IdentitiesOverlap(t *testing.T) {
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	// convert to encodable form to add an un-staked consensus node
	enc := rootSnapshot.Encodable()

	// 2 overlapping and 1 new consensus node in next epoch
	removedIdentity := enc.Identities.Filter(filter.HasRole(flow.RoleConsensus)).Sample(1)[0]
	newIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nextEpochIdentities := append(
		enc.Identities.Filter(filter.Not(filter.HasNodeID(removedIdentity.NodeID))),
		newIdentity,
	)

	currEpoch := &enc.Epochs.Current              // take pointer so assignments apply
	currEpoch.FinalView = currEpoch.FirstView + 4 // first epoch lasts 5 views
	enc.Epochs.Next = &inmem.EncodableEpoch{
		Counter:           currEpoch.Counter + 1,
		FirstView:         currEpoch.FinalView + 1,
		FinalView:         currEpoch.FinalView + 1 + 10000,
		RandomSource:      unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		InitialIdentities: nextEpochIdentities,
		Clustering:        unittest.ClusterList(1, nextEpochIdentities),
		Clusters:          currEpoch.Clusters,
		DKG: &inmem.EncodableDKG{
			GroupKey:     encodable.RandomBeaconPubKey{unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()},
			Participants: unittest.DKGParticipantLookup(nextEpochIdentities),
		},
	}
	enc.LatestSeal.ResultID = enc.LatestResult.ID()
	enc.Phase = flow.EpochPhaseCommitted
	enc.Identities = append(enc.Identities, newIdentity)

	// convert back to protocol state snapshot
	rootSnapshot = inmem.SnapshotFromEncodable(enc)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	// confirm that we have transitioned to the new epoch
	pstate := nodes[0].state
	counter, err := pstate.Final().Epochs().Current().Counter()
	require.NoError(t, err)
	assert.Equal(t, enc.Epochs.Next.Counter, counter)

	cleanupNodes(nodes)
}

// test consensus across an epoch boundary, where the identity table in the new
// epoch is disjoint from the identity table in the first epoch.
// TODO copied from above - needs implementation
func TestEpochTransition_IdentitiesDisjoint(t *testing.T) {
	// must finalize 8 blocks, we specify the epoch transition after 4 views
	stopper := NewStopper(8, 0)
	rootSnapshot := createRootSnapshot(t, 3)

	// convert to encodable form to add an un-staked consensus node
	enc := rootSnapshot.Encodable()

	fmt.Println("epoch1 sns: ", enc.Identities.Filter(filter.HasRole(flow.RoleConsensus)).NodeIDs())

	// 2 overlapping and 1 new consensus node in next epoch
	removedIdentity := enc.Identities.Filter(filter.HasRole(flow.RoleConsensus)).Sample(1)[0]
	newIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nextEpochIdentities := append(
		enc.Identities.Filter(filter.Not(filter.HasNodeID(removedIdentity.NodeID))),
		newIdentity,
	)
	fmt.Println("epoch1 sns: ", nextEpochIdentities.Filter(filter.HasRole(flow.RoleConsensus)).NodeIDs())
	fmt.Println("removed: ", removedIdentity.NodeID)
	fmt.Println("added: ", newIdentity.NodeID)

	currEpoch := &enc.Epochs.Current              // take pointer so assignments apply
	currEpoch.FinalView = currEpoch.FirstView + 4 // first epoch lasts 5 views
	enc.Epochs.Next = &inmem.EncodableEpoch{
		Counter:           currEpoch.Counter + 1,
		FirstView:         currEpoch.FinalView + 1,
		FinalView:         currEpoch.FinalView + 1 + 10000,
		RandomSource:      unittest.SeedFixture(flow.EpochSetupRandomSourceLength),
		InitialIdentities: nextEpochIdentities,
		Clustering:        unittest.ClusterList(1, nextEpochIdentities),
		Clusters:          currEpoch.Clusters,
		DKG: &inmem.EncodableDKG{
			GroupKey:     encodable.RandomBeaconPubKey{unittest.KeyFixture(crypto.BLSBLS12381).PublicKey()},
			Participants: unittest.DKGParticipantLookup(nextEpochIdentities),
		},
	}
	enc.LatestSeal.ResultID = enc.LatestResult.ID()
	enc.Phase = flow.EpochPhaseCommitted
	enc.Identities = append(enc.Identities, newIdentity)

	// convert back to protocol state snapshot
	rootSnapshot = inmem.SnapshotFromEncodable(enc)

	nodes, hub := createNodes(t, stopper, rootSnapshot)

	hub.WithFilter(blockNothing)
	runNodes(nodes)

	unittest.AssertClosesBefore(t, stopper.stopped, 30*time.Second)

	allViews := allFinalizedViews(t, nodes)
	assertSafety(t, allViews)

	// confirm that we have transitioned to the new epoch
	pstate := nodes[0].state
	counter, err := pstate.Final().Epochs().Current().Counter()
	require.NoError(t, err)
	assert.Equal(t, enc.Epochs.Next.Counter, counter)

	cleanupNodes(nodes)
}
