package state_test

import (
	"testing"

	"github.com/holiman/uint256"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
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
			uint256.NewInt(0),
			uint64(0),
			nil,
			gethCommon.Hash{},
		)

		// create an account with code
		newBal := uint256.NewInt(10)
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

		newBal = uint256.NewInt(12)
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
			uint256.NewInt(0),
			uint64(0),
			nil,
			gethCommon.Hash{},
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
			uint256.NewInt(0),
			uint64(0),
			nil,
			gethCommon.Hash{},
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
		err = view.CreateAccount(addr1, uint256.NewInt(10), 0, []byte("ABC"), gethCommon.Hash{1, 2, 3})
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
		require.Equal(t, new(uint256.Int), bal)
		require.Equal(t, false, view.IsCreated(gethCommon.Address{}))
		require.Equal(t, uint64(0), view.GetRefund())
		require.Equal(t, gethCommon.Hash{}, view.GetTransientState(types.SlotAddress{}))
		require.Equal(t, false, view.AddressInAccessList(gethCommon.Address{}))
		addrFound, slotFound := view.SlotInAccessList(types.SlotAddress{})
		require.Equal(t, false, addrFound)
		require.Equal(t, false, slotFound)
	})

	t.Run("test code storage", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
		view, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		bal := new(uint256.Int)
		nonce := uint64(0)

		addr1 := testutils.RandomCommonAddress(t)
		var code1 []byte
		codeHash1 := gethTypes.EmptyCodeHash
		err = view.CreateAccount(addr1, bal, nonce, code1, codeHash1)
		require.NoError(t, err)

		ret, err := view.GetCode(addr1)
		require.NoError(t, err)
		require.Equal(t, code1, ret)

		addr2 := testutils.RandomCommonAddress(t)
		code2 := []byte("code2")
		codeHash2 := gethCrypto.Keccak256Hash(code2)
		err = view.CreateAccount(addr2, bal, nonce, code2, codeHash2)
		require.NoError(t, err)

		ret, err = view.GetCode(addr2)
		require.NoError(t, err)
		require.Equal(t, code2, ret)

		err = view.Commit()
		require.NoError(t, err)
		orgSize := ledger.TotalStorageSize()
		require.Equal(t, uint64(1), view.NumberOfContracts())

		err = view.UpdateAccount(addr1, bal, nonce, code2, codeHash2)
		require.NoError(t, err)

		err = view.Commit()
		require.NoError(t, err)
		require.Equal(t, orgSize, ledger.TotalStorageSize())
		require.Equal(t, uint64(1), view.NumberOfContracts())

		ret, err = view.GetCode(addr1)
		require.NoError(t, err)
		require.Equal(t, code2, ret)

		// now remove the code from account 1
		err = view.UpdateAccount(addr1, bal, nonce, code1, codeHash1)
		require.NoError(t, err)

		// there should not be any side effect on the code return for account 2
		// and no impact on storage size
		ret, err = view.GetCode(addr2)
		require.NoError(t, err)
		require.Equal(t, code2, ret)

		ret, err = view.GetCode(addr1)
		require.NoError(t, err)
		require.Equal(t, code1, ret)

		err = view.Commit()
		require.NoError(t, err)
		require.Equal(t, orgSize, ledger.TotalStorageSize())
		require.Equal(t, uint64(1), view.NumberOfContracts())

		// now update account 2 and there should a reduction in storage
		err = view.UpdateAccount(addr2, bal, nonce, code1, codeHash1)
		require.NoError(t, err)

		ret, err = view.GetCode(addr2)
		require.NoError(t, err)
		require.Equal(t, code1, ret)

		err = view.Commit()
		require.NoError(t, err)
		require.Greater(t, orgSize, ledger.TotalStorageSize())
		require.Equal(t, uint64(0), view.NumberOfContracts())

		// delete account 2
		err = view.DeleteAccount(addr2)
		require.NoError(t, err)

		ret, err = view.GetCode(addr2)
		require.NoError(t, err)
		require.Len(t, ret, 0)

		require.Greater(t, orgSize, ledger.TotalStorageSize())
		require.Equal(t, uint64(1), view.NumberOfAccounts())
	})

	t.Run("test account iterator", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
		view, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		accountCounts := 10
		nonces := make(map[gethCommon.Address]uint64)
		balances := make(map[gethCommon.Address]*uint256.Int)
		codeHashes := make(map[gethCommon.Address]gethCommon.Hash)
		for i := 0; i < accountCounts; i++ {
			addr := testutils.RandomCommonAddress(t)
			balance := testutils.RandomUint256Int(1000)
			nonce := testutils.RandomBigInt(1000).Uint64()
			code := testutils.RandomData(t)
			codeHash := testutils.RandomCommonHash(t)

			err = view.CreateAccount(addr, balance, nonce, code, codeHash)
			require.NoError(t, err)

			nonces[addr] = nonce
			balances[addr] = balance
			codeHashes[addr] = codeHash
		}
		err = view.Commit()
		require.NoError(t, err)

		ai, err := view.AccountIterator()
		require.NoError(t, err)

		counter := 0
		for {
			acc, err := ai.Next()
			require.NoError(t, err)
			if acc == nil {
				break
			}
			require.Equal(t, nonces[acc.Address], acc.Nonce)
			delete(nonces, acc.Address)
			require.Equal(t, balances[acc.Address].Uint64(), acc.Balance.Uint64())
			delete(balances, acc.Address)
			require.Equal(t, codeHashes[acc.Address], acc.CodeHash)
			delete(codeHashes, acc.Address)
			counter += 1
		}

		require.Equal(t, accountCounts, counter)
	})

	t.Run("test code iterator", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
		view, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		codeCounts := 10
		codeByCodeHash := make(map[gethCommon.Hash][]byte)
		refCountByCodeHash := make(map[gethCommon.Hash]uint64)
		for i := 0; i < codeCounts; i++ {

			code := testutils.RandomData(t)
			codeHash := testutils.RandomCommonHash(t)
			refCount := 0
			// we add each code couple of times through different accounts
			for j := 1; j <= i+1; j++ {
				addr := testutils.RandomCommonAddress(t)
				balance := testutils.RandomUint256Int(1000)
				nonce := testutils.RandomBigInt(1000).Uint64()
				err = view.CreateAccount(addr, balance, nonce, code, codeHash)
				require.NoError(t, err)
				refCount += 1
			}
			codeByCodeHash[codeHash] = code
			refCountByCodeHash[codeHash] = uint64(refCount)
		}
		err = view.Commit()
		require.NoError(t, err)

		ci, err := view.CodeIterator()
		require.NoError(t, err)

		counter := 0
		for {
			cic, err := ci.Next()
			require.NoError(t, err)
			if cic == nil {
				break
			}
			require.Equal(t, codeByCodeHash[cic.Hash], cic.Code)
			delete(codeByCodeHash, cic.Hash)
			require.Equal(t, refCountByCodeHash[cic.Hash], cic.RefCounts)
			delete(refCountByCodeHash, cic.Hash)
			counter += 1
		}

		require.Equal(t, codeCounts, counter)
	})

	t.Run("test account storage iterator", func(t *testing.T) {
		ledger := testutils.GetSimpleValueStore()
		rootAddr := flow.Address{1, 2, 3, 4, 5, 6, 7, 8}
		view, err := state.NewBaseView(ledger, rootAddr)
		require.NoError(t, err)

		addr := testutils.RandomCommonAddress(t)
		code := []byte("code")
		balance := testutils.RandomUint256Int(1000)
		nonce := testutils.RandomBigInt(1000).Uint64()
		codeHash := gethCrypto.Keccak256Hash(code)
		err = view.CreateAccount(addr, balance, nonce, code, codeHash)
		require.NoError(t, err)

		slotCounts := 10
		values := make(map[gethCommon.Hash]gethCommon.Hash)

		for i := 0; i < slotCounts; i++ {
			key := testutils.RandomCommonHash(t)
			value := testutils.RandomCommonHash(t)

			err = view.UpdateSlot(
				types.SlotAddress{
					Address: addr,
					Key:     key,
				}, value)
			require.NoError(t, err)
			values[key] = value
		}
		err = view.Commit()
		require.NoError(t, err)

		asi, err := view.AccountStorageIterator(addr)
		require.NoError(t, err)

		counter := 0
		for {
			slot, err := asi.Next()
			require.NoError(t, err)
			if slot == nil {
				break
			}
			require.Equal(t, addr, slot.Address)
			require.Equal(t, values[slot.Key], slot.Value)
			delete(values, slot.Key)
			counter += 1
		}

		require.Equal(t, slotCounts, counter)

		// test non existing address
		addr2 := testutils.RandomCommonAddress(t)
		_, err = view.AccountStorageIterator(addr2)
		require.Error(t, err)

		// test address without storage
		err = view.CreateAccount(addr2, balance, nonce, code, codeHash)
		require.NoError(t, err)

		err = view.Commit()
		require.NoError(t, err)

		_, err = view.AccountStorageIterator(addr2)
		require.Error(t, err)
	})

}

func checkAccount(t *testing.T,
	view *state.BaseView,
	addr gethCommon.Address,
	exists bool,
	balance *uint256.Int,
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
