package ptrie

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/ledger/mtrie"
	"github.com/onflow/flow-go/storage/ledger/mtrie/proof"
	"github.com/onflow/flow-go/storage/ledger/utils"
)

func withMForest(
	t *testing.T,
	keyByteSize int,
	numberOfActiveTries int, f func(t *testing.T, mForest *mtrie.MForest)) {

	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	mForest, err := mtrie.NewMForest(keyByteSize, dir, numberOfActiveTries, metricsCollector, nil)
	require.NoError(t, err)

	f(t, mForest)
}

func TestPartialTrieEmptyTrie(t *testing.T) {

	keyByteSize := 1 // key size of 8 bits
	withMForest(t, keyByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

		// add key1 and value1 to the empty trie
		key1 := make([]byte, 1) // 00000000 (0)
		value1 := []byte{'a'}
		updatedValue1 := []byte{'A'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1)
		values = append(values, value1)

		rootHash := mForest.GetEmptyRootHash()

		retValues, err := mForest.Read(rootHash, keys)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(rootHash, keys)
		require.NoError(t, err, "error getting proofs values")

		encBP, _ := proof.EncodeBatchProof(bp)
		psmt, err := NewPSMT(rootHash, keyByteSize, keys, retValues, encBP)

		require.NoError(t, err, "error building partial trie")
		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before set]")
		}
		newTrie, err := mForest.Update(rootHash, keys, values)
		require.NoError(t, err, "error updating trie")
		rootHash = newTrie.RootHash()

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after set]")
		}

		keys = make([][]byte, 0)
		values = make([][]byte, 0)
		keys = append(keys, key1)
		values = append(values, updatedValue1)

		newTrie, err = mForest.Update(rootHash, keys, values)
		require.NoError(t, err, "error updating trie")
		rootHash = newTrie.RootHash()

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}

	})
}

func TestPartialTrieLeafUpdates(t *testing.T) {

	keyByteSize := 1 // key size of 8 bits
	withMForest(t, keyByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

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

		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), keys, values)
		require.NoError(t, err, "error updating trie")
		newRoot := newTrie.RootHash()

		retvalues, err := mForest.Read(newRoot, keys)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(newRoot, keys)
		require.NoError(t, err, "error getting batch proof")

		encBP, _ := proof.EncodeBatchProof(bp)
		psmt, err := NewPSMT(newRoot, keyByteSize, keys, retvalues, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2)
		newTrie2, err := mForest.Update(newRoot, keys, values)
		require.NoError(t, err, "error updating trie")
		newRoot2 := newTrie2.RootHash()

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieMiddleBranching(t *testing.T) {

	keyByteSize := 1 // key size of 8 bits
	withMForest(t, keyByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {
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

		retvalues, err := mForest.Read(mForest.GetEmptyRootHash(), keys)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(mForest.GetEmptyRootHash(), keys)
		require.NoError(t, err, "error getting batch proof")

		encBP, _ := proof.EncodeBatchProof(bp)
		psmt, err := NewPSMT(mForest.GetEmptyRootHash(), keyByteSize, keys, retvalues, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(mForest.GetEmptyRootHash(), psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}
		// first update
		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), keys, values)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newTrie.RootHash(), psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// second update
		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2, updatedValue3)
		newTrie2, err := mForest.Update(newTrie.RootHash(), keys, values)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newTrie2.RootHash(), psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieRootUpdates(t *testing.T) {
	keyByteSize := 1 // key size of 8 bits
	withMForest(t, keyByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

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

		retvalues, err := mForest.Read(mForest.GetEmptyRootHash(), keys)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(mForest.GetEmptyRootHash(), keys)
		require.NoError(t, err, "error getting batch proof")

		encBP, _ := proof.EncodeBatchProof(bp)
		psmt, err := NewPSMT(mForest.GetEmptyRootHash(), keyByteSize, keys, retvalues, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(mForest.GetEmptyRootHash(), psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// first update
		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), keys, values)
		require.NoError(t, err, "error updating trie")
		newRoot := newTrie.RootHash()

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// second update
		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2)
		newTrie2, err := mForest.Update(newRoot, keys, values)
		require.NoError(t, err, "error updating trie")
		newRoot2 := newTrie2.RootHash()

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestMixProof(t *testing.T) {
	keyByteSize := 1 // key size of 8 bits
	withMForest(t, keyByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {
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

		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), keys, values)
		require.NoError(t, err, "error updating trie")
		newRoot := newTrie.RootHash()

		keys = make([][]byte, 0)
		keys = append(keys, key1, key2, key3)

		retvalues, err := mForest.Read(newRoot, keys)
		require.NoError(t, err, "error reading values")

		bp, err := mForest.Proofs(newRoot, keys)
		require.NoError(t, err, "error getting batch proof")

		encBP, _ := proof.EncodeBatchProof(bp)
		psmt, err := NewPSMT(newRoot, keyByteSize, keys, retvalues, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		keys = make([][]byte, 0)
		keys = append(keys, key2, key3)

		values = make([][]byte, 0)
		values = append(values, []byte{'X'}, []byte{'Y'})

		trie2, err := mForest.Update(newRoot, keys, values)
		require.NoError(t, err, "error updating trie")
		root2 := trie2.RootHash()

		proot2, _, err := psmt.Update(keys, values)
		require.NoError(t, err, "error updating partial trie")

		if !bytes.Equal(root2, proot2) {
			t.Fatalf("root2 hash doesn't match [%x] != [%x]", root2, proot2)
		}

	})

}

func TestRandomProofs(t *testing.T) {
	keyByteSize := 2 // key size of 16 bits
	experimentRep := 20
	for e := 0; e < experimentRep; e++ {
		withMForest(t, keyByteSize, experimentRep+1, func(t *testing.T, mForest *mtrie.MForest) {

			// insert some values to an empty trie
			keys := make([][]byte, 0)
			values := make([][]byte, 0)
			rand.Seed(time.Now().UnixNano())

			numberOfKeys := rand.Intn(256) + 1
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

			nTrie, err := mForest.Update(mForest.GetEmptyRootHash(), insertKeys, insertValues)
			require.NoError(t, err, "error updating trie")
			root := nTrie.RootHash()

			// shuffle keys for read
			rand.Shuffle(len(keys), func(i, j int) {
				keys[i], keys[j] = keys[j], keys[i]
				values[i], values[j] = values[j], values[i]
			})

			retvalues, err := mForest.Read(root, keys)
			require.NoError(t, err, "error reading values")

			bp, err := mForest.Proofs(root, keys)
			require.NoError(t, err, "error getting batch proof")

			encBP, _ := proof.EncodeBatchProof(bp)
			psmt, err := NewPSMT(root, keyByteSize, keys, retvalues, encBP)
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

			newTrie2, err := mForest.Update(root, updateKeys, updateValues)
			require.NoError(t, err, "error updating trie")
			root2 := newTrie2.RootHash()

			proot2, _, err := psmt.Update(updateKeys, updateValues)
			require.NoError(t, err, "error updating partial trie")

			if !bytes.Equal(root2, proot2) {
				t.Fatalf("root2 hash doesn't match [%x] != [%x]", root2, proot2)
			}

		})
	}
}

// TODO add test for incompatible proofs [Byzantine milestone]
// TODO add test key not exist [Byzantine milestone]
