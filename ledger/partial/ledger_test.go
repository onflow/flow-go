package partial_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/ledger/partial"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFunctionalityWithCompleteTrie(t *testing.T) {

	w := &fixtures.NoopWAL{}

	l, err := complete.NewLedger(w, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(l)
	<-compactor.Ready()

	defer func() {
		<-l.Done()
		<-compactor.Done()
	}()

	// create empty update
	state := l.InitialState()
	keys := testutils.RandomUniqueKeys(3, 2, 2, 4)
	values := testutils.RandomValues(3, 1, 32)
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

	// test batch querying existent keys
	query, err = ledger.NewQuery(newState, keys[0:2])
	require.NoError(t, err)

	retValues, err := pled.Get(query)
	require.NoError(t, err)
	require.Equal(t, 2, len(retValues))
	for i := 0; i < len(retValues); i++ {
		require.Equal(t, values[i], retValues[i])
	}

	// test querying single existent key
	querySingleValue, err := ledger.NewQuerySingleValue(newState, keys[0])
	require.NoError(t, err)

	retValue, err := pled.GetSingleValue(querySingleValue)
	require.NoError(t, err)
	require.Equal(t, values[0], retValue)

	// test batch getting missing keys
	query, err = ledger.NewQuery(newState, keys[1:3])
	require.NoError(t, err)

	retValues, err = pled.Get(query)
	require.Error(t, err)
	require.Nil(t, retValues)

	e, ok := err.(*ledger.ErrMissingKeys)
	require.True(t, ok)
	assert.Equal(t, len(e.Keys), 1)
	require.True(t, e.Keys[0].Equals(&keys[2]))

	// test querying single non-existent key
	querySingleValue, err = ledger.NewQuerySingleValue(newState, keys[2])
	require.NoError(t, err)

	retValue, err = pled.GetSingleValue(querySingleValue)
	require.Error(t, err)
	require.Nil(t, retValue)

	e, ok = err.(*ledger.ErrMissingKeys)
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

	// No updates.
	keys, values := executionState.RegisterEntriesToKeysValues(nil)

	updated, err := ledger.NewUpdate(emptyState, keys, values)
	require.NoError(t, err)

	// Read one register during execution.
	registerID := flow.NewRegisterID("b", "nk")
	allKeys := []ledger.Key{
		convert.RegisterIDToLedgerKey(registerID),
	}

	newState := updated.State()

	proofQuery, err := ledger.NewQuery(newState, allKeys)
	require.NoError(t, err)

	proof, err := l.Prove(proofQuery)
	require.NoError(t, err)

	pled, err := partial.NewLedger(proof, newState, partial.DefaultPathFinderVersion)
	assert.NoError(t, err)
	assert.Equal(t, pled.InitialState(), emptyState)

	query, err := ledger.NewQuery(newState, []ledger.Key{convert.RegisterIDToLedgerKey(registerID)})
	require.NoError(t, err)

	results, err := pled.Get(query)
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.Empty(t, results[0])
}

func TestEmptyLedger(t *testing.T) {
	l, err := complete.NewLedger(&fixtures.NoopWAL{}, 100, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	u, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{},
		[]ledger.Value{},
	)
	require.NoError(t, err)
	newState, trieUpdate, err := l.Set(u)
	require.NoError(t, err)
	require.True(t, trieUpdate.IsEmpty())
	require.Equal(t, u.State(), newState)
}
