package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewRichProtocolStateEntry checks that NewRichProtocolStateEntry creates valid identity tables depending on the state
// of epoch which is derived from the protocol state entry.
func TestNewRichProtocolStateEntry(t *testing.T) {
	// Conditions right after a spork:
	//  * no previous epoch exists from the perspective of the freshly-sporked protocol state
	//  * network is currently in the staking phase for the next epoch, hence no service events for the next epoch exist
	t.Run("staking-root-protocol-state", func(t *testing.T) {
		currentEpochSetup := unittest.EpochSetupFixture()
		currentEpochCommit := unittest.EpochCommitFixture()
		stateEntry := &flow.ProtocolStateEntry{
			CurrentEpoch: flow.EpochStateContainer{
				SetupID:    currentEpochSetup.ID(),
				CommitID:   currentEpochCommit.ID(),
				Identities: flow.DynamicIdentityEntryListFromIdentities(currentEpochSetup.Participants),
			},
			PreviousEpochEventIDs:           flow.EventIDs{},
			InvalidStateTransitionAttempted: false,
		}
		entry, err := flow.NewRichProtocolStateEntry(
			stateEntry,
			nil,
			nil,
			currentEpochSetup,
			currentEpochCommit,
			nil,
			nil,
		)
		assert.NoError(t, err)
		assert.Equal(t, currentEpochSetup.Participants, entry.Identities, "should be equal to current epoch setup participants")
	})

	// Common situation during the staking phase for epoch N+1
	//  * we are currently in Epoch N
	//  * previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
	//  * network is currently in the staking phase for the next epoch, hence no service events for the next epoch exist
	t.Run("staking-phase", func(t *testing.T) {
		stateEntry := unittest.ProtocolStateFixture()
		richEntry, err := flow.NewRichProtocolStateEntry(
			stateEntry.ProtocolStateEntry,
			stateEntry.PreviousEpochSetup,
			stateEntry.PreviousEpochCommit,
			stateEntry.CurrentEpochSetup,
			stateEntry.CurrentEpochCommit,
			nil,
			nil,
		)
		assert.NoError(t, err)
		expectedIdentities := stateEntry.CurrentEpochSetup.Participants.Union(stateEntry.PreviousEpochSetup.Participants)
		assert.Equal(t, expectedIdentities, richEntry.Identities, "should be equal to current epoch setup participants + previous epoch setup participants")
		assert.Nil(t, richEntry.NextEpoch)
	})

	// Common situation during the epoch setup phase for epoch N+1
	//  * we are currently in Epoch N
	//  * previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
	//  * network is currently in the setup phase for the next epoch, i.e. EpochSetup event (starting setup phase) has already been observed
	t.Run("setup-phase", func(t *testing.T) {
		stateEntry := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichProtocolStateEntry) {
			entry.NextEpochCommit = nil
			entry.NextEpoch.CommitID = flow.ZeroID
		})

		richEntry, err := flow.NewRichProtocolStateEntry(
			stateEntry.ProtocolStateEntry,
			stateEntry.PreviousEpochSetup,
			stateEntry.PreviousEpochCommit,
			stateEntry.CurrentEpochSetup,
			stateEntry.CurrentEpochCommit,
			stateEntry.NextEpochSetup,
			nil,
		)
		assert.NoError(t, err)
		expectedIdentities := stateEntry.CurrentEpochSetup.Participants.Union(stateEntry.NextEpochSetup.Participants)
		assert.Equal(t, expectedIdentities, richEntry.Identities, "should be equal to current epoch setup participants + next epoch setup participants")
		assert.Nil(t, richEntry.NextEpochCommit)
		expectedIdentities = stateEntry.NextEpochSetup.Participants.Union(stateEntry.CurrentEpochSetup.Participants)
		assert.Equal(t, expectedIdentities, richEntry.NextIdentities, "should be equal to next epoch setup participants + current epoch setup participants")
	})

	// TODO: include test for epoch setup phase where no prior epoch exist (i.e. first epoch setup phase after spork)

	// Common situation during the epoch commit phase for epoch N+1
	//  * we are currently in Epoch N
	//  * previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
	//  * The network has completed the epoch setup phase, i.e. published the EpochSetup and EpochCommit events for epoch N+1.
	t.Run("commit-phase", func(t *testing.T) {
		stateEntry := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())

		richEntry, err := flow.NewRichProtocolStateEntry(
			stateEntry.ProtocolStateEntry,
			stateEntry.PreviousEpochSetup,
			stateEntry.PreviousEpochCommit,
			stateEntry.CurrentEpochSetup,
			stateEntry.CurrentEpochCommit,
			stateEntry.NextEpochSetup,
			stateEntry.NextEpochCommit,
		)
		assert.NoError(t, err)
		expectedIdentities := stateEntry.CurrentEpochSetup.Participants.Union(stateEntry.NextEpochSetup.Participants)
		assert.Equal(t, expectedIdentities, richEntry.Identities, "should be equal to current epoch setup participants + next epoch setup participants")
		expectedIdentities = stateEntry.NextEpochSetup.Participants.Union(stateEntry.CurrentEpochSetup.Participants)
		assert.Equal(t, expectedIdentities, richEntry.NextIdentities, "should be equal to next epoch setup participants + current epoch setup participants")
	})

	// TODO: include test for epoch commit phase where no prior epoch exist (i.e. first epoch commit phase after spork)

}

// TestProtocolStateEntry_Copy tests if the copy method returns a deep copy of the entry.
// All changes to copy shouldn't affect the original entry -- except for key changes.
func TestProtocolStateEntry_Copy(t *testing.T) {
	entry := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState()).ProtocolStateEntry
	cpy := entry.Copy()
	assert.Equal(t, entry, cpy)
	assert.NotSame(t, entry.NextEpoch, cpy.NextEpoch)
	assert.NotSame(t, entry.PreviousEpochEventIDs, cpy.PreviousEpochEventIDs)
	assert.NotSame(t, entry.CurrentEpoch, cpy.CurrentEpoch)

	cpy.InvalidStateTransitionAttempted = !entry.InvalidStateTransitionAttempted
	assert.NotEqual(t, entry, cpy)

	assert.Equal(t, entry.CurrentEpoch.Identities[0], cpy.CurrentEpoch.Identities[0])
	cpy.CurrentEpoch.Identities[0].Dynamic.Weight = 123
	assert.NotEqual(t, entry.CurrentEpoch.Identities[0], cpy.CurrentEpoch.Identities[0])

	cpy.CurrentEpoch.Identities = append(cpy.CurrentEpoch.Identities, &flow.DynamicIdentityEntry{
		NodeID: unittest.IdentifierFixture(),
		Dynamic: flow.DynamicIdentity{
			Weight:  100,
			Ejected: false,
		},
	})
	assert.NotEqual(t, entry.CurrentEpoch.Identities, cpy.CurrentEpoch.Identities)
}
