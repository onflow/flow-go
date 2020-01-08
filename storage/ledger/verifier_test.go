package ledger

import (
	"bytes"
	"testing"

	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTrieUntrustedAndVerify(t *testing.T) {
	unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
		f, err := NewTrieStorage(db)
		if err != nil {
			t.Fatal(err)
		}

		keys, values := makeTestKeys()

		newRoot, proofs, err := f.UpdateRegistersWithProof(keys, values)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(f.tree.GetRoot().GetValue(), newRoot) {
			t.Fatalf("Something in UpdateRegister went wrong")
		}

		v := NewTrieVerifier(f.tree.GetHeight(), f.tree.GetDefaultHashes())
		_, err = v.VerifyRegistersProof(keys, f.tree.GetRoot().GetValue(), values, proofs)
		if err != nil {
			t.Fatal(err)
		}
	})
}
