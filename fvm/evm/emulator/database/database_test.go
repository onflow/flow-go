package database_test

import (
	"fmt"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestDatabase(t *testing.T) {

	key1 := []byte("ABC")
	key2 := []byte("DEF")
	value1 := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	value2 := []byte{9, 10, 11}

	t.Run("test basic database functionality", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend types.Backend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
				db, err := database.NewDatabase(backend, flowEVMRoot)
				require.NoError(t, err)

				err = db.Put(key1, value1)
				require.NoError(t, err)

				err = db.Commit(gethTypes.EmptyRootHash)
				require.NoError(t, err)

				newdb, err := database.NewDatabase(backend, flowEVMRoot)
				require.NoError(t, err)

				has, err := newdb.Has(key1)
				require.NoError(t, err)
				require.True(t, has)

				retValue, err := newdb.Get(key1)
				require.NoError(t, err)

				require.Equal(t, value1, retValue)

				err = newdb.Delete(key1)
				require.NoError(t, err)

				has, err = newdb.Has(key1)
				require.NoError(t, err)
				require.False(t, has)

				h, err := newdb.GetRootHash()
				require.NoError(t, err)
				require.Equal(t, gethTypes.EmptyRootHash, h)

				newRoot := gethCommon.Hash{1, 2, 3}
				err = newdb.Commit(newRoot)
				require.NoError(t, err)

				h, err = newdb.GetRootHash()
				require.NoError(t, err)
				require.Equal(t, newRoot, h)
			})
		})
	})

	t.Run("test batch functionality", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend types.Backend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
				db, err := database.NewDatabase(backend, flowEVMRoot)
				require.NoError(t, err)

				err = db.Put(key1, value1)
				require.NoError(t, err)

				has, err := db.Has(key1)
				require.NoError(t, err)
				require.True(t, has)

				batch := db.NewBatch()
				err = batch.Delete(key1)
				require.NoError(t, err)

				err = batch.Put(key2, value2)
				require.NoError(t, err)

				has, err = db.Has(key2)
				require.NoError(t, err)
				require.False(t, has)

				err = batch.Write()
				require.NoError(t, err)

				retVal, err := db.Get(key2)
				require.NoError(t, err)
				require.Equal(t, value2, retVal)

				has, err = db.Has(key1)
				require.NoError(t, err)
				require.False(t, has)
			})
		})
	})

	t.Run("test non fatal error", func(t *testing.T) {
		ledger := &testutils.TestValueStore{
			GetValueFunc: func(_, _ []byte) ([]byte, error) {
				return nil, errors.NewLedgerInteractionLimitExceededError(0, 0)
			},
			SetValueFunc: func(owner, key, value []byte) error {
				return nil
			},
		}
		testutils.RunWithTestFlowEVMRootAddress(t, ledger, func(flowEVMRoot flow.Address) {
			_, err := database.NewDatabase(ledger, flowEVMRoot)
			require.Error(t, err)
			require.True(t, types.IsADatabaseError(err))
		})
	})

	t.Run("test fatal error (get value)", func(t *testing.T) {
		ledger := &testutils.TestValueStore{
			GetValueFunc: func(_, _ []byte) ([]byte, error) {
				return nil, fmt.Errorf("a fatal error")
			},
			SetValueFunc: func(owner, key, value []byte) error {
				return nil
			},
		}
		testutils.RunWithTestFlowEVMRootAddress(t, ledger, func(flowEVMRoot flow.Address) {
			_, err := database.NewDatabase(ledger, flowEVMRoot)
			require.Error(t, err)
			require.True(t, types.IsAFatalError(err))
		})
	})

	t.Run("test fatal error (storage id allocation)", func(t *testing.T) {
		ledger := &testutils.TestValueStore{
			AllocateStorageIndexFunc: func(_ []byte) (atree.StorageIndex, error) {
				return atree.StorageIndex{}, fmt.Errorf("a fatal error")
			},
			GetValueFunc: func(_, _ []byte) ([]byte, error) {
				return nil, nil
			},
			SetValueFunc: func(owner, key, value []byte) error {
				return nil
			},
		}
		testutils.RunWithTestFlowEVMRootAddress(t, ledger, func(flowEVMRoot flow.Address) {
			_, err := database.NewDatabase(ledger, flowEVMRoot)
			require.Error(t, err)
			require.True(t, types.IsAFatalError(err))
		})
	})

	t.Run("test fatal error (not implemented methods)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend types.Backend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
				db, err := database.NewDatabase(backend, flowEVMRoot)
				require.NoError(t, err)

				_, err = db.Stat("")
				require.Error(t, err)
				require.True(t, types.IsAFatalError(err))

				_, err = db.NewSnapshot()
				require.Error(t, err)
				require.True(t, types.IsAFatalError(err))
			})
		})
	})
}
