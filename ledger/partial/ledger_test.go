package partial_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
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
	payloads := utils.KeyValuesToPayloads(keys, values)
	paths, err := pathfinder.KeysToPaths(keys, l.PathFinderVersion())
	require.NoError(t, err)

	update, err := ledger.NewTrieUpdate(state, paths[0:2], payloads[0:2])
	require.NoError(t, err)

	newState, err := l.Set(update)
	require.NoError(t, err)

	query := ledger.NewTrieRead(newState, []ledger.Path{paths[0], paths[1]})
	proof, err := l.Prove(query)
	require.NoError(t, err)

	pled, err := partial.NewLedger(proof, newState, partial.DefaultPathFinderVersion)
	assert.NoError(t, err)
	assert.Equal(t, pled.InitialState(), newState)

	// test batch querying existent keys
	query = ledger.NewTrieRead(newState, paths[0:2])

	retValues, err := pled.Get(query)
	require.NoError(t, err)
	require.Equal(t, 2, len(retValues))
	for i := 0; i < len(retValues); i++ {
		require.Equal(t, values[i], retValues[i])
	}

	// test querying single existent key
	querySingleValue := ledger.NewTrieReadSingleValue(newState, paths[0])

	retValue, err := pled.GetSingleValue(querySingleValue)
	require.NoError(t, err)
	require.Equal(t, values[0], retValue)

	// test batch getting missing keys
	query = ledger.NewTrieRead(newState, paths[1:3])

	retValues, err = pled.Get(query)
	require.Error(t, err)
	require.Nil(t, retValues)

	e, ok := err.(*ledger.ErrMissingPaths)
	require.True(t, ok)
	assert.Equal(t, len(e.Paths), 1)
	require.True(t, e.Paths[0].Equals(paths[2]))

	// test querying single non-existent key
	querySingleValue = ledger.NewTrieReadSingleValue(newState, paths[2])

	retValue, err = pled.GetSingleValue(querySingleValue)
	require.Error(t, err)
	require.Nil(t, retValue)

	e, ok = err.(*ledger.ErrMissingPaths)
	require.True(t, ok)
	assert.Equal(t, len(e.Paths), 1)
	require.True(t, e.Paths[0].Equals(paths[2]))

	// test missing keys (set)
	update, err = ledger.NewTrieUpdate(state, paths[1:3], payloads[1:3])
	require.NoError(t, err)

	_, err = pled.Set(update)
	require.Error(t, err)

	e, ok = err.(*ledger.ErrMissingPaths)
	require.True(t, ok)
	assert.Equal(t, len(e.Paths), 1)
	require.True(t, e.Paths[0].Equals(paths[2]))

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

	registerIDs, values := view.Delta().RegisterUpdates()
	payloads, err := executionState.RegistersToLedgerPayload(registerIDs, values)
	require.NoError(t, err)
	paths, err := executionState.RegisterIDsToLedgerPath(registerIDs, l.PathFinderVersion())
	require.NoError(t, err)

	updated, err := ledger.NewTrieUpdate(
		emptyState,
		paths,
		payloads,
	)
	require.NoError(t, err)

	allRegisters := view.Interactions().AllRegisters()
	allPaths, err := executionState.RegisterIDsToLedgerPath(allRegisters, l.PathFinderVersion())
	require.NoError(t, err)
	newState := updated.State()

	proofQuery := ledger.NewTrieRead(newState, allPaths)

	proof, err := l.Prove(proofQuery)
	require.NoError(t, err)

	pled, err := partial.NewLedger(proof, newState, partial.DefaultPathFinderVersion)
	assert.NoError(t, err)
	assert.Equal(t, pled.InitialState(), emptyState)

	keyID := ledger.KeyID(registerID)
	path, err := pathfinder.KeyIDToPath(keyID, pled.PathFinderVersion())
	require.NoError(t, err)

	query := ledger.NewTrieRead(newState, []ledger.Path{path})

	results, err := pled.Get(query)
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.Empty(t, results[0])

}
