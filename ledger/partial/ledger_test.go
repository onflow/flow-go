package partial_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/partial"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFunctionalityWithCompleteTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		l, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil, complete.DefaultPathFinderVersion, complete.DefaultHasherVersion)
		require.NoError(t, err)

		// create empty update
		state := l.InitialState()
		keys := utils.RandomUniqueKeys(3, 2, 2, 4)
		values := utils.RandomValues(3, 1, 32)
		update, err := ledger.NewUpdate(state, keys[0:2], values[0:2])
		require.NoError(t, err)

		newState, err := l.Set(update)
		require.NoError(t, err)

		query, err := ledger.NewQuery(newState, keys[0:2])
		require.NoError(t, err)
		proof, err := l.Prove(query)
		require.NoError(t, err)

		pled, err := partial.NewLedger(proof, newState, partial.DefaultPathFinderVersion, partial.DefaultHasherVersion)
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

		_, err = pled.Set(update)
		require.Error(t, err)

		e, ok = err.(*ledger.ErrMissingKeys)
		require.True(t, ok)
		assert.Equal(t, len(e.Keys), 1)
		require.True(t, e.Keys[0].Equals(&keys[2]))

	})
}
