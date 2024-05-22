package state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	psmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_ProtocolState verifies the different scenarios of retrieving a protocol state, global parameters
// and KV store snapshots by block ID for the `ProtocolState`. Happy and unhappy paths are covered.
func Test_ProtocolState(t *testing.T) {
	epochProtocolStateDB := storagemock.NewProtocolState(t)
	protocolKVStoreDB := storagemock.NewProtocolKVStore(t)
	globalParams := psmock.NewGlobalParams(t)
	protocolState := NewProtocolState(epochProtocolStateDB, protocolKVStoreDB, globalParams)

	t.Run("testing `ProtocolState.AtBlockID`", func(t *testing.T) {
		test_AtBlockID(t, protocolState, epochProtocolStateDB)
	})
	t.Run("testing `ProtocolState.GlobalParams`", func(t *testing.T) {
		test_GlobalParams(t, protocolState, globalParams)
	})
	t.Run("testing `ProtocolState.KVStoreAtBlockID`", func(t *testing.T) {
		test_KVStoreAtBlockID(t, protocolState, protocolKVStoreDB)
	})
}

// Test_MutableProtocolState verifies the different scenarios of retrieving a protocol state, global parameters
// and KV store snapshots by block ID for the `MutableProtocolState`. Happy and unhappy paths are covered.
func Test_MutableProtocolState(t *testing.T) {
	epochProtocolStateDB := storagemock.NewProtocolState(t)
	protocolKVStoreDB := storagemock.NewProtocolKVStore(t)
	globalParams := psmock.NewGlobalParams(t)
	headersDB := storagemock.NewHeaders(t)
	resultsDB := storagemock.NewExecutionResults(t)
	setupsDB := storagemock.NewEpochSetups(t)
	commitsDB := storagemock.NewEpochCommits(t)

	mutableProtocolState := NewMutableProtocolState(
		epochProtocolStateDB,
		protocolKVStoreDB,
		globalParams,
		headersDB,
		resultsDB,
		setupsDB,
		commitsDB)

	t.Run("testing `MutableProtocolState.AtBlockID`", func(t *testing.T) {
		test_AtBlockID(t, mutableProtocolState, epochProtocolStateDB)
	})
	t.Run("testing `MutableProtocolState.GlobalParams`", func(t *testing.T) {
		test_GlobalParams(t, mutableProtocolState, globalParams)
	})
	t.Run("testing `MutableProtocolState.KVStoreAtBlockID`", func(t *testing.T) {
		test_KVStoreAtBlockID(t, mutableProtocolState, protocolKVStoreDB)
	})
}

func test_AtBlockID(t *testing.T, protocolState protocol.ProtocolState, epochProtocolStateDB *storagemock.ProtocolState) {
	blockID := unittest.IdentifierFixture()

	t.Run("retrieve epoch state for existing blocks", func(t *testing.T) {
		epochState := unittest.EpochStateFixture(unittest.WithValidDKG())
		epochProtocolStateDB.On("ByBlockID", blockID).Return(epochState, nil).Once()

		epochProtocolState, err := protocolState.AtBlockID(blockID)
		require.NoError(t, err)
		assert.Equal(t, epochState.CurrentEpochIdentityTable, epochProtocolState.Identities())
	})
	t.Run("retrieving epoch state for non-existing block yields storage.ErrNotFound error", func(t *testing.T) {
		epochProtocolStateDB.On("ByBlockID", blockID).Return(nil, storage.ErrNotFound).Once()
		_, err := protocolState.AtBlockID(blockID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
	t.Run("exception during retrieve is propagated", func(t *testing.T) {
		exception := errors.New("exception")
		epochProtocolStateDB.On("ByBlockID", blockID).Return(nil, exception).Once()
		_, err := protocolState.AtBlockID(blockID)
		require.ErrorIs(t, err, exception)
	})
}

func test_GlobalParams(t *testing.T, protocolState protocol.ProtocolState, globalParams *psmock.GlobalParams) {
	expectedChainID := flow.Testnet
	globalParams.On("ChainID").Return(expectedChainID, nil).Once()
	actualChainID := protocolState.GlobalParams().ChainID()
	assert.Equal(t, expectedChainID, actualChainID)
}

func test_KVStoreAtBlockID(t *testing.T, protocolState protocol.ProtocolState, protocolKVStoreDB *storagemock.ProtocolKVStore) {
	blockID := unittest.IdentifierFixture()
	expectedState := &kvstore.Modelv1{
		Modelv0: kvstore.Modelv0{
			UpgradableModel: kvstore.UpgradableModel{},
			EpochStateID:    unittest.IdentifierFixture(),
		},
	}
	version, encStateData, err := expectedState.VersionedEncode()
	require.NoError(t, err)
	encExpectedState := &flow.PSKeyValueStoreData{
		Version: version,
		Data:    encStateData,
	}

	t.Run("retrieve KVStoreReader", func(t *testing.T) {
		protocolKVStoreDB.On("ByBlockID", blockID).Return(encExpectedState, nil).Once()
		state, err := protocolState.KVStoreAtBlockID(blockID)
		assert.NoError(t, err)
		assert.Equal(t, expectedState, state)
	})

	t.Run("error retrieving KVStoreReader", func(t *testing.T) {
		exception := errors.New("exception")
		protocolKVStoreDB.On("ByBlockID", blockID).Return(nil, exception).Once()
		_, err := protocolState.KVStoreAtBlockID(blockID)
		assert.ErrorIs(t, err, exception)
	})
}
