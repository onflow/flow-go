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

		require.Equal(t, len(ledger.RegisterTouches), 3) // storage_used + exists + key count
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

type TestLedger struct {
	contracts []byte
}

func (l *TestLedger) Set(_, _, key string, value flow.RegisterValue) {
	if key == "contract_names" {
		l.contracts = value
	}
}
func (l *TestLedger) Get(_, _, key string) (flow.RegisterValue, error) {
	if key == "exists" {
		return []byte("1"), nil
	}
	if key == "contract_names" {
		return l.contracts, nil
	}
	return nil, nil
}
func (l *TestLedger) Touch(_, _, _ string)  {}
func (l *TestLedger) Delete(_, _, _ string) {}

func TestAccounts_SetContracts(t *testing.T) {

	address := flow.HexToAddress("0x01")

	t.Run("Setting a contract puts it in Contracts", func(t *testing.T) {
		ledger := state.NewMapLedger()
		a := state.NewAccounts(ledger)
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
		ledger := state.NewMapLedger()
		a := state.NewAccounts(ledger)
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
		ledger := state.NewMapLedger()
		a := state.NewAccounts(ledger)
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
		ledger := state.NewMapLedger()
		a := state.NewAccounts(ledger)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)
	})
	t.Run("Removing a contract removes it", func(t *testing.T) {
		ledger := state.NewMapLedger()
		a := state.NewAccounts(ledger)
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
