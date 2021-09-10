package partial_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/ledger/partial"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

func TestFunctionalityWithCompleteTrie(t *testing.T) {

	l, err := complete.NewLedger(&fixtures.NoopWAL{}, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	// create empty update
	state := l.InitialState()
	keys := utils.RandomUniqueKeys(3, 2, 2, 4)
	values := utils.RandomValues(3, 1, 32)
	update, err := ledger.NewUpdate(state, keys[0:2], values[0:2])
	require.NoError(t, err)

	newState, _, err := l.Set(update)
	require.NoError(t, err)

	query, err := ledger.NewQuery(newState, keys[0:2])
	require.NoError(t, err)
	proof, err := l.Prove(query)
	require.NoError(t, err)

	pled, err := partial.NewLedger(proof, newState, partial.DefaultPathFinderVersion)
	assert.NoError(t, err)
	assert.Equal(t, pled.InitialState(), newState)

	// test missing keys (get)
	query, err = ledger.NewQuery(newState, keys[1:3])
	require.NoError(t, err)

	_, err = pled.Get(query)
	require.Error(t, err)

	e, ok := err.(*ledger.ErrMissingKeys)
	require.True(t, ok)
	assert.Equal(t, len(e.Keys), 1)
	require.True(t, e.Keys[0].Equals(&keys[2]))

	// test missing keys (set)
	update, err = ledger.NewUpdate(state, keys[1:3], values[1:3])
	require.NoError(t, err)

	_, _, err = pled.Set(update)
	require.Error(t, err)

	e, ok = err.(*ledger.ErrMissingKeys)
	require.True(t, ok)
	assert.Equal(t, len(e.Keys), 1)
	require.True(t, e.Keys[0].Equals(&keys[2]))

}

func TestProofsForEmptyRegisters(t *testing.T) {

	l, err := complete.NewLedger(&fixtures.NoopWAL{}, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	// create empty update
	emptyState := l.InitialState()

	view := delta.NewView(executionState.LedgerGetRegister(l, flow.StateCommitment(emptyState)))

	registerID := flow.NewRegisterID("b", "o", "nk")

	v, err := view.Get(registerID.Owner, registerID.Controller, registerID.Key)
	require.NoError(t, err)
	require.Empty(t, v)

	ids, values := view.Delta().RegisterUpdates()
	updated, err := ledger.NewUpdate(
		emptyState,
		executionState.RegisterIDSToKeys(ids),
		executionState.RegisterValuesToValues(values),
	)
	require.NoError(t, err)

	allRegisters := view.Interactions().AllRegisters()
	allKeys := executionState.RegisterIDSToKeys(allRegisters)
	newState := updated.State()

	proofQuery, err := ledger.NewQuery(newState, allKeys)
	require.NoError(t, err)

	proof, err := l.Prove(proofQuery)
	require.NoError(t, err)

	pled, err := partial.NewLedger(proof, newState, partial.DefaultPathFinderVersion)
	assert.NoError(t, err)
	assert.Equal(t, pled.InitialState(), emptyState)

	query, err := ledger.NewQuery(newState, []ledger.Key{executionState.RegisterIDToKey(registerID)})
	require.NoError(t, err)

	results, err := pled.Get(query)
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.Empty(t, results[0])

}
