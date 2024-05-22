package inmem_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow/filter"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDynamicProtocolStateAdapter tests if the DynamicProtocolStateAdapter returns expected values when created
// using constructor passing a RichProtocolStateEntry.
func TestDynamicProtocolStateAdapter(t *testing.T) {
	// construct a valid protocol state entry that has semantically correct DKGParticipantKeys
	entry := unittest.EpochStateFixture(unittest.WithValidDKG())

	globalParams := mock.NewGlobalParams(t)
	adapter := inmem.NewDynamicProtocolStateAdapter(entry, globalParams)

	t.Run("clustering", func(t *testing.T) {
		clustering, err := inmem.ClusteringFromSetupEvent(entry.CurrentEpochSetup)
		require.NoError(t, err)
		actual, err := adapter.Clustering()
		require.NoError(t, err)
		assert.Equal(t, clustering, actual)
	})
	t.Run("epoch", func(t *testing.T) {
		assert.Equal(t, entry.CurrentEpochSetup.Counter, adapter.Epoch())
	})
	t.Run("setup", func(t *testing.T) {
		assert.Equal(t, entry.CurrentEpochSetup, adapter.EpochSetup())
	})
	t.Run("commit", func(t *testing.T) {
		assert.Equal(t, entry.CurrentEpochCommit, adapter.EpochCommit())
	})
	t.Run("dkg", func(t *testing.T) {
		dkg, err := adapter.DKG()
		require.NoError(t, err)
		assert.Equal(t, entry.CurrentEpochCommit.DKGGroupKey, dkg.GroupKey())
		assert.Equal(t, len(entry.CurrentEpochCommit.DKGParticipantKeys), int(dkg.Size()))
		dkgParticipants := entry.CurrentEpochSetup.Participants.Filter(filter.IsValidDKGParticipant)
		for _, identity := range dkgParticipants {
			keyShare, err := dkg.KeyShare(identity.NodeID)
			require.NoError(t, err)
			index, err := dkg.Index(identity.NodeID)
			require.NoError(t, err)
			assert.Equal(t, entry.CurrentEpochCommit.DKGParticipantKeys[index], keyShare)
		}
	})
	t.Run("entry", func(t *testing.T) {
		actualEntry := adapter.Entry()
		assert.Equal(t, entry, actualEntry, "entry should be equal to the one passed to the constructor")
		assert.NotSame(t, entry, actualEntry, "entry should be a copy of the one passed to the constructor")
	})
	t.Run("identities", func(t *testing.T) {
		assert.Equal(t, entry.CurrentEpochIdentityTable, adapter.Identities())
	})
	t.Run("global-params", func(t *testing.T) {
		expectedChainID := flow.Testnet
		globalParams.On("ChainID").Return(expectedChainID, nil).Once()
		actualChainID := adapter.GlobalParams().ChainID()
		assert.Equal(t, expectedChainID, actualChainID)
	})
	t.Run("epoch-phase-staking", func(t *testing.T) {
		entry := unittest.EpochStateFixture()
		adapter := inmem.NewDynamicProtocolStateAdapter(entry, globalParams)
		assert.Equal(t, flow.EpochPhaseStaking, adapter.EpochPhase())
		assert.True(t, adapter.PreviousEpochExists())
		assert.False(t, adapter.InvalidEpochTransitionAttempted())
	})
	t.Run("epoch-phase-setup", func(t *testing.T) {
		entry := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		// cleanup the commit event, so we are in setup phase
		entry.NextEpoch.CommitID = flow.ZeroID

		adapter := inmem.NewDynamicProtocolStateAdapter(entry, globalParams)
		assert.Equal(t, flow.EpochPhaseSetup, adapter.EpochPhase())
		assert.True(t, adapter.PreviousEpochExists())
		assert.False(t, adapter.InvalidEpochTransitionAttempted())
	})
	t.Run("epoch-phase-commit", func(t *testing.T) {
		entry := unittest.EpochStateFixture(unittest.WithNextEpochProtocolState())
		adapter := inmem.NewDynamicProtocolStateAdapter(entry, globalParams)
		assert.Equal(t, flow.EpochPhaseCommitted, adapter.EpochPhase())
		assert.True(t, adapter.PreviousEpochExists())
		assert.False(t, adapter.InvalidEpochTransitionAttempted())
	})
	t.Run("invalid-state-transition-attempted", func(t *testing.T) {
		entry := unittest.EpochStateFixture(func(entry *flow.RichProtocolStateEntry) {
			entry.InvalidEpochTransitionAttempted = true
		})
		adapter := inmem.NewDynamicProtocolStateAdapter(entry, globalParams)
		assert.True(t, adapter.InvalidEpochTransitionAttempted())
	})
	t.Run("no-previous-epoch", func(t *testing.T) {
		entry := unittest.EpochStateFixture(func(entry *flow.RichProtocolStateEntry) {
			entry.PreviousEpoch = nil
			entry.PreviousEpochSetup = nil
			entry.PreviousEpochCommit = nil
		})
		adapter := inmem.NewDynamicProtocolStateAdapter(entry, globalParams)
		assert.False(t, adapter.PreviousEpochExists())
	})
}
