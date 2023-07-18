package protocol_state

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestDynamicProtocolStateAdapter tests if the dynamicProtocolStateAdapter returns expected values when created
// using constructor passing a RichProtocolStateEntry.
func TestDynamicProtocolStateAdapter(t *testing.T) {
	// construct a valid protocol state entry that has semantically correct DKGParticipantKeys
	entry := unittest.ProtocolStateFixture(func(entry *flow.RichProtocolStateEntry) {
		commit := entry.CurrentEpochCommit
		dkgParticipants := entry.CurrentEpochSetup.Participants.Filter(filter.IsValidDKGParticipant)
		lookup := unittest.DKGParticipantLookup(dkgParticipants)
		commit.DKGParticipantKeys = make([]crypto.PublicKey, len(lookup))
		for _, participant := range lookup {
			commit.DKGParticipantKeys[participant.Index] = participant.KeyShare
		}
	})

	adapter, err := newDynamicProtocolStateAdapter(entry)
	require.NoError(t, err)

	t.Run("identities", func(t *testing.T) {
		assert.Equal(t, entry.Identities, adapter.Identities())
	})
	t.Run("global-params", func(t *testing.T) {

	})
}
