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
	t.Run("retrieve state for existing blocks", func(t *testing.T) {
		dynamicProtocolState, err := protocolState.AtBlockID(blockID)
		require.NoError(t, err)

		assert.Equal(t, entry.CurrentEpochIdentityTable, dynamicProtocolState.Identities())

		other, err := protocolState.AtBlockID(otherBlockID)
		require.NoError(t, err)
		require.NotEqual(t, dynamicProtocolState.Identities(), other.Identities())
	})
	t.Run("retrieve state for non-existing block yields storage.ErrNotFound error", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()
		protocolStateDB.On("ByBlockID", blockID).Return(nil, storage.ErrNotFound).Once()
		_, err := protocolState.AtBlockID(blockID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
	t.Run("exception during retrieve is propagated", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()
		exception := errors.New("exception")
		protocolStateDB.On("ByBlockID", blockID).Return(nil, exception).Once()
		_, err := protocolState.AtBlockID(blockID)
		require.ErrorIs(t, err, exception)
	})
	t.Run("retrieve global-params", func(t *testing.T) {
		expectedChainID := flow.Testnet
		globalParams.On("ChainID").Return(expectedChainID, nil).Once()
		actualChainID := protocolState.GlobalParams().ChainID()
		assert.Equal(t, expectedChainID, actualChainID)
	})
}

// TestMutableProtocolState_Mutator tests happy path of creating a state mutator, and that `Mutator` returns an error
// if the parent protocol state has not been found.
func TestMutableProtocolState_Mutator(t *testing.T) {
	protocolStateDB := storagemock.NewProtocolState(t)
	globalParams := mock.NewGlobalParams(t)
	globalParams.On("EpochCommitSafetyThreshold").Return(uint64(1000))
	headersDB := storagemock.NewHeaders(t)
	resultsDB := storagemock.NewExecutionResults(t)
	setupsDB := storagemock.NewEpochSetups(t)
	commitsDB := storagemock.NewEpochCommits(t)

	mutableState := NewMutableProtocolState(
		protocolStateDB,
		globalParams,
		headersDB,
		resultsDB,
		setupsDB,
		commitsDB)

	t.Run("happy-path", func(t *testing.T) {
		parentState := unittest.ProtocolStateFixture()
		candidate := unittest.BlockHeaderFixture()
		protocolStateDB.On("ByBlockID", candidate.ParentID).Return(parentState, nil)
		mutator, err := mutableState.Mutator(candidate.View, candidate.ParentID)
		require.NoError(t, err)
		require.NotNil(t, mutator)
	})
	t.Run("parent-not-found", func(t *testing.T) {
		candidate := unittest.BlockHeaderFixture()
		protocolStateDB.On("ByBlockID", candidate.ParentID).Return(nil, storage.ErrNotFound)
		mutator, err := mutableState.Mutator(candidate.View, candidate.ParentID)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, mutator)
	})
	t.Run("invalid-state-transition-triggered", func(t *testing.T) {
		parentState := unittest.ProtocolStateFixture()
		parentState.InvalidEpochTransitionAttempted = true
		candidate := unittest.BlockHeaderFixture()
		protocolStateDB.On("ByBlockID", candidate.ParentID).Return(parentState, nil)
		mutator, err := mutableState.Mutator(candidate.View, candidate.ParentID)
		require.NoError(t, err)
		require.NotNil(t, mutator)
	})
}
