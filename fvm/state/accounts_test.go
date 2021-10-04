package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccounts_Create(t *testing.T) {
	t.Run("Sets registers", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		// storage_used + exists + key count
		require.Equal(t, len(view.Ledger.RegisterTouches), 3)
	})

	t.Run("Fails if account exists", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.Create(nil, address)

		require.Error(t, err)
	})
}

func TestAccounts_GetWithNoKeys(t *testing.T) {
	view := utils.NewSimpleView()
	sth := state.NewStateHolder(state.NewState(view))
	accounts := state.NewAccounts(sth)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

func TestAccounts_GetPublicKey(t *testing.T) {

	t.Run("non-existent key index", func(t *testing.T) {

		address := flow.HexToAddress("01")

		for _, ledgerValue := range [][]byte{{}, nil} {

			view := utils.NewSimpleView()

			err := view.Set(
				string(address.Bytes()), string(address.Bytes()), "public_key_0",
				ledgerValue,
			)
			require.NoError(t, err)

			sth := state.NewStateHolder(state.NewState(view))
			accounts := state.NewAccounts(sth)

			err = accounts.Create(nil, address)
			require.NoError(t, err)

			_, err = accounts.GetPublicKey(address, 0)
			require.True(t, errors.IsAccountAccountPublicKeyNotFoundError(err))
		}
	})
}

func TestAccounts_GetPublicKeyCount(t *testing.T) {

	t.Run("non-existent key count", func(t *testing.T) {

		address := flow.HexToAddress("01")

		for _, ledgerValue := range [][]byte{{}, nil} {

			view := utils.NewSimpleView()
			err := view.Set(
				string(address.Bytes()), string(address.Bytes()), "public_key_count",
				ledgerValue,
			)
			require.NoError(t, err)

			sth := state.NewStateHolder(state.NewState(view))
			accounts := state.NewAccounts(sth)

			err = accounts.Create(nil, address)
			require.NoError(t, err)

			count, err := accounts.GetPublicKeyCount(address)
			require.NoError(t, err)
			require.Equal(t, uint64(0), count)
		}
	})
}

func TestAccounts_GetPublicKeys(t *testing.T) {

	t.Run("non-existent key count", func(t *testing.T) {

		address := flow.HexToAddress("01")

		for _, ledgerValue := range [][]byte{{}, nil} {

			view := utils.NewSimpleView()
			err := view.Set(
				string(address.Bytes()), string(address.Bytes()), "public_key_count",
				ledgerValue,
			)
			require.NoError(t, err)

			sth := state.NewStateHolder(state.NewState(view))
			accounts := state.NewAccounts(sth)

			err = accounts.Create(nil, address)
			require.NoError(t, err)

			keys, err := accounts.GetPublicKeys(address)
			require.NoError(t, err)
			require.Empty(t, keys)
		}
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func TestAccounts_GetWithNoKeysCounter(t *testing.T) {
	view := utils.NewSimpleView()

	sth := state.NewStateHolder(state.NewState(view))
	accounts := state.NewAccounts(sth)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	err = view.Delete(
		string(address.Bytes()),
		string(address.Bytes()),
		"public_key_count")

	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

func TestAccounts_SetContracts(t *testing.T) {

	address := flow.HexToAddress("0x01")

	t.Run("Setting a contract puts it in Contracts", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		a := state.NewAccounts(sth)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 1, "There should only be one contract")
		require.Equal(t, contractNames[0], "Dummy")
	})
	t.Run("Setting a contract again, does not add it to contracts", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		a := state.NewAccounts(sth)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 1, "There should only be one contract")
		require.Equal(t, contractNames[0], "Dummy")
	})
	t.Run("Setting more contracts always keeps them sorted", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		a := state.NewAccounts(sth)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("ZedDummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("ADummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 3)
		require.Equal(t, contractNames[0], "ADummy")
		require.Equal(t, contractNames[1], "Dummy")
		require.Equal(t, contractNames[2], "ZedDummy")
	})
	t.Run("Removing a contract does not fail if there is none", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		a := state.NewAccounts(sth)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)
	})
	t.Run("Removing a contract removes it", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		a := state.NewAccounts(sth)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 0, "There should be no contract")
	})
}

func TestAccount_StorageUsed(t *testing.T) {

	t.Run("Storage used on account creation is deterministic", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55), storageUsed)
	})

	t.Run("Storage used on register set increases", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55+34), storageUsed)
	})

	t.Run("Storage used, set twice on same register to same value, stays the same", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55+34), storageUsed)
	})

	t.Run("Storage used, set twice on same register to larger value, increases", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(13))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55+35), storageUsed)
	})

	t.Run("Storage used, set twice on same register to smaller value, decreases", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(11))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55+33), storageUsed)
	})

	t.Run("Storage used, after register deleted, decreases", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", nil)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55+0), storageUsed)
	})

	t.Run("Storage used on a complex scenario has correct value", func(t *testing.T) {
		view := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(view))
		accounts := state.NewAccounts(sth)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(11))
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key2", createByteArray(22))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key2", createByteArray(23))
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key3", createByteArray(22))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key3", createByteArray(0))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, uint64(55+33+46), storageUsed)
	})
}

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}

func TestAccounts_AllocateStorageIndex(t *testing.T) {
	view := utils.NewSimpleView()

	sth := state.NewStateHolder(state.NewState(view))
	accounts := state.NewAccounts(sth)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	// no register set case
	i, err := accounts.AllocateStorageIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.StorageIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))

	// register already set case
	i, err = accounts.AllocateStorageIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.StorageIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 2}))

	// register update successful
	i, err = accounts.AllocateStorageIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.StorageIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 3}))
}
