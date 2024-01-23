package state_test

import (
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestBaseView(t *testing.T) {
	t.Parallel()

	t.Run("test account functionalities", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
		view, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)

		// data calls for a non-existent account
		checkAccount(t,
			view,
			addr1,
			false,
			big.NewInt(0),
			uint64(0),
			nil,
			gethTypes.EmptyCodeHash,
		)

		// create an account with code
		newBal := big.NewInt(10)
		newNonce := uint64(5)
		newCode := []byte("some code")
		newCodeHash := gethCommon.Hash{1, 2}

		err = view.CreateAccount(addr1, newBal, newNonce, newCode, newCodeHash)
		require.NoError(t, err)

		// check data from cache
		checkAccount(t,
			view,
			addr1,
			true,
			newBal,
			newNonce,
			newCode,
			newCodeHash,
		)

		// commit the changes and create a new baseview
		err = view.Commit()
		require.NoError(t, err)

		view, err = state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		checkAccount(t,
			view,
			addr1,
			true,
			newBal,
			newNonce,
			newCode,
			newCodeHash,
		)

		// test update account

		newBal = big.NewInt(12)
		newNonce = uint64(6)
		newCode = []byte("some new code")
		newCodeHash = gethCommon.Hash{2, 3}
		err = view.UpdateAccount(addr1, newBal, newNonce, newCode, newCodeHash)
		require.NoError(t, err)

		// check data from cache
		checkAccount(t,
			view,
			addr1,
			true,
			newBal,
			newNonce,
			newCode,
			newCodeHash,
		)

		// commit the changes and create a new baseview
		err = view.Commit()
		require.NoError(t, err)

		view, err = state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		checkAccount(t,
			view,
			addr1,
			true,
			newBal,
			newNonce,
			newCode,
			newCodeHash,
		)

		// test delete account

		err = view.DeleteAccount(addr1)
		require.NoError(t, err)

		// check from cache
		checkAccount(t,
			view,
			addr1,
			false,
			big.NewInt(0),
			uint64(0),
			nil,
			gethTypes.EmptyCodeHash,
		)

		// commit the changes and create a new baseview
		err = view.Commit()
		require.NoError(t, err)

		view, err = state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		checkAccount(t,
			view,
			addr1,
			false,
			big.NewInt(0),
			uint64(0),
			nil,
			gethTypes.EmptyCodeHash,
		)
	})

	t.Run("test slot storage", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
		view, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		addr1 := testutils.RandomCommonAddress(t)
		key1 := testutils.RandomCommonHash(t)
		slot1 := types.SlotAddress{
			Address: addr1,
			Key:     key1,
		}

		// non-existent account
		value, err := view.GetState(slot1)
		require.NoError(t, err)
		require.Equal(t, value, gethCommon.Hash{})

		// store a new value
		newValue := testutils.RandomCommonHash(t)

		// updating slot for non-existent account should fail
		err = view.UpdateSlot(slot1, newValue)
		require.Error(t, err)

		// account should have code to have slots
		err = view.CreateAccount(addr1, big.NewInt(10), 0, []byte("ABC"), gethCommon.Hash{1, 2, 3})
		require.NoError(t, err)

		err = view.UpdateSlot(slot1, newValue)
		require.NoError(t, err)

		// return result from the cache
		value, err = view.GetState(slot1)
		require.NoError(t, err)
		require.Equal(t, newValue, value)

		// commit changes
		err = view.Commit()
		require.NoError(t, err)

		view2, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		// return state from ledger
		value, err = view2.GetState(slot1)
		require.NoError(t, err)
		require.Equal(t, newValue, value)
	})

	t.Run("default values method calls", func(t *testing.T) {
		// calls to these method that has always same value
		view, err := state.NewBaseView(testutils.GetSimpleValueStore(), flow.Address{1, 2, 3, 4})
		require.NoError(t, err)

		dest, bal := view.HasSelfDestructed(gethCommon.Address{})
		require.Equal(t, false, dest)
		require.Equal(t, new(big.Int), bal)
		require.Equal(t, false, view.IsCreated(gethCommon.Address{}))
		require.Equal(t, uint64(0), view.GetRefund())
		require.Equal(t, gethCommon.Hash{}, view.GetTransientState(types.SlotAddress{}))
		require.Equal(t, false, view.AddressInAccessList(gethCommon.Address{}))
		addrFound, slotFound := view.SlotInAccessList(types.SlotAddress{})
		require.Equal(t, false, addrFound)
		require.Equal(t, false, slotFound)
	})
}

func checkAccount(t *testing.T,
	view *state.BaseView,
	addr gethCommon.Address,
	exists bool,
	balance *big.Int,
	nonce uint64,
	code []byte,
	codeHash gethCommon.Hash,
) {
	ex, err := view.Exist(addr)
	require.NoError(t, err)
	require.Equal(t, exists, ex)

	bal, err := view.GetBalance(addr)
	require.NoError(t, err)
	require.Equal(t, balance, bal)

	no, err := view.GetNonce(addr)
	require.NoError(t, err)
	require.Equal(t, nonce, no)

	cd, err := view.GetCode(addr)
	require.NoError(t, err)
	require.Equal(t, code, cd)

	cs, err := view.GetCodeSize(addr)
	require.NoError(t, err)
	require.Equal(t, len(code), cs)

	ch, err := view.GetCodeHash(addr)
	require.NoError(t, err)
	require.Equal(t, codeHash, ch)
}
