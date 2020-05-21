package ptrie

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

func withMForest(
	t *testing.T,
	trieHeight int,
	numberOfActiveTries int, f func(t *testing.T, mForest *mtrie.MForest)) {

	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	mForest, err := mtrie.NewMForest(trieHeight, dir, numberOfActiveTries, nil)
	require.NoError(t, err)

	f(t, mForest)
}

func TestPartialTrieEmptyTrie(t *testing.T) {

	trieHeight := 9 // should be key size (in bits) + 1
	withMForest(t, trieHeight, 10, func(t *testing.T, mForest *mtrie.MForest) {

		// add key1 and value1 to the empty trie
		key1 := make([]byte, 1) // 00000000 (0)
		value1 := []byte{'a'}
		updatedValue1 := []byte{'A'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1)
		values = append(values, value1)

		rootHash := mForest.GetEmptyRootHash()

		retValues, err := mForest.Read(keys, rootHash)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(keys, rootHash)
		require.NoError(t, err, "error getting proofs values")

		psmt, err := NewPSMT(rootHash, trieHeight, keys, retValues, bp.EncodeBatchProof())

		require.NoError(t, err, "error building partial trie")
		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before set]")
		}
		rootHash, err = mForest.Update(keys, values, rootHash)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after set]")
		}

		keys = make([][]byte, 0)
		values = make([][]byte, 0)
		keys = append(keys, key1)
		values = append(values, updatedValue1)

		rootHash, err = mForest.Update(keys, values, rootHash)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}

	})
}

func TestPartialTrieLeafUpdates(t *testing.T) {

	trieHeight := 9 // should be key size (in bits) + 1
	withMForest(t, trieHeight, 10, func(t *testing.T, mForest *mtrie.MForest) {

		// add key1 and value1 to the empty trie
		key1 := make([]byte, 1) // 00000000 (0)
		value1 := []byte{'a'}
		updatedValue1 := []byte{'A'}

		key2 := make([]byte, 1) // 00000001 (1)
		utils.SetBit(key2, 7)
		value2 := []byte{'b'}
		updatedValue2 := []byte{'B'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1, key2)
		values = append(values, value1, value2)

		newRoot, err := mForest.Update(keys, values, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error updating trie")

		retvalues, err := mForest.Read(keys, newRoot)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(keys, newRoot)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(newRoot, trieHeight, keys, retvalues, bp.EncodeBatchProof())
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2)
		newRoot2, err := mForest.Update(keys, values, newRoot)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieMiddleBranching(t *testing.T) {

	trieHeight := 9 // should be key size (in bits) + 1
	withMForest(t, trieHeight, 10, func(t *testing.T, mForest *mtrie.MForest) {
		key1 := make([]byte, 1) // 00000000 (0)
		value1 := []byte{'a'}
		updatedValue1 := []byte{'A'}

		key2 := make([]byte, 1) // 00000010 (2)
		utils.SetBit(key2, 6)
		value2 := []byte{'b'}
		updatedValue2 := []byte{'B'}

		key3 := make([]byte, 1) // 00001000 (8)
		utils.SetBit(key3, 4)
		value3 := []byte{'c'}
		updatedValue3 := []byte{'C'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1, key2, key3)
		values = append(values, value1, value2, value3)

		retvalues, err := mForest.Read(keys, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(keys, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(mForest.GetEmptyRootHash(), trieHeight, keys, retvalues, bp.EncodeBatchProof())
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(mForest.GetEmptyRootHash(), psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}
		// first update
		newRoot, err := mForest.Update(keys, values, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// second update
		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2, updatedValue3)
		newRoot2, err := mForest.Update(keys, values, newRoot)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieRootUpdates(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1
	withMForest(t, trieHeight, 10, func(t *testing.T, mForest *mtrie.MForest) {

		key1 := make([]byte, 1) // 00000000 (0)
		value1 := []byte{'a'}
		updatedValue1 := []byte{'A'}

		key2 := make([]byte, 1) // 10000000 (128)
		utils.SetBit(key2, 0)
		value2 := []byte{'b'}
		updatedValue2 := []byte{'B'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1, key2)
		values = append(values, value1, value2)

		retvalues, err := mForest.Read(keys, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(keys, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(mForest.GetEmptyRootHash(), trieHeight, keys, retvalues, bp.EncodeBatchProof())
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(mForest.GetEmptyRootHash(), psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// first update
		newRoot, err := mForest.Update(keys, values, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// second update
		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2)
		newRoot2, err := mForest.Update(keys, values, newRoot)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestMixProof(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1
	withMForest(t, trieHeight, 10, func(t *testing.T, mForest *mtrie.MForest) {
		key1 := make([]byte, 1) // 00000001 (1)
		utils.SetBit(key1, 7)
		value1 := []byte{'a'}

		key2 := make([]byte, 1) // 00000010 (2)
		utils.SetBit(key2, 6)

		key3 := make([]byte, 1) // 00001000 (8)
		utils.SetBit(key3, 4)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1, key3)
		values = append(values, value1, value3)

		newRoot, err := mForest.Update(keys, values, mForest.GetEmptyRootHash())
		require.NoError(t, err, "error updating trie")

		keys = make([][]byte, 0)
		keys = append(keys, key1, key2, key3)

		retvalues, err := mForest.Read(keys, newRoot)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(keys, newRoot)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(newRoot, trieHeight, keys, retvalues, bp.EncodeBatchProof())
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		keys = make([][]byte, 0)
		keys = append(keys, key2, key3)

		values = make([][]byte, 0)
		values = append(values, []byte{'X'}, []byte{'Y'})

		root2, err := mForest.Update(keys, values, newRoot)
		require.NoError(t, err, "error updating trie")

		proot2, _, err := psmt.Update(keys, values)
		require.NoError(t, err, "error updating partial trie")

		if !bytes.Equal(root2, proot2) {
			t.Fatalf("root2 hash doesn't match [%x] != [%x]", root2, proot2)
		}

	})

}

func TestRandomProofs(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	experimentRep := 20
	for e := 0; e < experimentRep; e++ {
		withMForest(t, trieHeight, experimentRep+1, func(t *testing.T, mForest *mtrie.MForest) {

			// insert some values to an empty trie
			keys := make([][]byte, 0)
			values := make([][]byte, 0)
			rand.Seed(time.Now().UnixNano())

			numberOfKeys := rand.Intn(256) + 1
			if numberOfKeys == 0 {
				numberOfKeys = 1
			}

			alreadySelectKeys := make(map[string]bool)
			i := 0
			for i < numberOfKeys {
				key := make([]byte, 2)
				rand.Read(key)
				// deduplicate
				if _, found := alreadySelectKeys[string(key)]; !found {
					keys = append(keys, key)
					alreadySelectKeys[string(key)] = true
					value := make([]byte, 4)
					rand.Read(value)
					values = append(values, value)
					i++
				}
			}

			// keep a subset as initial insert and keep the rest as default value read
			split := rand.Intn(numberOfKeys)
			insertKeys := keys[:split]
			insertValues := values[:split]

			root, err := mForest.Update(insertKeys, insertValues, mForest.GetEmptyRootHash())
			require.NoError(t, err, "error updating trie")

			// shuffle keys for read
			rand.Shuffle(len(keys), func(i, j int) {
				keys[i], keys[j] = keys[j], keys[i]
				values[i], values[j] = values[j], values[i]
			})

			retvalues, err := mForest.Read(keys, root)
			require.NoError(t, err, "error reading values")

			bp, err := mForest.Proofs(keys, root)
			require.NoError(t, err, "error getting batch proof")

			psmt, err := NewPSMT(root, trieHeight, keys, retvalues, bp.EncodeBatchProof())
			require.NoError(t, err, "error building partial trie")

			if !bytes.Equal(root, psmt.root.ComputeValue()) {
				t.Fatal("root hash doesn't match")
			}

			// select a subset of shuffled keys for random updates
			split = rand.Intn(numberOfKeys)
			updateKeys := keys[:split]
			updateValues := values[:split]
			// random updates
			rand.Shuffle(len(updateValues), func(i, j int) {
				updateValues[i], updateValues[j] = updateValues[j], updateValues[i]
			})

			root2, err := mForest.Update(updateKeys, updateValues, root)
			require.NoError(t, err, "error updating trie")

			proot2, _, err := psmt.Update(updateKeys, updateValues)
			require.NoError(t, err, "error updating partial trie")

			if !bytes.Equal(root2, proot2) {
				t.Fatalf("root2 hash doesn't match [%x] != [%x]", root2, proot2)
			}

		})
	}
}

// // TODO add test for incompatible proofs [Byzantine milestone]
// // TODO add test key not exist [Byzantine milestone]
