package protocol_state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProtocolState_AtBlockID
func TestProtocolState_AtBlockID(t *testing.T) {
	entry := unittest.ProtocolStateFixture(unittest.WithValidDKG())
	otherEntry := unittest.ProtocolStateFixture(unittest.WithValidDKG())
	blockID := unittest.IdentifierFixture()
	otherBlockID := unittest.IdentifierFixture()

	protocolStateDB := storagemock.NewProtocolState(t)
	protocolStateDB.On("ByBlockID", blockID).Return(entry, nil).Once()
	protocolStateDB.On("ByBlockID", otherBlockID).Return(otherEntry, nil).Once()

	globalParams := mock.NewGlobalParams(t)
	protocolState := NewProtocolState(protocolStateDB, globalParams)
	t.Run("exists", func(t *testing.T) {
		dynamicProtocolState, err := protocolState.AtBlockID(blockID)
		require.NoError(t, err)

		assert.Equal(t, entry.CurrentEpochIdentityTable, dynamicProtocolState.Identities())

		other, err := protocolState.AtBlockID(otherBlockID)
		require.NoError(t, err)
		require.NotEqual(t, dynamicProtocolState.Identities(), other.Identities())
	})
	t.Run("not-exists", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()
		protocolStateDB.On("ByBlockID", blockID).Return(nil, storage.ErrNotFound).Once()
		_, err := protocolState.AtBlockID(blockID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
	t.Run("exception", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()
		exception := errors.New("exception")
		protocolStateDB.On("ByBlockID", blockID).Return(nil, exception).Once()
		_, err := protocolState.AtBlockID(blockID)
		require.ErrorIs(t, err, exception)
	})
	t.Run("global-params", func(t *testing.T) {
		expectedChainID := flow.Testnet
		globalParams.On("ChainID").Return(expectedChainID, nil).Once()
		actualChainID := protocolState.GlobalParams().ChainID()
		assert.Equal(t, expectedChainID, actualChainID)
	})
}
