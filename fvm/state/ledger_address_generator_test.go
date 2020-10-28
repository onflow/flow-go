package state_test

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_NewLedgerBoundAddressGenerator_NoError(t *testing.T) {
	ledger := state.NewMapLedger()
	chain := flow.MonotonicEmulator.Chain()
	_, err := state.NewLedgerBoundAddressGenerator(ledger, chain)
	require.NoError(t, err)
}

func Test_NewLedgerBoundAddressGenerator_GeneratingUpdatesState(t *testing.T) {
	ledger := state.NewMapLedger()
	chain := flow.MonotonicEmulator.Chain()
	generator, err := state.NewLedgerBoundAddressGenerator(ledger, chain)
	require.NoError(t, err)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := ledger.Get("", "", "account_address_state")

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("01"))
}

func Test_NewLedgerBoundAddressGenerator_UsesLedgerState(t *testing.T) {
	ledger := state.NewMapLedger()
	ledger.Set("", "", "account_address_state", flow.HexToAddress("01").Bytes())
	chain := flow.MonotonicEmulator.Chain()
	generator, err := state.NewLedgerBoundAddressGenerator(ledger, chain)
	require.NoError(t, err)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := ledger.Get("", "", "account_address_state")

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("02"))
}
