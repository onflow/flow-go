package partial_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		currentState := l.InitialState()
		require.NoError(t, err)
		u := utils.UpdateFixture()
		u.SetState(currentState)

		newState, err := l.Set(u)
		require.NoError(t, err)

		q := utils.QueryFixture()
		q.SetState(newState)
		proof, err := l.Prove(q)
		require.NoError(t, err)

		pled, err := partial.NewLedger(proof, newState)
		assert.NoError(t, err)
		assert.Equal(t, pled.InitialState(), newState)

		// TODO add another update

		// TODO test missing keys
	})
}
