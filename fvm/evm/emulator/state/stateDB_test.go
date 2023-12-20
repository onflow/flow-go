package state_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func TestStateDB(t *testing.T) {
	t.Parallel()

	t.Run("test commit functionality", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
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
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
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
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
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

	// TODO: add test for error handling
}
