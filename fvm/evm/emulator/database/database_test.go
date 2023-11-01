package database_test

import (
	"fmt"
	"testing"

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

				err = db.Commit()
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

	t.Run("test fatal error", func(t *testing.T) {
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
			require.False(t, types.IsAFatalError(err))
		})
	})

}
