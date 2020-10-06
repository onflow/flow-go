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

// func TestNewPartialLedger(t *testing.T) {
// 	p, s := utils.TrieBatchProofFixture()
// 	encp := encoding.EncodeTrieBatchProof(p)
// 	pled, err := partial.NewLedger(encp, s)
// 	assert.NoError(t, err)
// 	assert.Equal(t, pled.InitState(), s)
// }

func TestFunctionalityWithCompleteTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		l, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, zerolog.Logger{}, nil)
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

		pled, err := partial.NewLedger(proof, newState)
		assert.NoError(t, err)
		assert.Equal(t, pled.InitialState(), newState)

		// test missing keys
		query, err = ledger.NewQuery(newState, keys[1:3])
		require.NoError(t, err)

		_, err = pled.Get(query)
		require.Error(t, err)
	})
}
