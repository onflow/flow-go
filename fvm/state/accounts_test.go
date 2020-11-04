package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccounts_Create(t *testing.T) {
	t.Run("Sets registers", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		require.Equal(t, len(ledger.RegisterTouches), 4) // storage_used + exists + code + key count
	})

	t.Run("Fails if account exists", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.Create(nil, address)

		require.Error(t, err)
	})
}

func TestAccounts_GetWithNoKeys(t *testing.T) {
	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func TestAccounts_GetWithNoKeysCounter(t *testing.T) {
	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	ledger.Delete(
		string(address.Bytes()),
		string(address.Bytes()),
		"public_key_count")

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

func TestAccount_StorageUsed(t *testing.T) {

	t.Run("Storage used on account creation is deterministic", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9)) // exists: 1 byte, storage_used 8 bytes
	})

	t.Run("Storage used on register set increases", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9+12)) // exists: 1 byte, storage_used 8 bytes, some_key 12
	})

	t.Run("Storage used, set twice on same register to same value, stays the same", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9+12)) // exists: 1 byte, storage_used 8 bytes, some_key 12
	})

	t.Run("Storage used, set twice on same register to larger value, increases", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(13))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9+13)) // exists: 1 byte, storage_used 8 bytes, some_key 13
	})

	t.Run("Storage used, set twice on same register to smaller value, decreases", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", createByteArray(11))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9+11)) // exists: 1 byte, storage_used 8 bytes, some_key 11
	})

	t.Run("Storage used, after register deleted, decreases", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(address, "some_key", createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(address, "some_key", nil)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9+0)) // exists: 1 byte, storage_used 8 bytes, some_key 0
	})

	t.Run("Storage used on a complex scenario has correct value", func(t *testing.T) {
		ledger := state.NewMapLedger()

		accounts := state.NewAccounts(ledger)
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

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, storageUsed, uint64(9+34)) // exists: 1 byte, storage_used 8 bytes, other 34
	})
}

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}
