package ledger_test

import (
	"testing"

	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTrieUntrustedAndVerify(t *testing.T) {
	trieH := 257
	unittest.RunWithTempDir(t, func(dbDir string) {
		f, err := ledger.NewTrieStorage(dbDir)
		if err != nil {
			t.Fatal(err)
		}

		ids, values := makeTestValues()
		stateCommitment := f.EmptyStateCommitment()
		newRoot, proofs, err := f.UpdateRegistersWithProof(ids, values, stateCommitment)
		if err != nil {
			t.Fatal(err)
		}

		v := ledger.NewTrieVerifier(trieH)
		_, err = v.VerifyRegistersProof(ids, values, proofs, newRoot)
		if err != nil {
			t.Fatal(err)
		}
	})
}
