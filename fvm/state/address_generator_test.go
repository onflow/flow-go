package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Test_NewStateBoundAddressGenerator_NoError(t *testing.T) {
	view := NewSimpleView()
	chain := flow.MonotonicEmulator.Chain()
	stm := state.NewStateManager(state.NewState(view))
	_, err := state.NewStateBoundAddressGenerator(stm, chain)
	require.NoError(t, err)
}

func Test_NewStateBoundAddressGenerator_GeneratingUpdatesState(t *testing.T) {
	view := NewSimpleView()
	chain := flow.MonotonicEmulator.Chain()
	stm := state.NewStateManager(state.NewState(view))
	generator, err := state.NewStateBoundAddressGenerator(stm, chain)
	require.NoError(t, err)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := view.Get("", "", "account_address_state")
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("01"))
}

func Test_NewStateBoundAddressGenerator_UsesLedgerState(t *testing.T) {
	view := NewSimpleView()
	err := view.Set("", "", "account_address_state", flow.HexToAddress("01").Bytes())
	require.NoError(t, err)

	chain := flow.MonotonicEmulator.Chain()
	stm := state.NewStateManager(state.NewState(view))
	generator, err := state.NewStateBoundAddressGenerator(stm, chain)
	require.NoError(t, err)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := view.Get("", "", "account_address_state")
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("02"))
}
