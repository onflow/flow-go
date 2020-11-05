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

		require.Equal(t, len(ledger.RegisterTouches), 3) // exists + code + key count
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

	chain := struct{ flow.Chain }{}
	address := flow.HexToAddress("0x01")

	t.Run("Setting a contract puts it in Contracts", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)

		err := a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 1, "There should only be one contract")
		require.Equal(t, contractNames[0], "Dummy")
	})
	t.Run("Setting a contract again, does not add it to contracts", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)

		err := a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 1, "There should only be one contract")
		require.Equal(t, contractNames[0], "Dummy")
	})
	t.Run("Setting more contracts always keeps them sorted", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)

		err := a.SetContract("Dummy", address, []byte("non empty string"))
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
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)

		err := a.DeleteContract("Dummy", address)
		require.NoError(t, err)
	})
	t.Run("Removing a contract removes it", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)

		err := a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 0, "There should be no contract")
	})
}
