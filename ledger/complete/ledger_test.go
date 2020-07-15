package complete_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestNewLedger(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {
		metricsCollector := &metrics.NoopCollector{}
		_, err := complete.NewLedger(dbDir, 100, metricsCollector, nil)
		assert.NoError(t, err)
	})
}

func TestLedger_Update(t *testing.T) {
	t.Run("empty update", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {

			l, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			// create empty update
			currentCommitment := l.EmptyStateCommitment()
			up, err := ledger.NewEmptyUpdate(currentCommitment)
			require.NoError(t, err)

			newCommitment, err := l.Set(up)
			require.NoError(t, err)

			// state commitment shouldn't change
			assert.True(t, bytes.Equal(currentCommitment, newCommitment))
		})
	})

	t.Run("non-empty update and query", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			// UpdateFixture

			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			curSC := led.EmptyStateCommitment()

			u := common.UpdateFixture()
			u.SetStateCommitment(curSC)

			newSc, err := led.Set(u)
			require.NoError(t, err)
			assert.False(t, bytes.Equal(curSC, newSc))

			q, err := ledger.NewQuery(newSc, u.Keys())
			require.NoError(t, err)

			retValues, err := led.Get(q)
			require.NoError(t, err)

			for i, v := range u.Values() {
				assert.Equal(t, v, retValues[i])
			}
		})
	})
}

func TestLedger_Get(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			curSC := led.EmptyStateCommitment()
			q, err := ledger.NewEmptyQuery(curSC)
			require.NoError(t, err)

			retValues, err := led.Get(q)
			require.NoError(t, err)
			assert.Equal(t, len(retValues), 0)
		})
	})

	t.Run("empty keys", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			curSC := led.EmptyStateCommitment()

			q := common.QueryFixture()
			q.SetStateCommitment(curSC)

			retValues, err := led.Get(q)
			require.NoError(t, err)

			assert.Equal(t, 2, len(retValues))
			assert.Equal(t, 0, len(retValues[0]))
			assert.Equal(t, 0, len(retValues[1]))
		})
	})
}

func TestLedger_Proof(t *testing.T) {
	t.Run("empty query", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			curSC := led.EmptyStateCommitment()
			q, err := ledger.NewEmptyQuery(curSC)
			require.NoError(t, err)

			retProof, err := led.Prove(q)
			require.NoError(t, err)

			proof, err := common.DecodeTrieBatchProof(retProof)
			require.NoError(t, err)
			assert.Equal(t, 0, len(proof.Proofs))
		})
	})

	t.Run("non-existing keys", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			curSC := led.EmptyStateCommitment()
			q := common.QueryFixture()
			q.SetStateCommitment(curSC)
			require.NoError(t, err)

			retProof, err := led.Prove(q)
			require.NoError(t, err)

			proof, err := common.DecodeTrieBatchProof(retProof)
			require.NoError(t, err)
			assert.Equal(t, 2, len(proof.Proofs))
			assert.True(t, common.VerifyTrieBatchProof(proof, curSC))
		})
	})

	t.Run("existing keys", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dbDir string) {
			led, err := complete.NewLedger(dbDir, 100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			curSC := led.EmptyStateCommitment()

			u := common.UpdateFixture()
			u.SetStateCommitment(curSC)

			newSc, err := led.Set(u)
			require.NoError(t, err)
			assert.False(t, bytes.Equal(curSC, newSc))

			q, err := ledger.NewQuery(newSc, u.Keys())
			require.NoError(t, err)

			retProof, err := led.Prove(q)
			require.NoError(t, err)

			proof, err := common.DecodeTrieBatchProof(retProof)
			require.NoError(t, err)
			assert.Equal(t, 2, len(proof.Proofs))
			assert.True(t, common.VerifyTrieBatchProof(proof, newSc))
		})
	})
}
