package ledger_test

import (
	"testing"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/ledger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTrieUntrustedAndVerify(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {
		metricsCollector := &metrics.NoopCollector{}
		f, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		if err != nil {
			t.Fatal(err)
		}

		ids, values := makeTestValues()
		stateCommitment := f.EmptyStateCommitment()
		newRoot, proofs, err := f.UpdateRegistersWithProof(ids, values, stateCommitment)
		if err != nil {
			t.Fatal(err)
		}

		v := ledger.NewTrieVerifier(ledger.RegisterKeySize)
		_, err = v.VerifyRegistersProof(ids, values, proofs, newRoot)
		if err != nil {
			t.Fatal(err)
		}
	})
}
