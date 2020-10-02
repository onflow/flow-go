package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccounts_GetWithNoKeys(t *testing.T) {
	chain := flow.Mainnet.Chain()

	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger, chain)

	address, err := accounts.Create(nil)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

// Some old account could be created without key count register
// we recreate it in a test
func TestAccounts_GetWithNoKeysCounter(t *testing.T) {
	chain := flow.Mainnet.Chain()

	ledger := state.NewMapLedger()

	accounts := state.NewAccounts(ledger, chain)

	address, err := accounts.Create(nil)
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
	if key == "contracts" {
		l.contracts = value
	}
}
func (l *TestLedger) Get(_, _, key string) (flow.RegisterValue, error) {
	if key == "exists" {
		return []byte("1"), nil
	}
	if key == "contracts" {
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
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		contracts, err := a.GetContracts(address)
		if err != nil {
			t.Errorf("GetContracts() error = %v", err)
		}
		if len(contracts) != 1 || contracts[0] != "Dummy" {
			t.Errorf("There should be only one contract")
		}
	})
	t.Run("Setting a contract again, does not add it to contracts", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)
		err := a.SetContract("Dummy", address, []byte("non empty string"))
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		err = a.SetContract("Dummy", address, []byte("non empty string"))
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		contracts, err := a.GetContracts(address)
		if err != nil {
			t.Errorf("GetContracts() error = %v", err)
		}
		if len(contracts) != 1 || contracts[0] != "Dummy" {
			t.Errorf("There should be only one contract")
		}
	})
	t.Run("Setting more contracts always keeps them sorted", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)
		err := a.SetContract("Dummy", address, []byte("non empty string"))
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		err = a.SetContract("ZedDummy", address, []byte("non empty string"))
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		err = a.SetContract("ADummy", address, []byte("non empty string"))
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		contracts, err := a.GetContracts(address)
		if err != nil {
			t.Errorf("GetContracts() error = %v", err)
		}
		if len(contracts) != 3 || contracts[0] != "ADummy" || contracts[1] != "Dummy" || contracts[2] != "ZedDummy" {
			t.Errorf("Contracts should be sorted")
		}
	})
	t.Run("Removing a contract does not fail if there is none", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)
		err := a.DeleteContract("Dummy", address)
		if err != nil {
			t.Errorf("DeleteContract() error = %v", err)
		}
	})
	t.Run("Removing a contract removes it", func(t *testing.T) {
		ledger := TestLedger{}
		a := state.NewAccounts(&ledger, chain)
		err := a.SetContract("Dummy", address, []byte("non empty string"))
		if err != nil {
			t.Errorf("SetContract() error = %v", err)
		}
		err = a.DeleteContract("Dummy", address)
		if err != nil {
			t.Errorf("DeleteContract() error = %v", err)
		}
		contracts, err := a.GetContracts(address)
		if err != nil {
			t.Errorf("GetContracts() error = %v", err)
		}
		if len(contracts) != 0 {
			t.Errorf("There should be no contract")
		}
	})
}
