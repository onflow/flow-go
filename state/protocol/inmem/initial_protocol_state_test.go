package inmem_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInitialProtocolStateAdapter tests if the InitialProtocolStateAdapter returns expected values when created
// using constructor passing a RichProtocolStateEntry.
func TestInitialProtocolStateAdapter(t *testing.T) {
	// construct a valid protocol state entry that has semantically correct DKGParticipantKeys
	entry := unittest.ProtocolStateFixture(unittest.WithValidDKG())

	adapter := inmem.NewInitialProtocolStateAdapter(entry)

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
}
