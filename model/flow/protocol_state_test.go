package flow_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEpochProtocolStateEntry_EpochPhase tests that all possible instances of an EpochMinStateEntry
// correctly compute the current epoch phase, taking into account EFM status and incorporated service events.
func TestEpochProtocolStateEntry_EpochPhase(t *testing.T) {

	t.Run("EFM triggered", func(t *testing.T) {
		t.Run("tentatively in staking phase", func(t *testing.T) {
			entry := unittest.EpochProtocolStateEntryFixture(flow.EpochPhaseStaking, true)
			assert.Equal(t, flow.EpochPhaseFallback, entry.EpochPhase())
		})
		t.Run("tentatively in setup phase", func(t *testing.T) {
			entry := unittest.EpochProtocolStateEntryFixture(flow.EpochPhaseSetup, true)
			assert.Equal(t, flow.EpochPhaseFallback, entry.EpochPhase())
		})
		t.Run("tentatively in committed phase", func(t *testing.T) {
			entry := unittest.EpochProtocolStateEntryFixture(flow.EpochPhaseCommitted, true)
			assert.Equal(t, flow.EpochPhaseCommitted, entry.EpochPhase())
		})
	})

	t.Run("EFM not triggered", func(t *testing.T) {
		t.Run("tentatively in staking phase", func(t *testing.T) {
			entry := unittest.EpochProtocolStateEntryFixture(flow.EpochPhaseStaking, false)
			assert.Equal(t, flow.EpochPhaseStaking, entry.EpochPhase())
		})
		t.Run("tentatively in setup phase", func(t *testing.T) {
			entry := unittest.EpochProtocolStateEntryFixture(flow.EpochPhaseSetup, false)
			assert.Equal(t, flow.EpochPhaseSetup, entry.EpochPhase())
		})
		t.Run("tentatively in committed phase", func(t *testing.T) {
			entry := unittest.EpochProtocolStateEntryFixture(flow.EpochPhaseCommitted, false)
			assert.Equal(t, flow.EpochPhaseCommitted, entry.EpochPhase())
		})
	})
}

// TestNewRichProtocolStateEntry checks that NewEpochRichStateEntry creates valid identity tables depending on the state
// of epoch which is derived from the protocol state entry.
func TestNewRichProtocolStateEntry(t *testing.T) {
	// Conditions right after a spork:
	//  * no previous epoch exists from the perspective of the freshly-sporked protocol state
	//  * network is currently in the staking phase for the next epoch, hence no service events for the next epoch exist
	t.Run("staking-root-protocol-state", func(t *testing.T) {
		setup := unittest.EpochSetupFixture()
		currentEpochCommit := unittest.EpochCommitFixture()
		identities := make(flow.DynamicIdentityEntryList, 0, len(setup.Participants))
		for _, identity := range setup.Participants {
			identities = append(identities, &flow.DynamicIdentityEntry{
				NodeID:  identity.NodeID,
				Ejected: false,
			})
		}
		minStateEntry := &flow.EpochMinStateEntry{
			PreviousEpoch: nil,
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:          setup.ID(),
				CommitID:         currentEpochCommit.ID(),
				ActiveIdentities: identities,
			},
			EpochFallbackTriggered: false,
		}
		stateEntry, err := flow.NewEpochStateEntry(
			minStateEntry,
			nil,
			nil,
			setup,
			currentEpochCommit,
			nil,
			nil,
		)
		assert.NoError(t, err)
		assert.Equal(t, flow.EpochPhaseStaking, stateEntry.EpochPhase())

		richStateEntry, err := flow.NewEpochRichStateEntry(stateEntry)
		assert.NoError(t, err)

		expectedIdentities, err := flow.BuildIdentityTable(
			setup.Participants,
			identities,
			nil,
			nil,
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.CurrentEpochIdentityTable, "should be equal to current epoch setup participants")
	})

	// Common situation during the staking phase for epoch N+1
	//  * we are currently in Epoch N
	//  * previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
	//  * network is currently in the staking phase for the next epoch, hence no service events for the next epoch exist
	t.Run("staking-phase", func(t *testing.T) {
		stateEntryFixture := unittest.EpochStateFixture()
		epochStateEntry, err := flow.NewEpochStateEntry(
			stateEntryFixture.EpochMinStateEntry,
			stateEntryFixture.PreviousEpochSetup,
			stateEntryFixture.PreviousEpochCommit,
			stateEntryFixture.CurrentEpochSetup,
			stateEntryFixture.CurrentEpochCommit,
			nil,
			nil,
		)
		assert.NoError(t, err)
		assert.Equal(t, flow.EpochPhaseStaking, epochStateEntry.EpochPhase())

		epochRichStateEntry, err := flow.NewEpochRichStateEntry(epochStateEntry)
		assert.NoError(t, err)
		expectedIdentities, err := flow.BuildIdentityTable(
			stateEntryFixture.CurrentEpochSetup.Participants,
			stateEntryFixture.CurrentEpoch.ActiveIdentities,
			stateEntryFixture.PreviousEpochSetup.Participants,
			stateEntryFixture.PreviousEpoch.ActiveIdentities,
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, epochRichStateEntry.CurrentEpochIdentityTable, "should be equal to current epoch setup participants + previous epoch setup participants")
		assert.Nil(t, epochRichStateEntry.NextEpoch)
	})

	// Common situation during the epoch setup phase for epoch N+1
	//  * we are currently in Epoch N
	//  * previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
	//  * network is currently in the setup phase for the next epoch, i.e. EpochSetup event (starting setup phase) has already been observed
	t.Run("setup-phase", func(t *testing.T) {
		stateEntryFixture := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.EpochRichStateEntry) {
			entry.NextEpochCommit = nil
			entry.NextEpoch.CommitID = flow.ZeroID
		})

		stateEntry, err := flow.NewEpochStateEntry(
			stateEntryFixture.EpochMinStateEntry,
			stateEntryFixture.PreviousEpochSetup,
			stateEntryFixture.PreviousEpochCommit,
			stateEntryFixture.CurrentEpochSetup,
			stateEntryFixture.CurrentEpochCommit,
			stateEntryFixture.NextEpochSetup,
			nil,
		)
		assert.NoError(t, err)
		assert.Equal(t, flow.EpochPhaseSetup, stateEntry.EpochPhase())

		richStateEntry, err := flow.NewEpochRichStateEntry(stateEntry)
		assert.NoError(t, err)
		expectedIdentities, err := flow.BuildIdentityTable(
			stateEntryFixture.CurrentEpochSetup.Participants,
			stateEntryFixture.CurrentEpoch.ActiveIdentities,
			stateEntryFixture.NextEpochSetup.Participants,
			stateEntryFixture.NextEpoch.ActiveIdentities,
			flow.EpochParticipationStatusJoining,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.CurrentEpochIdentityTable, "should be equal to current epoch setup participants + next epoch setup participants")
		assert.Nil(t, richStateEntry.NextEpochCommit)
		expectedIdentities, err = flow.BuildIdentityTable(
			stateEntryFixture.NextEpochSetup.Participants,
			stateEntryFixture.NextEpoch.ActiveIdentities,
			stateEntryFixture.CurrentEpochSetup.Participants,
			stateEntryFixture.CurrentEpoch.ActiveIdentities,
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.NextEpochIdentityTable, "should be equal to next epoch setup participants + current epoch setup participants")
	})

	t.Run("setup-after-spork", func(t *testing.T) {
		stateEntryFixture := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.EpochRichStateEntry) {
			// no previous epoch since we are in the first epoch
			entry.PreviousEpochSetup = nil
			entry.PreviousEpochCommit = nil
			entry.PreviousEpoch = nil

			// next epoch is setup but not committed
			entry.NextEpochCommit = nil
			entry.NextEpoch.CommitID = flow.ZeroID
		})
		// sanity check that previous epoch is not populated in `stateEntry`
		assert.Nil(t, stateEntryFixture.PreviousEpoch)
		assert.Nil(t, stateEntryFixture.PreviousEpochSetup)
		assert.Nil(t, stateEntryFixture.PreviousEpochCommit)

		stateEntry, err := flow.NewEpochStateEntry(
			stateEntryFixture.EpochMinStateEntry,
			stateEntryFixture.PreviousEpochSetup,
			stateEntryFixture.PreviousEpochCommit,
			stateEntryFixture.CurrentEpochSetup,
			stateEntryFixture.CurrentEpochCommit,
			stateEntryFixture.NextEpochSetup,
			nil,
		)
		assert.NoError(t, err)
		assert.Equal(t, flow.EpochPhaseSetup, stateEntry.EpochPhase())

		richStateEntry, err := flow.NewEpochRichStateEntry(stateEntry)
		assert.NoError(t, err)
		expectedIdentities, err := flow.BuildIdentityTable(
			stateEntry.CurrentEpochSetup.Participants,
			stateEntry.CurrentEpoch.ActiveIdentities,
			stateEntry.NextEpochSetup.Participants,
			stateEntry.NextEpoch.ActiveIdentities,
			flow.EpochParticipationStatusJoining,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.CurrentEpochIdentityTable, "should be equal to current epoch setup participants + next epoch setup participants")
		assert.Nil(t, richStateEntry.NextEpochCommit)
		expectedIdentities, err = flow.BuildIdentityTable(
			stateEntry.NextEpochSetup.Participants,
			stateEntry.NextEpoch.ActiveIdentities,
			stateEntry.CurrentEpochSetup.Participants,
			stateEntry.CurrentEpoch.ActiveIdentities,
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.NextEpochIdentityTable, "should be equal to next epoch setup participants + current epoch setup participants")
	})

	// Common situation during the epoch commit phase for epoch N+1
	//  * we are currently in Epoch N
	//  * previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
	//  * The network has completed the epoch setup phase, i.e. published the EpochSetup and EpochCommit events for epoch N+1.
	t.Run("commit-phase", func(t *testing.T) {
		stateEntryFixture := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())

		stateEntry, err := flow.NewEpochStateEntry(
			stateEntryFixture.EpochMinStateEntry,
			stateEntryFixture.PreviousEpochSetup,
			stateEntryFixture.PreviousEpochCommit,
			stateEntryFixture.CurrentEpochSetup,
			stateEntryFixture.CurrentEpochCommit,
			stateEntryFixture.NextEpochSetup,
			stateEntryFixture.NextEpochCommit,
		)
		assert.NoError(t, err)
		assert.Equal(t, flow.EpochPhaseCommitted, stateEntry.EpochPhase())

		richStateEntry, err := flow.NewEpochRichStateEntry(stateEntry)
		assert.NoError(t, err)
		expectedIdentities, err := flow.BuildIdentityTable(
			stateEntry.CurrentEpochSetup.Participants,
			stateEntry.CurrentEpoch.ActiveIdentities,
			stateEntry.NextEpochSetup.Participants,
			stateEntry.NextEpoch.ActiveIdentities,
			flow.EpochParticipationStatusJoining,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.CurrentEpochIdentityTable, "should be equal to current epoch setup participants + next epoch setup participants")
		expectedIdentities, err = flow.BuildIdentityTable(
			stateEntry.NextEpochSetup.Participants,
			stateEntry.NextEpoch.ActiveIdentities,
			stateEntry.CurrentEpochSetup.Participants,
			stateEntry.CurrentEpoch.ActiveIdentities,
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.NextEpochIdentityTable, "should be equal to next epoch setup participants + current epoch setup participants")
	})

	t.Run("commit-after-spork", func(t *testing.T) {
		stateEntryFixture := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.EpochRichStateEntry) {
			// no previous epoch since we are in the first epoch
			entry.PreviousEpochSetup = nil
			entry.PreviousEpochCommit = nil
			entry.PreviousEpoch = nil
		})
		// sanity check that previous epoch is not populated in `stateEntryFixture`
		assert.Nil(t, stateEntryFixture.PreviousEpoch)
		assert.Nil(t, stateEntryFixture.PreviousEpochSetup)
		assert.Nil(t, stateEntryFixture.PreviousEpochCommit)

		stateEntry, err := flow.NewEpochStateEntry(
			stateEntryFixture.EpochMinStateEntry,
			stateEntryFixture.PreviousEpochSetup,
			stateEntryFixture.PreviousEpochCommit,
			stateEntryFixture.CurrentEpochSetup,
			stateEntryFixture.CurrentEpochCommit,
			stateEntryFixture.NextEpochSetup,
			stateEntryFixture.NextEpochCommit,
		)
		assert.NoError(t, err)
		assert.Equal(t, flow.EpochPhaseCommitted, stateEntry.EpochPhase())

		richStateEntry, err := flow.NewEpochRichStateEntry(stateEntry)
		assert.NoError(t, err)
		expectedIdentities, err := flow.BuildIdentityTable(
			stateEntryFixture.CurrentEpochSetup.Participants,
			stateEntryFixture.CurrentEpoch.ActiveIdentities,
			stateEntryFixture.NextEpochSetup.Participants,
			stateEntryFixture.NextEpoch.ActiveIdentities,
			flow.EpochParticipationStatusJoining,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.CurrentEpochIdentityTable, "should be equal to current epoch setup participants + next epoch setup participants")
		expectedIdentities, err = flow.BuildIdentityTable(
			stateEntryFixture.NextEpochSetup.Participants,
			stateEntryFixture.NextEpoch.ActiveIdentities,
			stateEntryFixture.CurrentEpochSetup.Participants,
			stateEntryFixture.CurrentEpoch.ActiveIdentities,
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)
		assert.Equal(t, expectedIdentities, richStateEntry.NextEpochIdentityTable, "should be equal to next epoch setup participants + current epoch setup participants")
	})
}

// TestProtocolStateEntry_Copy tests if the copy method returns a deep copy of the entry.
// All changes to copy shouldn't affect the original entry -- except for key changes.
func TestProtocolStateEntry_Copy(t *testing.T) {
	entry := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState()).EpochMinStateEntry
	cpy := entry.Copy()
	assert.Equal(t, entry, cpy)
	assert.NotSame(t, entry.NextEpoch, cpy.NextEpoch)
	assert.NotSame(t, entry.PreviousEpoch, cpy.PreviousEpoch)
	assert.NotSame(t, entry.CurrentEpoch, cpy.CurrentEpoch)

	cpy.EpochFallbackTriggered = !entry.EpochFallbackTriggered
	assert.NotEqual(t, entry, cpy)

	assert.Equal(t, entry.CurrentEpoch.ActiveIdentities[0], cpy.CurrentEpoch.ActiveIdentities[0])
	cpy.CurrentEpoch.ActiveIdentities[0].Ejected = true
	assert.NotEqual(t, entry.CurrentEpoch.ActiveIdentities[0], cpy.CurrentEpoch.ActiveIdentities[0])

	cpy.CurrentEpoch.ActiveIdentities = append(cpy.CurrentEpoch.ActiveIdentities, &flow.DynamicIdentityEntry{
		NodeID:  unittest.IdentifierFixture(),
		Ejected: false,
	})
	assert.NotEqual(t, entry.CurrentEpoch.ActiveIdentities, cpy.CurrentEpoch.ActiveIdentities)
}

// TestBuildIdentityTable tests if BuildIdentityTable returns a correct identity, whenever we pass arguments with or without
// overlap. It also tests if the function returns an error when the arguments are not ordered in the same order.
func TestBuildIdentityTable(t *testing.T) {
	t.Run("invalid-adjacent-identity-status", func(t *testing.T) {
		targetEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		adjacentEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])

		// Per convention, BuildIdentityTable only accepts EpochParticipationStatusLeaving or EpochParticipationStatusJoining
		// for the *adjacent* epoch, because these are the only sensible values.
		for _, status := range []flow.EpochParticipationStatus{flow.EpochParticipationStatusActive, flow.EpochParticipationStatusEjected} {
			identityList, err := flow.BuildIdentityTable(
				targetEpochIdentities.ToSkeleton(),
				flow.DynamicIdentityEntryListFromIdentities(targetEpochIdentities),
				adjacentEpochIdentities.ToSkeleton(),
				flow.DynamicIdentityEntryListFromIdentities(adjacentEpochIdentities),
				status,
			)
			assert.Error(t, err)
			assert.Empty(t, identityList)
		}
	})
	t.Run("happy-path-no-identities-overlap", func(t *testing.T) {
		targetEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		adjacentEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])

		identityList, err := flow.BuildIdentityTable(
			targetEpochIdentities.ToSkeleton(),
			flow.DynamicIdentityEntryListFromIdentities(targetEpochIdentities),
			adjacentEpochIdentities.ToSkeleton(),
			flow.DynamicIdentityEntryListFromIdentities(adjacentEpochIdentities),
			flow.EpochParticipationStatusLeaving,
		)
		assert.NoError(t, err)

		expectedIdentities := targetEpochIdentities.Union(adjacentEpochIdentities.Map(func(identity flow.Identity) flow.Identity {
			identity.EpochParticipationStatus = flow.EpochParticipationStatusLeaving
			return identity
		}))
		assert.Equal(t, expectedIdentities, identityList)
	})
	t.Run("happy-path-identities-overlap", func(t *testing.T) {
		targetEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		adjacentEpochIdentities := unittest.IdentityListFixture(10)
		sampledIdentities, err := targetEpochIdentities.Sample(2)
		// change address so we can assert that we take identities from target epoch and not adjacent epoch
		for i, identity := range sampledIdentities.Copy() {
			identity.Address = fmt.Sprintf("%d", i)
			adjacentEpochIdentities = append(adjacentEpochIdentities, identity)
		}
		assert.NoError(t, err)
		adjacentEpochIdentities = adjacentEpochIdentities.Sort(flow.Canonical[flow.Identity])

		identityList, err := flow.BuildIdentityTable(
			targetEpochIdentities.ToSkeleton(),
			flow.DynamicIdentityEntryListFromIdentities(targetEpochIdentities),
			adjacentEpochIdentities.ToSkeleton(),
			flow.DynamicIdentityEntryListFromIdentities(adjacentEpochIdentities),
			flow.EpochParticipationStatusJoining,
		)
		assert.NoError(t, err)

		expectedIdentities := targetEpochIdentities.Union(adjacentEpochIdentities.Map(func(identity flow.Identity) flow.Identity {
			identity.EpochParticipationStatus = flow.EpochParticipationStatusJoining
			return identity
		}))
		assert.Equal(t, expectedIdentities, identityList)
	})
	t.Run("target-epoch-identities-not-ordered", func(t *testing.T) {
		targetEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		targetEpochIdentitySkeletons, err := targetEpochIdentities.ToSkeleton().Shuffle()
		assert.NoError(t, err)
		targetEpochDynamicIdentities := flow.DynamicIdentityEntryListFromIdentities(targetEpochIdentities)

		adjacentEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		identityList, err := flow.BuildIdentityTable(
			targetEpochIdentitySkeletons,
			targetEpochDynamicIdentities,
			adjacentEpochIdentities.ToSkeleton(),
			flow.DynamicIdentityEntryListFromIdentities(adjacentEpochIdentities),
			flow.EpochParticipationStatusLeaving,
		)
		assert.Error(t, err)
		assert.Empty(t, identityList)
	})
	t.Run("adjacent-epoch-identities-not-ordered", func(t *testing.T) {
		adjacentEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		adjacentEpochIdentitySkeletons, err := adjacentEpochIdentities.ToSkeleton().Shuffle()
		assert.NoError(t, err)
		adjacentEpochDynamicIdentities := flow.DynamicIdentityEntryListFromIdentities(adjacentEpochIdentities)

		targetEpochIdentities := unittest.IdentityListFixture(10).Sort(flow.Canonical[flow.Identity])
		identityList, err := flow.BuildIdentityTable(
			targetEpochIdentities.ToSkeleton(),
			flow.DynamicIdentityEntryListFromIdentities(targetEpochIdentities),
			adjacentEpochIdentitySkeletons,
			adjacentEpochDynamicIdentities,
			flow.EpochParticipationStatusLeaving,
		)
		assert.Error(t, err)
		assert.Empty(t, identityList)
	})
}
