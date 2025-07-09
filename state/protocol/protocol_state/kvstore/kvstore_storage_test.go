package kvstore_test

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	protocol_statemock "github.com/onflow/flow-go/state/protocol/protocol_state/mock"
	"github.com/onflow/flow-go/storage/badger/transaction"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProtocolKVStore_StoreTx verifies correct functioning of `ProtocolKVStore.StoreTx`. In a nutshell,
// `ProtocolKVStore` should encode the provided snapshot and call the lower-level storage abstraction
// to persist the encoded result.
func TestProtocolKVStore_StoreTx(t *testing.T) {
	llStorage := storagemock.NewProtocolKVStore(t) // low-level storage of versioned binary Protocol State snapshots
	kvState := protocol_statemock.NewKVStoreAPI(t) // instance of key-value store, which we want to persist
	kvStateID := unittest.IdentifierFixture()

	store := kvstore.NewProtocolKVStore(llStorage) // instance that we are testing

	// On the happy path, where the input `kvState` encodes its state successfully, the wrapped store
	// should be called to persist the version-encoded snapshot.
	t.Run("happy path", func(t *testing.T) {
		expectedVersion := uint64(13)
		encData := unittest.RandomBytes(117)
		versionedSnapshot := &flow.PSKeyValueStoreData{
			Version: expectedVersion,
			Data:    encData,
		}
		kvState.On("VersionedEncode").Return(expectedVersion, encData, nil).Once()

		deferredUpdate := storagemock.NewDeferredDBUpdate(t)
		deferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
		llStorage.On("StoreTx", kvStateID, versionedSnapshot).Return(deferredUpdate.Execute).Once()

		// Calling `StoreTx` should return the output of the wrapped low-level storage, which is a deferred database
		// update. Conceptually, it is possible that `ProtocolKVStore` wraps the deferred database operation in faulty
		// code, such that it cannot be executed. Therefore, we execute the top-level deferred database update below
		// and verify that the deferred database operation returned by the lower-level is actually reached.
		dbUpdate := store.StoreTx(kvStateID, kvState)
		err := dbUpdate(&transaction.Tx{})
		require.NoError(t, err)
	})

	// On the unhappy path, i.e. when the encoding of input `kvState` failed, `ProtocolKVStore` should produce
	// a deferred database update that always returns the encoding error.
	t.Run("encoding fails", func(t *testing.T) {
		encodingError := errors.New("encoding error")
		kvState.On("VersionedEncode").Return(uint64(0), nil, encodingError).Once()

		dbUpdate := store.StoreTx(kvStateID, kvState)
		err := dbUpdate(&transaction.Tx{})
		require.ErrorIs(t, err, encodingError)
	})
}

// TestProtocolKVStore_IndexTx verifies that `ProtocolKVStore.IndexTx` delegate all calls directly to the
// low-level storage abstraction.
func TestProtocolKVStore_IndexTx(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	stateID := unittest.IdentifierFixture()
	llStorage := storagemock.NewProtocolKVStore(t) // low-level storage of versioned binary Protocol State snapshots

	store := kvstore.NewProtocolKVStore(llStorage) // instance that we are testing

	// should be called to persist the version-encoded snapshot.
	t.Run("happy path", func(t *testing.T) {
		deferredUpdate := storagemock.NewDeferredDBUpdate(t)
		deferredUpdate.On("Execute", mock.Anything).Return(nil).Once()
		llStorage.On("IndexTx", blockID, stateID).Return(deferredUpdate.Execute).Once()

		// Calling `IndexTx` should return the output of the wrapped low-level storage, which is a deferred database
		// update. Conceptually, it is possible that `ProtocolKVStore` wraps the deferred database operation in faulty
		// code, such that it cannot be executed. Therefore, we execute the top-level deferred database update below
		// and verify that the deferred database operation returned by the lower-level is actually reached.
		dbUpdate := store.IndexTx(blockID, stateID)
		err := dbUpdate(&transaction.Tx{})
		require.NoError(t, err)
	})

	// On the unhappy path, the deferred database update from the lower level just errors upon execution.
	// This error should be escalated.
	t.Run("unhappy path", func(t *testing.T) {
		indexingError := errors.New("indexing error")
		deferredUpdate := storagemock.NewDeferredDBUpdate(t)
		deferredUpdate.On("Execute", mock.Anything).Return(indexingError).Once()
		llStorage.On("IndexTx", blockID, stateID).Return(deferredUpdate.Execute).Once()

		dbUpdate := store.IndexTx(blockID, stateID)
		err := dbUpdate(&transaction.Tx{})
		require.ErrorIs(t, err, indexingError)
	})
}

// TestProtocolKVStore_ByBlockID verifies correct functioning of `ProtocolKVStore.ByBlockID`. In a nutshell,
// `ProtocolKVStore` should attempt to retrieve the encoded snapshot from the lower-level storage abstraction
// and return the decoded result.
func TestProtocolKVStore_ByBlockID(t *testing.T) {
	blockID := unittest.IdentifierFixture()
	llStorage := storagemock.NewProtocolKVStore(t) // low-level storage of versioned binary Protocol State snapshots

	store := kvstore.NewProtocolKVStore(llStorage) // instance that we are testing

	// On the happy path, `ProtocolKVStore` should decode the snapshot retrieved by the lowe-level storage abstraction.
	// should be called to persist the version-encoded snapshot.
	t.Run("happy path", func(t *testing.T) {
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
		llStorage.On("ByBlockID", blockID).Return(encExpectedState, nil).Once()

		decodedState, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expectedState, decodedState)
	})

	// On the unhappy path, either `ProtocolKVStore.ByBlockID` could error, or the decoding could fail. In either case,
	// the error should be escalated to the caller.
	t.Run("low-level `ProtocolKVStore.ByBlockID` errors", func(t *testing.T) {
		someError := errors.New("some problem")
		llStorage.On("ByBlockID", blockID).Return(nil, someError).Once()

		_, err := store.ByBlockID(blockID)
		require.ErrorIs(t, err, someError)
	})
	t.Run("decoding fails with `ErrUnsupportedVersion`", func(t *testing.T) {
		versionedSnapshot := &flow.PSKeyValueStoreData{
			Version: math.MaxUint64,
			Data:    unittest.RandomBytes(117),
		}
		llStorage.On("ByBlockID", blockID).Return(versionedSnapshot, nil).Once()

		_, err := store.ByBlockID(blockID)
		require.ErrorIs(t, err, kvstore.ErrUnsupportedVersion)
	})
	t.Run("decoding yields exception", func(t *testing.T) {
		versionedSnapshot := &flow.PSKeyValueStoreData{
			Version: 1, // model version 1 is known, but data is random, which should yield an `irrecoverable.Exception`
			Data:    unittest.RandomBytes(117),
		}
		llStorage.On("ByBlockID", blockID).Return(versionedSnapshot, nil).Once()

		_, err := store.ByBlockID(blockID)
		require.NotErrorIs(t, err, kvstore.ErrUnsupportedVersion)
	})
}

// TestProtocolKVStore_ByID verifies correct functioning of `ProtocolKVStore.ByID`. In a nutshell,
// `ProtocolKVStore` should attempt to retrieve the encoded snapshot from the lower-level storage
// abstraction and return the decoded result.
func TestProtocolKVStore_ByID(t *testing.T) {
	protocolStateID := unittest.IdentifierFixture()
	llStorage := storagemock.NewProtocolKVStore(t) // low-level storage of versioned binary Protocol State snapshots

	store := kvstore.NewProtocolKVStore(llStorage) // instance that we are testing

	// On the happy path, `ProtocolKVStore` should decode the snapshot retrieved by the lowe-level storage abstraction.
	// should be called to persist the version-encoded snapshot.
	t.Run("happy path", func(t *testing.T) {
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
		llStorage.On("ByID", protocolStateID).Return(encExpectedState, nil).Once()

		decodedState, err := store.ByID(protocolStateID)
		require.NoError(t, err)
		require.Equal(t, expectedState, decodedState)
	})

	// On the unhappy path, either `ProtocolKVStore.ByID` could error, or the decoding could fail. In either case,
	// the error should be escalated to the caller.
	t.Run("low-level `ProtocolKVStore.ByID` errors", func(t *testing.T) {
		someError := errors.New("some problem")
		llStorage.On("ByID", protocolStateID).Return(nil, someError).Once()

		_, err := store.ByID(protocolStateID)
		require.ErrorIs(t, err, someError)
	})
	t.Run("decoding fails with `ErrUnsupportedVersion`", func(t *testing.T) {
		versionedSnapshot := &flow.PSKeyValueStoreData{
			Version: math.MaxUint64,
			Data:    unittest.RandomBytes(117),
		}
		llStorage.On("ByID", protocolStateID).Return(versionedSnapshot, nil).Once()

		_, err := store.ByID(protocolStateID)
		require.ErrorIs(t, err, kvstore.ErrUnsupportedVersion)
	})
	t.Run("decoding yields exception", func(t *testing.T) {
		versionedSnapshot := &flow.PSKeyValueStoreData{
			Version: 1, // model version 1 is known, but data is random, which should yield an `irrecoverable.Exception`
			Data:    unittest.RandomBytes(117),
		}
		llStorage.On("ByID", protocolStateID).Return(versionedSnapshot, nil).Once()

		_, err := store.ByID(protocolStateID)
		require.NotErrorIs(t, err, kvstore.ErrUnsupportedVersion)
	})
}
