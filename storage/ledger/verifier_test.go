package ledger

// // TODO (Ramtin) this is going to be updated in the next PRs
// import (
// 	"bytes"
// 	"testing"

// 	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
// 	"github.com/dapperlabs/flow-go/storage/ledger/trie"
// 	"github.com/dapperlabs/flow-go/utils/unittest"
// )

// func TestTrieUntrustedAndVerify(t *testing.T) {
// 	unittest.RunWithLevelDB(t, func(db *leveldb.LevelDB) {
// 		f, err := NewTrieStorage(db)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		ids, values := makeTestValues()
// 		newRoot, proofs, err := f.UpdateRegistersWithProof(ids, values)
// 		if err != nil {
// 			t.Fatal(err)
// 		}

// 		if !bytes.Equal(f.tree.GetRoot().GetValue(), newRoot) {
// 			t.Fatalf("Something in UpdateRegister went wrong")
// 		}

// 		v := NewTrieVerifier(f.tree.GetHeight(), trie.GetDefaultHashes())
// 		_, err = v.VerifyRegistersProof(ids, f.tree.GetRoot().GetValue(), values, proofs)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 	})
// }
