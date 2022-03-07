// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package merkle

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const TreeTestLength = 100000

var expectedEmptyHash = []byte{
	14, 87, 81, 192, 38, 229, 67,
	178, 232, 171, 46, 176, 96, 153,
	218, 161, 209, 229, 223, 71, 119,
	143, 119, 135, 250, 171, 69, 205,
	241, 47, 227, 168}

// TestTreeInitialization verifies that tree initialization only accepts
// compatible key lengths.
func TestTreeInitialization(t *testing.T) {
	// constructor should reject `keyLength` _outside_ of interval [1, maxKeyLength]
	t.Run("key length outside of compatible bounds", func(t *testing.T) {
		tree, err := NewTree(-1)
		require.Nil(t, tree)
		require.ErrorIs(t, err, ErrorIncompatibleKeyLength)

		tree, err = NewTree(0)
		require.Nil(t, tree)
		require.ErrorIs(t, err, ErrorIncompatibleKeyLength)

		tree, err = NewTree(maxKeyLength + 1)
		require.Nil(t, tree)
		require.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})

	// constructor should accept `keyLength` values in the interval [1, maxKeyLength]
	t.Run("compatible key length", func(t *testing.T) {
		tree, err := NewTree(1)
		require.NotNil(t, tree)
		require.NoError(t, err)

		tree, err = NewTree(maxKeyLength)
		require.NotNil(t, tree)
		require.NoError(t, err)
	})
}

// TestEmptyTreeHash verifies that an empty tree returns the expected empty hash.
// We test with:
//  * different key sizes
//  * a newly initialized trie (empty)
//  * a trie, whose last element was removed
func TestEmptyTreeHash(t *testing.T) {
	for _, keyLength := range []int{1, 32, maxKeyLength} {
		tree, _ := NewTree(keyLength)
		assert.Equal(t, tree.Hash(), expectedEmptyHash)

		// generate random key-value pair
		key := make([]byte, keyLength)
		rand.Read(key)
		val := []byte{1}

		// add key-value pair: hash should be non-empty
		replaced, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, replaced)
		assert.NotEmpty(t, tree.Hash())

		// remove key: hash should now be empty again
		removed := tree.Del(key)
		assert.True(t, removed)
		assert.Equal(t, tree.Hash(), expectedEmptyHash)
	}
}

// Test_ReferenceSingleEntry we construct a tree with a single key-value pair
// and compare its hash to a pre-computed value from a python reference implementation.
func Test_ReferenceSingleEntry(t *testing.T) {
	val, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")

	t.Run("2-byte path", func(t *testing.T) {
		key := []byte{22, 83}                                                                  // key: 00010110 01010011
		expectedRootHash := "3c4fd8e7bc5572d708d7ccab0a9ee06f74aac780e68c68d0b629ecb58a1fdf9d" // from python reference impl

		tree, err := NewTree(len(key))
		assert.NoError(t, err)
		replaced, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, replaced)
		require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
	})

	t.Run("32-byte path", func(t *testing.T) {
		key, _ := hex.DecodeString("1b30482d4dc8c1a8d846d05765c03a33f0267b56b9a7be8defe38958f89c95fc")
		expectedRootHash := "10eb7e9ffa397651acc2faf8a3c56207914418ca02ff9f39694effaf83d261e0" // from python reference impl

		tree, err := NewTree(len(key))
		assert.NoError(t, err)
		replaced, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, replaced)
		require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
	})

	t.Run("maxKeyLength-byte path", func(t *testing.T) {
		// as key, we just repeat the following 32 bytes 256 times
		k, _ := hex.DecodeString("1b30482d4dc8c1a8d846d05765c03a33f0267b56b9a7be8defe38958f89c95fc")
		key := make([]byte, 0, maxKeyLength)
		for i := 1; i <= maxKeyLength/len(k); i++ {
			key = append(key, k...)
		}
		key = append(key, k[:maxKeyLength%len(k)]...)

		expectedRootHash := "bf6eab5ce259b8a936f4fe205ca49f5d6614a7bee4162cafa5a6ab4691eba40d" // from python reference impl
		tree, err := NewTree(len(key))
		assert.NoError(t, err)
		replaced, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, replaced)
		assert.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
	})
}

// Test_2EntryTree we construct a tree with a 2 key-value pairs and compare
// its hash to a pre-computed value from a python reference implementation.
func Test_2EntryTree(t *testing.T) {
	keyLength := 2
	key0 := []byte{20, 3}   // 00010100 00000011
	key1 := []byte{23, 252} // 00010111 11111100
	val0, _ := hex.DecodeString("62b0326507ebce9d4a242908d20559")
	val1, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")
	expectedRootHash := "7f372aca94b91a527539967ba966c3a91c91e97b265fc4830801b4bcca01b06e" // from python reference impl

	tree, err := NewTree(keyLength)
	assert.NoError(t, err)
	replaced, err := tree.Put(key0, val0)
	require.False(t, replaced)
	require.NoError(t, err)
	replaced, err = tree.Put(key1, val1)
	require.False(t, replaced)
	require.NoError(t, err)
	require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
}

// Test_KeyValuesAreSafeFromExternalModification verifies that the
// tree implementation is _not_ vulnerable to the slices of the key-value
// pair being modified in-place _after_ addition to the tree.
func Test_KeyValuesAreSafeFromExternalModification(t *testing.T) {
	// we re-use the same key-value pairs as in Test_2EntryTree:
	keyLength := 2
	key0 := []byte{20, 3}   // 00010100 00000011
	key1 := []byte{23, 252} // 00010111 11111100
	val0, _ := hex.DecodeString("62b0326507ebce9d4a242908d20559")
	val1, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")
	expectedRootHash := "7f372aca94b91a527539967ba966c3a91c91e97b265fc4830801b4bcca01b06e" // from python reference impl

	// we now put the key-value pairs into a tree,
	// but modify the key and value slices right after *in-place*
	postKey := []byte{255, 255}
	postVal, _ := hex.DecodeString("1b30482d4dc8c1a8d846d05765c03a")
	tree, err := NewTree(keyLength)
	assert.NoError(t, err)
	replaced, err := tree.Put(key0, val0)
	require.False(t, replaced)
	require.NoError(t, err)
	copy(key0, postKey)
	copy(val0, postVal)
	replaced, err = tree.Put(key1, val1)
	require.False(t, replaced)
	require.NoError(t, err)
	copy(key1, postKey)
	copy(val1, postVal)

	// (key1, val1) and (key2, val2) should now contain the same data as (postKey, postVal)
	require.Equal(t, postKey, key0)
	require.Equal(t, postVal, val0)
	require.Equal(t, postKey, key1)
	require.Equal(t, postVal, val1)
	// but the tree's root hash should still be the expected value:
	require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
}

// Test_KeyLengthChecked verifies that the Tree implementation checks that
// * the key has the length as configured at construction time
// * rejects addition of key-value pair, if key does not conform to pre-configured length
func Test_KeyLengthChecked(t *testing.T) {
	val, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")

	t.Run("nil key", func(t *testing.T) {
		tree, err := NewTree(1)
		assert.NoError(t, err)
		_, err = tree.Put(nil, val) // nil key is not of length 17 and should be rejected
		assert.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})

	t.Run("empty key", func(t *testing.T) {
		tree, err := NewTree(1)
		assert.NoError(t, err)
		_, err = tree.Put([]byte{}, val) // empty key is not of length 17 and should be rejected
		assert.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})

	t.Run("1-byte key", func(t *testing.T) {
		key := make([]byte, 1)
		tree, err := NewTree(1)
		assert.NoError(t, err)
		replaced, err := tree.Put(key, val) // key has the pre-configured length and should be accepted
		assert.NoError(t, err)
		assert.False(t, replaced)
	})

	t.Run("maxKeyLength-byte key", func(t *testing.T) {
		key := make([]byte, maxKeyLength)
		tree, err := NewTree(maxKeyLength)
		assert.NoError(t, err)
		replaced, err := tree.Put(key, val) // key has the pre-configured length and should be accepted
		assert.NoError(t, err)
		assert.False(t, replaced)
	})

	t.Run("key too long", func(t *testing.T) {
		key := make([]byte, maxKeyLength+1)
		tree, err := NewTree(maxKeyLength)
		assert.NoError(t, err)
		_, err = tree.Put(key, val)
		assert.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})
}

// TestTreeSingle verifies addition, retrieval and deletion operations
// of a _single_ key-value pair to an otherwise empty tree.
func TestTreeSingle(t *testing.T) {
	// initialize the random generator, tree and zero hash
	rand.Seed(time.Now().UnixNano())
	keyLength := 32
	tree, err := NewTree(keyLength)
	assert.NoError(t, err)

	// for the pre-defined number of times...
	for i := 0; i < TreeTestLength; i++ {
		// insert a random key with a random value and make sure it didn't
		// exist yet; collisions are unlikely enough to never happen
		key, val := randomKeyValuePair(keyLength, 128)
		replaced, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, replaced)

		// retrieve the value again, check it as successful and the same
		out, retrieved := tree.Get(key)
		if assert.True(t, retrieved) {
			assert.Equal(t, val, out)
		}

		// delete the value again, check it was successful
		deleted := tree.Del(key)
		assert.True(t, deleted)
		_, retrieved = tree.Get(key)
		assert.False(t, retrieved)

		// get the root hash and make sure it's empty again as the tree is empty
		assert.Equal(t, tree.Hash(), expectedEmptyHash)
	}
}

// TestTreeBatch tests addition and deletion of multiple key-value pairs.
// Key-value pairs are added and deleted in the same order.
func TestTreeBatch(t *testing.T) {
	// initialize random generator, tree, zero hash
	rand.Seed(time.Now().UnixNano())
	keyLength := 32
	tree, err := NewTree(keyLength)
	assert.NoError(t, err)

	// insert a batch of random key-value pairs
	keys := make([][]byte, 0, TreeTestLength)
	vals := make([][]byte, 0, TreeTestLength)
	for i := 0; i < TreeTestLength; i++ {
		key, val := randomKeyValuePair(keyLength, 128)
		keys = append(keys, key)
		vals = append(vals, val)
	}

	// insert key-value pairs and ensure there are no collisions
	for i, key := range keys {
		val := vals[i]
		replaced, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, replaced)
	}

	// retrieve all key-value pairs, ensure they are found and are correct
	for i, key := range keys {
		val := vals[i]
		out, retrieved := tree.Get(key)
		if assert.True(t, retrieved) {
			assert.Equal(t, val, out)
		}
	}

	// remove all key-value pairs, ensure it worked
	for _, key := range keys {
		deleted := tree.Del(key)
		assert.True(t, deleted)
	}

	// get the root hash and make sure it's empty again as the tree is empty
	assert.Equal(t, tree.Hash(), EmptyTreeRootHash)
}

// TestRandomOrder tests that root hash of tree is independent of the order
// in which the elements were added.
func TestRandomOrder(t *testing.T) {
	// initialize random generator, two trees and zero hash
	rand.Seed(time.Now().UnixNano())
	keyLength := 32
	tree1, err := NewTree(keyLength)
	assert.NoError(t, err)
	tree2, err := NewTree(keyLength)
	assert.NoError(t, err)

	// generate the desired number of keys and map a value to each key
	keys := make([][]byte, 0, TreeTestLength)
	vals := make(map[string][]byte)
	for i := 0; i < TreeTestLength; i++ {
		key, val := randomKeyValuePair(32, 128)
		keys = append(keys, key)
		vals[string(key)] = val
	}

	// insert all key-value paris into the first tree
	for _, key := range keys {
		val := vals[string(key)]
		replaced, err := tree1.Put(key, val)
		assert.NoError(t, err)
		require.False(t, replaced)
	}

	// shuffle the keys and insert them with random order into the second tree
	rand.Shuffle(len(keys), func(i int, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})
	for _, key := range keys {
		val := vals[string(key)]
		replaced, err := tree2.Put(key, val)
		assert.NoError(t, err)
		require.False(t, replaced)
	}

	// make sure the tree hashes were the same, in spite of random order
	assert.Equal(t, tree1.Hash(), tree2.Hash())

	// remove the key-value pairs from the first tree in random order
	for _, key := range keys {
		deleted := tree1.Del(key)
		require.True(t, deleted)
	}

	// get the root hash and make sure it's empty again as the tree is empty
	assert.Equal(t, tree1.Hash(), expectedEmptyHash)
}

func BenchmarkTree(b *testing.B) {
	for n := 1000; n < 1000000; n *= 10 {
		b.Run(fmt.Sprintf("put-%d", n), treePut(n))
		b.Run(fmt.Sprintf("get-%d", n), treeGet(n))
		b.Run(fmt.Sprintf("del-%d", n), treeDel(n))
		b.Run(fmt.Sprintf("hash-%d", n), treeHash(n))
	}
}

func randomKeyValuePair(keySize, valueSize int) ([]byte, []byte) {
	key := make([]byte, keySize)
	val := make([]byte, valueSize)
	_, _ = rand.Read(key)
	_, _ = rand.Read(val)
	return key, val
}

func createTree(n int) *Tree {
	t, err := NewTree(32)
	if err != nil {
		panic(err.Error())
	}
	for i := 0; i < n; i++ {
		key, val := randomKeyValuePair(32, 128)
		_, _ = t.Put(key, val)
	}
	return t
}

func treePut(n int) func(*testing.B) {
	return func(b *testing.B) {
		t := createTree(n)
		b.StopTimer()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key, val := randomKeyValuePair(32, 128)
			b.StartTimer()
			_, _ = t.Put(key, val)
			b.StopTimer()
			_ = t.Del(key)
		}
	}
}

func treeGet(n int) func(*testing.B) {
	return func(b *testing.B) {
		t := createTree(n)
		b.StopTimer()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key, val := randomKeyValuePair(32, 128)
			_, _ = t.Put(key, val)
			b.StartTimer()
			_, _ = t.Get(key)
			b.StopTimer()
			_ = t.Del(key)
		}
	}
}

func treeDel(n int) func(*testing.B) {
	return func(b *testing.B) {
		t := createTree(n)
		b.StopTimer()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key, val := randomKeyValuePair(32, 128)
			_, _ = t.Put(key, val)
			b.StartTimer()
			_ = t.Del(key)
			b.StopTimer()
		}
	}
}

func treeHash(n int) func(*testing.B) {
	return func(b *testing.B) {
		t := createTree(n)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = t.Hash()
		}
	}
}
