package protocol_state

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDynamicProtocolStateAdapter tests if the dynamicProtocolStateAdapter returns expected values when created
// using constructor passing a RichProtocolStateEntry.
func TestDynamicProtocolStateAdapter(t *testing.T) {
	// construct a valid protocol state entry that has semantically correct DKGParticipantKeys
	entry := unittest.ProtocolStateFixture(WithValidDKG())

	globalParams := mock.NewGlobalParams(t)
	adapter := newDynamicProtocolStateAdapter(entry, globalParams)

	t.Run("identities", func(t *testing.T) {
		assert.Equal(t, entry.Identities, adapter.Identities())
	})
	t.Run("global-params", func(t *testing.T) {
		expectedChainID := flow.Testnet
		globalParams.On("ChainID").Return(expectedChainID, nil).Once()
		actualChainID := adapter.GlobalParams().ChainID()
		assert.Equal(t, expectedChainID, actualChainID)
	})
	t.Run("epoch-status-staking", func(t *testing.T) {
		entry := unittest.ProtocolStateFixture()
		adapter := newDynamicProtocolStateAdapter(entry, globalParams)
		status := adapter.EpochStatus()
		assert.Equal(t, entry.PreviousEpochEventIDs, status.PreviousEpoch)
		assert.Equal(t, entry.CurrentEpochEventIDs, status.CurrentEpoch)
		assert.Equal(t, flow.EventIDs{}, status.NextEpoch)
		assert.False(t, status.InvalidServiceEventIncorporated)
	})
	t.Run("epoch-status-setup", func(t *testing.T) {
		entry := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		// cleanup the commit event, so we are in setup phase
		entry.NextEpochProtocolState.CurrentEpochEventIDs.CommitID = flow.ZeroID

		adapter := newDynamicProtocolStateAdapter(entry, globalParams)
		status := adapter.EpochStatus()
		assert.Equal(t, entry.PreviousEpochEventIDs, status.PreviousEpoch)
		assert.Equal(t, entry.CurrentEpochEventIDs, status.CurrentEpoch)
		assert.Equal(t, flow.EventIDs{SetupID: entry.NextEpochProtocolState.CurrentEpochSetup.ID()}, status.NextEpoch)
		assert.False(t, status.InvalidServiceEventIncorporated)
	})
	t.Run("epoch-status-commit", func(t *testing.T) {
		entry := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		adapter := newDynamicProtocolStateAdapter(entry, globalParams)
		status := adapter.EpochStatus()
		assert.Equal(t, entry.PreviousEpochEventIDs, status.PreviousEpoch)
		assert.Equal(t, entry.CurrentEpochEventIDs, status.CurrentEpoch)
		assert.Equal(t, entry.NextEpochProtocolState.CurrentEpochEventIDs, status.NextEpoch)
		assert.False(t, status.InvalidServiceEventIncorporated)
	})
	t.Run("invalid-state-transition-attempted", func(t *testing.T) {
		entry := unittest.ProtocolStateFixture(func(entry *flow.RichProtocolStateEntry) {
			entry.InvalidStateTransitionAttempted = true
		})
		adapter := newDynamicProtocolStateAdapter(entry, globalParams)
		status := adapter.EpochStatus()
		assert.True(t, status.InvalidServiceEventIncorporated)
	})
}
