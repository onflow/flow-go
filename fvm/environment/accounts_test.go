package environment_test

import (
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccounts_Create(t *testing.T) {
	t.Run("Sets registers", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// account status
		require.Equal(t, len(snapshot.AllRegisterIDs()), 1)
	})

	t.Run("Fails if account exists", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.Create(nil, address)

		require.Error(t, err)
	})
}

func TestAccounts_GetWithNoKeys(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
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
		registerId := flow.NewRegisterID(
			address,
			"public_key_0")

		for _, value := range [][]byte{{}, nil} {
			txnState := testutils.NewSimpleTransaction(
				snapshot.MapStorageSnapshot{
					registerId: value,
				})
			accounts := environment.NewAccounts(txnState)

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			_, err = accounts.GetPublicKey(address, 0)
			require.True(t, errors.IsAccountPublicKeyNotFoundError(err))
		}
	})
}

func TestAccounts_GetPublicKeyCount(t *testing.T) {

	t.Run("non-existent key count", func(t *testing.T) {

		address := flow.HexToAddress("01")
		registerId := flow.NewRegisterID(
			address,
			"public_key_count")

		for _, value := range [][]byte{{}, nil} {
			txnState := testutils.NewSimpleTransaction(
				snapshot.MapStorageSnapshot{
					registerId: value,
				})
			accounts := environment.NewAccounts(txnState)

			err := accounts.Create(nil, address)
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
		registerId := flow.NewRegisterID(
			address,
			"public_key_count")

		for _, value := range [][]byte{{}, nil} {
			txnState := testutils.NewSimpleTransaction(
				snapshot.MapStorageSnapshot{
					registerId: value,
				})

			accounts := environment.NewAccounts(txnState)

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			keys, err := accounts.GetPublicKeys(address)
			require.NoError(t, err)
			require.Empty(t, keys)
		}
	})
}

func TestAccounts_SetContracts(t *testing.T) {

	address := flow.HexToAddress("0x01")

	t.Run("Setting a contract puts it in Contracts", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
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
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
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
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
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
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)
	})
	t.Run("Removing a contract removes it", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
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
	emptyAccountSize := uint64(48)

	t.Run("Storage used on account creation is deterministic", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize, storageUsed)
	})

	t.Run("Storage used on register set increases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(32), storageUsed)
	})

	t.Run("Storage used, set twice on same register to same value, stays the same", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(32), storageUsed)
	})

	t.Run("Storage used, set twice on same register to larger value, increases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, createByteArray(13))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(33), storageUsed)
	})

	t.Run("Storage used, set twice on same register to smaller value, decreases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, createByteArray(11))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(31), storageUsed)
	})

	t.Run("Storage used, after register deleted, decreases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, nil)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(0), storageUsed)
	})

	t.Run("Storage used on a complex scenario has correct value", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		key1 := flow.NewRegisterID(address, "some_key")
		err = accounts.SetValue(key1, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key1, createByteArray(11))
		require.NoError(t, err)

		key2 := flow.NewRegisterID(address, "some_key2")
		err = accounts.SetValue(key2, createByteArray(22))
		require.NoError(t, err)
		err = accounts.SetValue(key2, createByteArray(23))
		require.NoError(t, err)

		key3 := flow.NewRegisterID(address, "some_key3")
		err = accounts.SetValue(key3, createByteArray(22))
		require.NoError(t, err)
		err = accounts.SetValue(key3, createByteArray(0))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(33+42), storageUsed)
	})
}

func TestStatefulAccounts_GenerateAccountLocalID(t *testing.T) {

	// Create 3 accounts
	addressA := flow.HexToAddress("0x01")
	addressB := flow.HexToAddress("0x02")
	addressC := flow.HexToAddress("0x03")
	txnState := testutils.NewSimpleTransaction(nil)
	a := environment.NewAccounts(txnState)
	err := a.Create(nil, addressA)
	require.NoError(t, err)
	err = a.Create(nil, addressB)
	require.NoError(t, err)
	err = a.Create(nil, addressC)
	require.NoError(t, err)

	// setup some state
	_, err = a.GenerateAccountLocalID(addressA)
	require.NoError(t, err)
	_, err = a.GenerateAccountLocalID(addressA)
	require.NoError(t, err)
	_, err = a.GenerateAccountLocalID(addressB)
	require.NoError(t, err)

	// assert

	// addressA
	id, err := a.GenerateAccountLocalID(addressA)
	require.NoError(t, err)
	require.Equal(t, uint64(3), id)

	// addressB
	id, err = a.GenerateAccountLocalID(addressB)
	require.NoError(t, err)
	require.Equal(t, uint64(2), id)

	// addressC
	id, err = a.GenerateAccountLocalID(addressC)
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)
}

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}

func TestAccounts_AllocateStorageIndex(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	// no register set case
	i, err := accounts.AllocateSlabIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))

	// register already set case
	i, err = accounts.AllocateSlabIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 2}))

	// register update successful
	i, err = accounts.AllocateSlabIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 3}))
}
