package protocol_state

import (
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestDynamicProtocolStateAdapter tests if the dynamicProtocolStateAdapter returns expected values when created
// using constructor passing a RichProtocolStateEntry.
func TestDynamicProtocolStateAdapter(t *testing.T) {
	// construct a valid protocol state entry that has semantically correct DKGParticipantKeys
	entry := unittest.ProtocolStateFixture(WithValidDKG())

	adapter, err := newDynamicProtocolStateAdapter(entry)
	require.NoError(t, err)

	t.Run("identities", func(t *testing.T) {
		assert.Equal(t, entry.Identities, adapter.Identities())
	})
	t.Run("global-params", func(t *testing.T) {

	})
}
