package state_test

import (
	"fmt"
	"testing"

	"github.com/holiman/uint256"
	"github.com/onflow/atree"
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/core/tracing"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethParams "github.com/onflow/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var rootAddr = flow.Address{1, 2, 3, 4, 5, 6, 7, 8}

func TestStateDB(t *testing.T) {
	t.Parallel()

	t.Run("test Empty method", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)
		// non-existent account
		require.True(t, db.Empty(addr1))
		require.NoError(t, db.Error())

		db.CreateAccount(addr1)
		require.NoError(t, db.Error())

		require.True(t, db.Empty(addr1))
		require.NoError(t, db.Error())

		db.AddBalance(addr1, uint256.NewInt(10), tracing.BalanceChangeUnspecified)
		require.NoError(t, db.Error())

		require.False(t, db.Empty(addr1))
	})

	t.Run("test create contract method", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)
		require.False(t, db.IsNewContract(addr1))
		require.NoError(t, db.Error())

		db.CreateContract(addr1)
		require.NoError(t, db.Error())

		require.True(t, db.IsNewContract(addr1))
		require.NoError(t, db.Error())
	})

	t.Run("test commit functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)
		key1 := testutils.RandomCommonHash(t)
		value1 := testutils.RandomCommonHash(t)

		db.CreateAccount(addr1)
		require.NoError(t, db.Error())

		db.AddBalance(addr1, uint256.NewInt(5), tracing.BalanceChangeUnspecified)
		require.NoError(t, db.Error())

		// should have code to be able to set state
		db.SetCode(addr1, []byte{1, 2, 3})
		require.NoError(t, db.Error())

		db.SetState(addr1, key1, value1)

		ret := db.GetState(addr1, key1)
		require.Equal(t, value1, ret)

		ret = db.GetCommittedState(addr1, key1)
		require.Equal(t, gethCommon.Hash{}, ret)

		err = db.Commit(true)
		require.NoError(t, err)

		ret = db.GetCommittedState(addr1, key1)
		require.Equal(t, value1, ret)

		// create a new db
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		bal := db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, uint256.NewInt(5), bal)

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

		db.AddBalance(addr1, uint256.NewInt(5), tracing.BalanceChangeUnspecified)
		require.NoError(t, db.Error())

		bal := db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, uint256.NewInt(5), bal)

		snapshot2 := db.Snapshot()
		require.Equal(t, 2, snapshot2)

		db.AddBalance(addr1, uint256.NewInt(5), tracing.BalanceChangeUnspecified)
		require.NoError(t, db.Error())

		bal = db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, uint256.NewInt(10), bal)

		// revert to snapshot 2
		db.RevertToSnapshot(snapshot2)
		require.NoError(t, db.Error())

		bal = db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, uint256.NewInt(5), bal)

		// revert to snapshot 1
		db.RevertToSnapshot(snapshot1)
		require.NoError(t, db.Error())

		bal = db.GetBalance(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, uint256.NewInt(0), bal)

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

		ret := db.Logs(1, gethCommon.Hash{}, 1)
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

	t.Run("test Prepare functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)

		sender := testutils.RandomCommonAddress(t)
		coinbase := testutils.RandomCommonAddress(t)
		dest := testutils.RandomCommonAddress(t)
		precompiles := []gethCommon.Address{
			testutils.RandomCommonAddress(t),
			testutils.RandomCommonAddress(t),
		}

		txAccesses := gethTypes.AccessList([]gethTypes.AccessTuple{
			{Address: testutils.RandomCommonAddress(t),
				StorageKeys: []gethCommon.Hash{
					testutils.RandomCommonHash(t),
					testutils.RandomCommonHash(t),
				},
			},
		})

		rules := gethParams.Rules{
			IsBerlin:   true,
			IsShanghai: true,
		}

		require.NoError(t, err)
		db.Prepare(rules, sender, coinbase, &dest, precompiles, txAccesses)

		require.True(t, db.AddressInAccessList(sender))
		require.True(t, db.AddressInAccessList(coinbase))
		require.True(t, db.AddressInAccessList(dest))

		for _, add := range precompiles {
			require.True(t, db.AddressInAccessList(add))
		}

		for _, el := range txAccesses {
			for _, key := range el.StorageKeys {
				addrFound, slotFound := db.SlotInAccessList(el.Address, key)
				require.True(t, addrFound)
				require.True(t, slotFound)
			}
		}
	})

	t.Run("test non-fatal error handling", func(t *testing.T) {
		ledger := &testutils.TestValueStore{
			GetValueFunc: func(owner, key []byte) ([]byte, error) {
				return nil, nil
			},
			SetValueFunc: func(owner, key, value []byte) error {
				return atree.NewUserError(fmt.Errorf("key not found"))
			},
			AllocateSlabIndexFunc: func(owner []byte) (atree.SlabIndex, error) {
				return atree.SlabIndex{}, nil
			},
		}
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		db.CreateAccount(testutils.RandomCommonAddress(t))

		err = db.Commit(true)
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
			AllocateSlabIndexFunc: func(owner []byte) (atree.SlabIndex, error) {
				return atree.SlabIndex{}, nil
			},
		}
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		db.CreateAccount(testutils.RandomCommonAddress(t))

		err = db.Commit(true)
		// ret := db.Error()
		require.Error(t, err)
		// check wrapping
		require.True(t, types.IsAFatalError(err))
	})

	t.Run("test storage root functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)

		// non existing account
		require.False(t, db.Exist(addr1))
		require.NoError(t, db.Error())
		root := db.GetStorageRoot(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, gethCommon.Hash{}, root)

		// accounts without slots
		db.CreateAccount(addr1)
		require.NoError(t, db.Error())
		err = db.Commit(true)
		require.NoError(t, err)

		root = db.GetStorageRoot(addr1)
		require.NoError(t, db.Error())
		require.Equal(t, gethTypes.EmptyRootHash, root)

		// add slots to the account
		key := testutils.RandomCommonHash(t)
		value := testutils.RandomCommonHash(t)
		db.SetCode(addr1, []byte("somecode"))
		require.NoError(t, db.Error())
		db.SetState(addr1, key, value)
		require.NoError(t, db.Error())
		err = db.Commit(true)
		require.NoError(t, err)

		root = db.GetStorageRoot(addr1)
		require.NoError(t, db.Error())
		require.NotEqual(t, gethCommon.Hash{}, root)
		require.NotEqual(t, gethTypes.EmptyRootHash, root)
	})

	t.Run("test Selfdestruct6780 functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		db, err := state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)

		// test 1 - an already existing contract
		// fail for selfdestruction
		addr1 := testutils.RandomCommonAddress(t)
		balance1 := uint256.NewInt(100)
		code1 := []byte("some code")
		db.CreateAccount(addr1)
		db.SetCode(addr1, code1)
		db.AddBalance(addr1, balance1, tracing.BalanceChangeTransfer)
		require.NoError(t, db.Error())
		err = db.Commit(true)
		require.NoError(t, err)
		// renew db
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)
		// call self destruct
		db.Selfdestruct6780(addr1)
		require.NoError(t, db.Error())
		// noop is expected
		require.Equal(t, balance1, db.GetBalance(addr1))
		require.Equal(t, code1, db.GetCode(addr1))
		require.NoError(t, db.Error())

		// test 2 - account exist before with some balance
		// but not a contract - selfdestruct should work
		addr2 := testutils.RandomCommonAddress(t)
		balance2 := uint256.NewInt(200)
		db.CreateAccount(addr2)
		db.AddBalance(addr2, balance2, tracing.BalanceChangeTransfer)
		require.NoError(t, db.Error())
		// commit and renew db
		err = db.Commit(true)
		require.NoError(t, err)
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)
		// call self destruct should not work
		db.Selfdestruct6780(addr2)
		require.NoError(t, db.Error())
		// still no impact
		require.Equal(t, balance2, db.GetBalance(addr2))
		require.Empty(t, db.GetCode(addr2))
		require.NoError(t, db.Error())
		// commit and renew db
		err = db.Commit(true)
		require.NoError(t, err)
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)
		// set code and call contract creation
		db.SetCode(addr2, code1)
		db.CreateContract(addr2)
		require.Equal(t, code1, db.GetCode(addr2))
		// now calling selfdestruct should do the job
		db.Selfdestruct6780(addr2)
		require.NoError(t, db.Error())
		err = db.Commit(true)
		require.NoError(t, err)
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)
		// now query
		require.Equal(t, uint256.NewInt(0), db.GetBalance(addr2))
		require.Empty(t, db.GetCode(addr2))
		require.NoError(t, db.Error())

		// test 3 - create account and call self destruct in a single operation
		// selfdestruct should work
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)
		addr3 := testutils.RandomCommonAddress(t)
		balance3 := uint256.NewInt(300)
		key := testutils.RandomCommonHash(t)
		value := testutils.RandomCommonHash(t)
		db.CreateAccount(addr3)
		db.CreateContract(addr3)
		db.SetCode(addr3, code1)
		db.SetState(addr3, key, value)
		db.AddBalance(addr3, balance3, tracing.BalanceChangeTransfer)
		require.NoError(t, db.Error())
		// call self destruct
		db.Selfdestruct6780(addr3)
		require.NoError(t, db.Error())
		// commit changes
		err = db.Commit(true)
		require.NoError(t, err)
		// renew db
		db, err = state.NewStateDB(ledger, rootAddr)
		require.NoError(t, err)
		// account should not exist
		require.False(t, db.Exist(addr3))
		require.Equal(t, uint256.NewInt(0), db.GetBalance(addr3))
		require.Empty(t, db.GetCode(addr3))
		require.Equal(t, gethCommon.Hash{}, db.GetState(addr3, key))
		require.NoError(t, db.Error())
	})
}
