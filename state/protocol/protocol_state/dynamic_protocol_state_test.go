package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"testing"
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
}
