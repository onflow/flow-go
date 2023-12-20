package state_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

var rootAddr = flow.Address{1, 2, 3, 4, 5, 6, 7, 8}

func TestStateDB(t *testing.T) {
	t.Parallel()

	t.Run("test commit functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)
		key1 := testutils.RandomCommonHash(t)
		value1 := testutils.RandomCommonHash(t)

		db.CreateAccount(addr1)
		require.NoError(t, db.Error())

		db.AddBalance(addr1, big.NewInt(5))
		require.NoError(t, db.Error())

		// should have code to be able to set state
		db.SetCode(addr1, []byte{1, 2, 3})
		require.NoError(t, db.Error())

		db.SetState(addr1, key1, value1)

		err = db.Commit()
		require.NoError(t, err)

		// create a new db
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		bal := db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, big.NewInt(5), bal)

		val := db.GetState(addr1, key1)
		require.NoError(t, db.Error())
		require.Equal(t, value1, val)
	})

	t.Run("test snapshot and revert functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)
		require.False(t, db.Exist(addr1))
		require.NoError(t, db.Error())

		snapshot1 := db.Snapshot()
		require.Equal(t, 1, snapshot1)

		db.CreateAccount(addr1)
		require.NoError(t, db.Error())

		require.True(t, db.Exist(addr1))
		require.NoError(t, db.Error())

		db.AddBalance(addr1, big.NewInt(5))
		require.NoError(t, db.Error())

		bal := db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, big.NewInt(5), bal)

		snapshot2 := db.Snapshot()
		require.Equal(t, 2, snapshot2)

		db.AddBalance(addr1, big.NewInt(5))
		require.NoError(t, db.Error())

		bal = db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, big.NewInt(10), bal)

		// revert to snapshot 2
		db.RevertToSnapshot(snapshot2)
		require.NoError(t, db.Error())

		bal = db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, big.NewInt(5), bal)

		// revert to snapshot 1
		db.RevertToSnapshot(snapshot1)
		require.NoError(t, db.Error())

		bal = db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, big.NewInt(0), bal)

		// revert to an invalid snapshot
		db.RevertToSnapshot(10)
		require.Error(t, db.Error())
	})

	t.Run("test log functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		logs := []*gethTypes.Log{
			testutils.GetRandomLogFixture(t),
			testutils.GetRandomLogFixture(t),
			testutils.GetRandomLogFixture(t),
			testutils.GetRandomLogFixture(t),
		}

		db.AddLog(logs[0])
		db.AddLog(logs[1])

		_ = db.Snapshot()

		db.AddLog(logs[2])
		db.AddLog(logs[3])

		snapshot := db.Snapshot()
		db.AddLog(testutils.GetRandomLogFixture(t))
		db.RevertToSnapshot(snapshot)

		ret := db.Logs(common.Hash{}, 1, common.Hash{}, 1)
		require.Equal(t, ret, logs)
	})

	t.Run("test refund functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		require.Equal(t, uint64(0), db.GetRefund())
		db.AddRefund(10)
		require.Equal(t, uint64(10), db.GetRefund())
		db.SubRefund(3)
		require.Equal(t, uint64(7), db.GetRefund())

		snap1 := db.Snapshot()
		db.AddRefund(10)
		require.Equal(t, uint64(17), db.GetRefund())

		db.RevertToSnapshot(snap1)
		require.Equal(t, uint64(7), db.GetRefund())
	})

	t.Run("test non-fatal error handling", func(t *testing.T) {
		ledger := &testutils.TestValueStore{
			GetValueFunc: func(owner, key []byte) ([]byte, error) {
				return nil, nil
			},
			SetValueFunc: func(owner, key, value []byte) error {
				return atree.NewUserError(fmt.Errorf("key not found"))
			},
			AllocateStorageIndexFunc: func(owner []byte) (atree.StorageIndex, error) {
				return atree.StorageIndex{}, nil
			},
		}
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		db.CreateAccount(testutils.RandomCommonAddress(t))

		err = db.Commit()
		// ret := db.Error()
		require.Error(t, err)
		// check wrapping
		require.True(t, types.IsAStateError(err))
	})

	t.Run("test fatal error handling", func(t *testing.T) {
		ledger := &testutils.TestValueStore{
			GetValueFunc: func(owner, key []byte) ([]byte, error) {
				return nil, nil
			},
			SetValueFunc: func(owner, key, value []byte) error {
				return atree.NewFatalError(fmt.Errorf("key not found"))
			},
			AllocateStorageIndexFunc: func(owner []byte) (atree.StorageIndex, error) {
				return atree.StorageIndex{}, nil
			},
		}
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		db.CreateAccount(testutils.RandomCommonAddress(t))

		err = db.Commit()
		// ret := db.Error()
		require.Error(t, err)
		// check wrapping
		require.True(t, types.IsAFatalError(err))
	})

}
