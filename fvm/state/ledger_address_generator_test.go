package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Test_NewLedgerBoundAddressGenerator_NoError(t *testing.T) {
	ledger := state.NewMapLedger()
	chain := flow.MonotonicEmulator.Chain()
	st := state.NewState(ledger, MaxStateKeySize, MaxStateValueSize, MaxStateInteractionSize)
	_, err := state.NewLedgerBoundAddressGenerator(st, chain)
	require.NoError(t, err)
}

func Test_NewLedgerBoundAddressGenerator_GeneratingUpdatesState(t *testing.T) {
	ledger := state.NewMapLedger()
	chain := flow.MonotonicEmulator.Chain()
	st := state.NewState(ledger, MaxStateKeySize, MaxStateValueSize, MaxStateInteractionSize)
	generator, err := state.NewLedgerBoundAddressGenerator(st, chain)
	require.NoError(t, err)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	st.Commit()
	stateBytes, err := ledger.Get("", "", "account_address_state")
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("01"))
}

func Test_NewLedgerBoundAddressGenerator_UsesLedgerState(t *testing.T) {
	ledger := state.NewMapLedger()
	ledger.Set("", "", "account_address_state", flow.HexToAddress("01").Bytes())
	chain := flow.MonotonicEmulator.Chain()
	st := state.NewState(ledger, MaxStateKeySize, MaxStateValueSize, MaxStateInteractionSize)
	generator, err := state.NewLedgerBoundAddressGenerator(st, chain)
	require.NoError(t, err)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	st.Commit()
	stateBytes, err := ledger.Get("", "", "account_address_state")
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("02"))
}
