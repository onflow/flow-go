package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func Test_NewStateBoundAddressGenerator_NoError(t *testing.T) {
	view := utils.NewSimpleView()
	chain := flow.MonotonicEmulator.Chain()
	sth := state.NewStateHolder(state.NewState(view))
	env := state.NewStateBoundAddressGenerator(sth, chain)
	require.NotNil(t, env)
}

func Test_NewStateBoundAddressGenerator_GeneratingUpdatesState(t *testing.T) {
	view := utils.NewSimpleView()
	chain := flow.MonotonicEmulator.Chain()
	sth := state.NewStateHolder(state.NewState(view))
	generator := state.NewStateBoundAddressGenerator(sth, chain)
	_, err := generator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := view.Get("", "", "account_address_state")
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("01"))
}

func Test_NewStateBoundAddressGenerator_UsesLedgerState(t *testing.T) {
	view := utils.NewSimpleView()
	err := view.Set("", "", "account_address_state", flow.HexToAddress("01").Bytes())
	require.NoError(t, err)

	chain := flow.MonotonicEmulator.Chain()
	sth := state.NewStateHolder(state.NewState(view))
	generator := state.NewStateBoundAddressGenerator(sth, chain)

	_, err = generator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := view.Get("", "", "account_address_state")
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("02"))
	// counts is one unit higher than returned index (index include zero, but counts starts from 1)
	require.Equal(t, uint64(2), generator.AddressCount())
}
