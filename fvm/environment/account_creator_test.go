package environment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func Test_NewAccountCreator_NoError(t *testing.T) {
	chain := flow.MonotonicEmulator.Chain()
	txnState := testutils.NewSimpleTransaction(nil)
	creator := environment.NewAddressGenerator(txnState, chain)
	require.NotNil(t, creator)
}

func Test_NewAccountCreator_GeneratingUpdatesState(t *testing.T) {
	chain := flow.MonotonicEmulator.Chain()
	txnState := testutils.NewSimpleTransaction(nil)
	creator := environment.NewAddressGenerator(txnState, chain)
	_, err := creator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := txnState.Get(flow.AddressStateRegisterID)
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("01"))
}

func Test_NewAccountCreator_UsesLedgerState(t *testing.T) {
	chain := flow.MonotonicEmulator.Chain()
	txnState := testutils.NewSimpleTransaction(
		snapshot.MapStorageSnapshot{
			flow.AddressStateRegisterID: flow.HexToAddress("01").Bytes(),
		})
	creator := environment.NewAddressGenerator(txnState, chain)

	_, err := creator.NextAddress()
	require.NoError(t, err)

	stateBytes, err := txnState.Get(flow.AddressStateRegisterID)
	require.NoError(t, err)

	require.Equal(t, flow.BytesToAddress(stateBytes), flow.HexToAddress("02"))
	// counts is one unit higher than returned index (index include zero, but counts starts from 1)
	require.Equal(t, uint64(2), creator.AddressCount())
}
