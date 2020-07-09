package ptrie

// import (
// 	"bytes"
// 	"io/ioutil"
// 	"math/rand"
// 	"os"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/require"

// 	"github.com/dapperlabs/flow-go/ledger/outright/mtrie"
// 	"github.com/dapperlabs/flow-go/ledger/outright/mtrie/proof"
// 	"github.com/dapperlabs/flow-go/ledger/utils"
// 	"github.com/dapperlabs/flow-go/module/metrics"
// )

// func withMForest(
// 	t *testing.T,
// 	keyByteSize int,
// 	numberOfActiveTries int, f func(t *testing.T, mForest *mtrie.MForest)) {

// 	dir, err := ioutil.TempDir("", "test-mtrie-")
// 	require.NoError(t, err)
// 	defer os.RemoveAll(dir)

// 	metricsCollector := &metrics.NoopCollector{}
// 	mForest, err := mtrie.NewMForest(keyByteSize, dir, numberOfActiveTries, metricsCollector, nil)
// 	require.NoError(t, err)

// 	f(t, mForest)
// }

// func TestPartialTrieEmptyTrie(t *testing.T) {

// 	pathByteSize := 1 // path size of 8 bits
// 	withMForest(t, pathByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

// 		// add path1 to the empty trie
// 		path1 := make([]byte, 1) // 00000000 (0)
// 		key1 := []byte{'A'}
// 		value1 := []byte{'a'}

// 		paths := [][]byte{path1}
// 		keys := [][]byte{key1}
// 		values := [][]byte{value1}

// 		rootHash := mForest.GetEmptyRootHash()

// 		retValues, err := mForest.Read(rootHash, paths)
// 		require.NoError(t, err, "error reading values")

// 		bp, err := mForest.Proofs(rootHash, paths)
// 		require.NoError(t, err, "error getting proofs values")

// 		encBP, _ := proof.EncodeBatchProof(bp)
// 		psmt, err := NewPSMT(rootHash, pathByteSize, paths, keys, retValues, encBP)

// 		require.NoError(t, err, "error building partial trie")
// 		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before set]")
// 		}
// 		newTrie, err := mForest.Update(rootHash, paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		rootHash = newTrie.RootHash()

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")

// 		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [after set]")
// 		}

// 		updatedValue1 := []byte{'b'}
// 		values = [][]byte{updatedValue1}

// 		newTrie, err = mForest.Update(rootHash, paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		rootHash = newTrie.RootHash()

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")

// 		if !bytes.Equal(rootHash, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [after update]")
// 		}

// 	})
// }

// func TestPartialTrieLeafUpdates(t *testing.T) {

// 	pathByteSize := 1 // path size of 8 bits
// 	withMForest(t, pathByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

// 		// add path1 to the empty trie
// 		path1 := make([]byte, 1) // 00000000 (0)
// 		key1 := []byte{'A'}
// 		value1 := []byte{'a'}
// 		updatedValue1 := []byte{'b'}

// 		path2 := make([]byte, 1) // 00000001 (1)
// 		utils.SetBit(path2, 7)
// 		key2 := []byte{'C'}
// 		value2 := []byte{'c'}
// 		updatedValue2 := []byte{'d'}

// 		paths := [][]byte{path1, path2}
// 		keys := [][]byte{key1, key2}
// 		values := [][]byte{value1, value2}

// 		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		newRoot := newTrie.RootHash()

// 		retvalues, err := mForest.Read(newRoot, paths)
// 		require.NoError(t, err, "error reading values")

// 		bp, err := mForest.Proofs(newRoot, paths)
// 		require.NoError(t, err, "error getting batch proof")

// 		encBP, _ := proof.EncodeBatchProof(bp)
// 		psmt, err := NewPSMT(newRoot, pathByteSize, paths, keys, retvalues, encBP)
// 		require.NoError(t, err, "error building partial trie")

// 		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before update]")
// 		}

// 		values = [][]byte{updatedValue1, updatedValue2}
// 		newTrie2, err := mForest.Update(newRoot, paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		newRoot2 := newTrie2.RootHash()

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")

// 		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [after update]")
// 		}
// 	})

// }

// func TestPartialTrieMiddleBranching(t *testing.T) {

// 	pathByteSize := 1 // key size of 8 bits
// 	withMForest(t, pathByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {
// 		path1 := make([]byte, 1) // 00000000 (0)
// 		key1 := []byte{'A'}
// 		value1 := []byte{'a'}
// 		updatedValue1 := []byte{'b'}

// 		path2 := make([]byte, 1) // 00000010 (2)
// 		utils.SetBit(path2, 6)
// 		key2 := []byte{'C'}
// 		value2 := []byte{'c'}
// 		updatedValue2 := []byte{'d'}

// 		path3 := make([]byte, 1) // 00001000 (8)
// 		utils.SetBit(path3, 4)
// 		key3 := []byte{'E'}
// 		value3 := []byte{'e'}
// 		updatedValue3 := []byte{'f'}

// 		paths := [][]byte{path1, path2, path3}
// 		keys := [][]byte{key1, key2, key3}
// 		values := [][]byte{value1, value2, value3}

// 		retvalues, err := mForest.Read(mForest.GetEmptyRootHash(), paths)
// 		require.NoError(t, err, "error reading values")

// 		bp, err := mForest.Proofs(mForest.GetEmptyRootHash(), paths)
// 		require.NoError(t, err, "error getting batch proof")

// 		encBP, _ := proof.EncodeBatchProof(bp)
// 		psmt, err := NewPSMT(mForest.GetEmptyRootHash(), pathByteSize, paths, keys, retvalues, encBP)
// 		require.NoError(t, err, "error building partial trie")

// 		if !bytes.Equal(mForest.GetEmptyRootHash(), psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before update]")
// 		}
// 		// first update
// 		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), paths, keys, values)
// 		require.NoError(t, err, "error updating trie")

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")

// 		if !bytes.Equal(newTrie.RootHash(), psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before update]")
// 		}

// 		// second update
// 		values = make([][]byte, 0)
// 		values = append(values, updatedValue1, updatedValue2, updatedValue3)
// 		newTrie2, err := mForest.Update(newTrie.RootHash(), paths, keys, values)
// 		require.NoError(t, err, "error updating trie")

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")

// 		if !bytes.Equal(newTrie2.RootHash(), psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [after update]")
// 		}
// 	})

// }

// func TestPartialTrieRootUpdates(t *testing.T) {
// 	pathByteSize := 1 // path size of 8 bits
// 	withMForest(t, pathByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

// 		path1 := make([]byte, 1) // 00000000 (0)
// 		key1 := []byte{'A'}
// 		value1 := []byte{'a'}
// 		updatedValue1 := []byte{'b'}

// 		path2 := make([]byte, 1) // 10000000 (128)
// 		utils.SetBit(path2, 0)
// 		key2 := []byte{'C'}
// 		value2 := []byte{'c'}
// 		updatedValue2 := []byte{'d'}

// 		paths := [][]byte{path1, path2}
// 		keys := [][]byte{key1, key2}
// 		values := [][]byte{value1, value2}

// 		retvalues, err := mForest.Read(mForest.GetEmptyRootHash(), paths)
// 		require.NoError(t, err, "error reading values")

// 		bp, err := mForest.Proofs(mForest.GetEmptyRootHash(), paths)
// 		require.NoError(t, err, "error getting batch proof")

// 		encBP, _ := proof.EncodeBatchProof(bp)
// 		psmt, err := NewPSMT(mForest.GetEmptyRootHash(), pathByteSize, paths, keys, retvalues, encBP)
// 		require.NoError(t, err, "error building partial trie")

// 		if !bytes.Equal(mForest.GetEmptyRootHash(), psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before update]")
// 		}

// 		// first update
// 		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		newRoot := newTrie.RootHash()

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")
// 		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before update]")
// 		}

// 		// second update
// 		values = make([][]byte, 0)
// 		values = append(values, updatedValue1, updatedValue2)
// 		newTrie2, err := mForest.Update(newRoot, paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		newRoot2 := newTrie2.RootHash()

// 		_, _, err = psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating psmt")
// 		if !bytes.Equal(newRoot2, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [after update]")
// 		}
// 	})

// }

// func TestMixProof(t *testing.T) {
// 	pathByteSize := 1 // path size of 8 bits
// 	withMForest(t, pathByteSize, 10, func(t *testing.T, mForest *mtrie.MForest) {

// 		path1 := make([]byte, 1) // 00000001 (1)
// 		utils.SetBit(path1, 7)
// 		key1 := []byte{'A'}
// 		value1 := []byte{'a'}

// 		path2 := make([]byte, 1) // 00000010 (2)
// 		utils.SetBit(path2, 6)
// 		key2 := []byte{'B'}

// 		path3 := make([]byte, 1) // 00001000 (8)
// 		utils.SetBit(path3, 4)
// 		key3 := []byte{'C'}
// 		value3 := []byte{'c'}

// 		paths := [][]byte{path1, path3}
// 		keys := [][]byte{key1, key3}
// 		values := [][]byte{value1, value3}

// 		newTrie, err := mForest.Update(mForest.GetEmptyRootHash(), paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		newRoot := newTrie.RootHash()

// 		paths = [][]byte{path1, path2, path3}
// 		keys = [][]byte{key1, key2, key3}
// 		retvalues, err := mForest.Read(newRoot, paths)
// 		require.NoError(t, err, "error reading values")

// 		bp, err := mForest.Proofs(newRoot, paths)
// 		require.NoError(t, err, "error getting batch proof")

// 		encBP, _ := proof.EncodeBatchProof(bp)
// 		psmt, err := NewPSMT(newRoot, pathByteSize, paths, keys, retvalues, encBP)
// 		require.NoError(t, err, "error building partial trie")

// 		if !bytes.Equal(newRoot, psmt.root.ComputeValue()) {
// 			t.Fatal("rootNode hash doesn't match [before update]")
// 		}

// 		paths = [][]byte{path2, path3}
// 		keys = [][]byte{key2, key3}
// 		values = [][]byte{[]byte{'X'}, []byte{'Y'}}

// 		trie2, err := mForest.Update(newRoot, paths, keys, values)
// 		require.NoError(t, err, "error updating trie")
// 		root2 := trie2.RootHash()

// 		proot2, _, err := psmt.Update(paths, keys, values)
// 		require.NoError(t, err, "error updating partial trie")

// 		if !bytes.Equal(root2, proot2) {
// 			t.Fatalf("root2 hash doesn't match [%x] != [%x]", root2, proot2)
// 		}

// 	})

// }

// func TestRandomProofs(t *testing.T) {
// 	pathByteSize := 2 // key size of 16 bits
// 	experimentRep := 20
// 	for e := 0; e < experimentRep; e++ {
// 		withMForest(t, pathByteSize, experimentRep+1, func(t *testing.T, mForest *mtrie.MForest) {

// 			// insert some values to an empty trie
// 			paths := make([][]byte, 0)
// 			keys := make([][]byte, 0)
// 			values := make([][]byte, 0)
// 			rand.Seed(time.Now().UnixNano())

// 			numberOfPaths := rand.Intn(256) + 1
// 			alreadySelectPaths := make(map[string]bool)
// 			i := 0
// 			for i < numberOfPaths {
// 				path := make([]byte, 2)
// 				rand.Read(path)
// 				// deduplicate
// 				if _, found := alreadySelectPaths[string(path)]; !found {
// 					paths = append(paths, path)
// 					alreadySelectPaths[string(path)] = true
// 					key := make([]byte, 4)
// 					rand.Read(key)
// 					keys = append(keys, key)
// 					value := make([]byte, 4)
// 					rand.Read(value)
// 					values = append(values, value)
// 					i++
// 				}
// 			}

// 			// keep a subset as initial insert and keep the rest as default value read
// 			split := rand.Intn(numberOfPaths)
// 			insertPaths := paths[:split]
// 			insertKeys := keys[:split]
// 			insertValues := values[:split]

// 			nTrie, err := mForest.Update(mForest.GetEmptyRootHash(), insertPaths, insertKeys, insertValues)
// 			require.NoError(t, err, "error updating trie")
// 			root := nTrie.RootHash()

// 			// shuffle paths for read
// 			rand.Shuffle(len(paths), func(i, j int) {
// 				paths[i], paths[j] = paths[j], paths[i]
// 				keys[i], keys[j] = keys[j], keys[i]
// 				values[i], values[j] = values[j], values[i]
// 			})

// 			retvalues, err := mForest.Read(root, paths)
// 			require.NoError(t, err, "error reading values")

// 			bp, err := mForest.Proofs(root, paths)
// 			require.NoError(t, err, "error getting batch proof")

// 			encBP, _ := proof.EncodeBatchProof(bp)
// 			psmt, err := NewPSMT(root, pathByteSize, paths, keys, retvalues, encBP)
// 			require.NoError(t, err, "error building partial trie")

// 			if !bytes.Equal(root, psmt.root.ComputeValue()) {
// 				t.Fatal("root hash doesn't match")
// 			}

// 			// select a subset of shuffled paths for random updates
// 			split = rand.Intn(numberOfPaths)
// 			updatePaths := paths[:split]
// 			updateKeys := keys[:split]
// 			updateValues := values[:split]
// 			// random updates
// 			rand.Shuffle(len(updateValues), func(i, j int) {
// 				updateValues[i], updateValues[j] = updateValues[j], updateValues[i]
// 			})

// 			newTrie2, err := mForest.Update(root, updatePaths, updateKeys, updateValues)
// 			require.NoError(t, err, "error updating trie")
// 			root2 := newTrie2.RootHash()

// 			proot2, _, err := psmt.Update(updatePaths, updateKeys, updateValues)
// 			require.NoError(t, err, "error updating partial trie")

// 			if !bytes.Equal(root2, proot2) {
// 				t.Fatalf("root2 hash doesn't match [%x] != [%x]", root2, proot2)
// 			}

// 		})
// 	}
// }

// // // TODO add test for incompatible proofs [Byzantine milestone]
// // // TODO add test key not exist [Byzantine milestone]
