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

		// storage_used + storage_capacity + exists + code + key count
		require.Equal(t, len(ledger.RegisterTouches), 5)
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
		require.Equal(t, storageUsed, uint64(17)) // exists: 1 byte, storage_ used & capacity 16 bytes
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
		require.Equal(t, storageUsed, uint64(17+12)) // exists: 1 byte, storage_ used & capacity 16 bytes, some_key 12
	})

	t.Run("Storage used on same register set twice to same value stays the same", func(t *testing.T) {
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
		require.Equal(t, storageUsed, uint64(17+12)) // exists: 1 byte, storage_ used & capacity 16 bytes, some_key 12
	})

	t.Run("Storage used on register set twice to larger value increases", func(t *testing.T) {
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
		require.Equal(t, storageUsed, uint64(17+13)) // exists: 1 byte, storage_ used & capacity 16 bytes, some_key 13
	})

	t.Run("Storage used on register set twice to smaller value decreases", func(t *testing.T) {
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
		require.Equal(t, storageUsed, uint64(17+11)) // exists: 1 byte, storage_ used & capacity 16 bytes, some_key 11
	})

	t.Run("Storage used proper value on a complex scenario", func(t *testing.T) {
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
		require.Equal(t, storageUsed, uint64(17+34)) // exists: 1 byte, storage_ used & capacity 16 bytes, other 34
	})
}

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}
