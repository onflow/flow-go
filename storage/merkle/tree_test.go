// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package merkle

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jrick/bitset"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"
)

const TreeTestLength = 100000

// TestEmptyTreeHash verifies that an empty tree has the expected hash value.
// Convention: Root hash equals to zero-state of blake2b hasher
func TestEmptyTreeHash(t *testing.T) {
	// zero-state of blake2b hasher
	b, _ := blake2b.New256(nil)
	zeroBlake := b.Sum(nil)

	// reference value (from python reference implementation)
	ref := "0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8"
	require.Equal(t, ref, hex.EncodeToString(zeroBlake))

	// compare with tree
	assert.Equal(t, zeroBlake, NewTree().Hash())
}

// Test_ReferenceSingleEntry we construct a tree with a single key-value pair
// and compare its hash to a pre-computed value from a python reference implementation.
func Test_ReferenceSingleEntry(t *testing.T) {
	val, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")

	t.Run("2-byte path", func(t *testing.T) {
		key := []byte{22, 83}                                                                  // key: 00010110 01010011
		expectedRootHash := "3c4fd8e7bc5572d708d7ccab0a9ee06f74aac780e68c68d0b629ecb58a1fdf9d" // from python reference impl

		tree := NewTree()
		existed, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, existed)
		require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
	})

	t.Run("32-byte path", func(t *testing.T) {
		key, _ := hex.DecodeString("1b30482d4dc8c1a8d846d05765c03a33f0267b56b9a7be8defe38958f89c95fc")
		expectedRootHash := "10eb7e9ffa397651acc2faf8a3c56207914418ca02ff9f39694effaf83d261e0" // from python reference impl

		tree := NewTree()
		existed, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, existed)
		require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
	})

	t.Run("8192-byte path", func(t *testing.T) {
		// for a key, we just repeat the following 32 bytes 256 times
		k, _ := hex.DecodeString("1b30482d4dc8c1a8d846d05765c03a33f0267b56b9a7be8defe38958f89c95fc")
		key := make([]byte, 0, 8192)
		for i := 1; i <= 256; i++ {
			key = append(key, k...)
		}

		expectedRootHash := "80ae4aaff2f9cc82e56968db6c313a578b07723701e6fd745f256b30fac496bf" // from python reference impl
		tree := NewTree()
		existed, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, existed)
		require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
	})
}

// Test_2EntryTree we construct a tree with a 2 key-value pairs and compare
// its hash to a pre-computed value from a python reference implementation.
func Test_2EntryTree(t *testing.T) {

	key0 := []byte{uint8(20), uint8(3)}   // 00010100 00000011
	key1 := []byte{uint8(23), uint8(252)} // 00010111 11111100
	val0, _ := hex.DecodeString("62b0326507ebce9d4a242908d20559")
	val1, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")
	expectedRootHash := "7f372aca94b91a527539967ba966c3a91c91e97b265fc4830801b4bcca01b06e" // from python reference impl

	fmt.Println()
	fmt.Println()
	k, _ := hex.DecodeString("1400")
	for i := 0; i < 16; i++ {
		if bitset.Bytes(k).Get(i) {
			fmt.Print(1)
		} else {
			fmt.Print(0)
		}
	}
	fmt.Println()

	for i := 0; i < 16; i++ {
		if bitset.Bytes(key0).Get(i) {
			fmt.Print(1)
		} else {
			fmt.Print(0)
		}
	}
	fmt.Println()
	for i := 0; i < 16; i++ {
		if bitset.Bytes(key1).Get(i) {
			fmt.Print(1)
		} else {
			fmt.Print(0)
		}
	}
	fmt.Println()

	tree := NewTree()
	tree.Put(key0, val0)
	tree.Put(key1, val1)

	require.Equal(t, expectedRootHash, hex.EncodeToString(tree.Hash()))
}

// Test_KeyValuesAreSafeFromExternalModification verifies that the
// tree implementation is _not_ vulnerable to the slices of the key-value
// pair being modified in-place _after_ addition to the tree.
func Test_KeyValuesAreSafeFromExternalModification(t *testing.T) {
	// random key-value pairs (collisions are unlikely enough to never happen)
	key1, val1 := randomKeyValuePair(32, 128)
	key2, val2 := randomKeyValuePair(32, 128)
	postKey, postVal := randomKeyValuePair(32, 128)

	// completely separate tree for reference
	referenceTree := NewTree()
	referenceTree.Put(key1, val1)
	referenceTree.Put(key2, val2)
	refHash := append([]byte{}, referenceTree.Hash()...) // copies hash
	fmt.Println(refHash)

	// put the same key-value pairs into a separate instance,
	// but overwrite the slices _afterwards_
	tree := NewTree()
	tree.Put(key1, val1)
	copy(key1, postKey)
	copy(val1, postVal)
	tree.Put(key2, val2)
	copy(key2, postKey)
	copy(val2, postVal)
	// (key1, val1) and (key2, val2) should now contain the same data as (postKey, postVal)
	require.Equal(t, postKey, key1)
	require.Equal(t, postVal, val1)
	require.Equal(t, postKey, key2)
	require.Equal(t, postVal, val2)

	require.Equal(t, refHash, tree.Hash())
}

// Test_KeyLengthChecked verifies that the Tree implementation checks that
// * the key length is between 1 and 8192 bytes
// * all keys have the4 same length
func Test_KeyLengthChecked(t *testing.T) {
	val, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")

	t.Run("nil key", func(t *testing.T) {
		tree := NewTree()
		_, err := tree.Put(nil, val)
		assert.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})

	t.Run("empty key", func(t *testing.T) {
		tree := NewTree()
		_, err := tree.Put([]byte{}, val)
		assert.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})

	t.Run("1-byte key", func(t *testing.T) {
		key := make([]byte, 1)
		tree := NewTree()
		_, err := tree.Put(key, val)
		assert.NoError(t, err)
	})

	t.Run("8192-byte key", func(t *testing.T) {
		key := make([]byte, maxKeyLength)
		tree := NewTree()
		_, err := tree.Put(key, val)
		assert.NoError(t, err)
	})

	t.Run("key too long", func(t *testing.T) {
		key := make([]byte, maxKeyLength+1)
		tree := NewTree()
		_, err := tree.Put(key, val)
		assert.ErrorIs(t, err, ErrorIncompatibleKeyLength)
	})
}

// Test_DifferentLengthKeys verifies that the Tree refuses to add entries
// with different key length
func Test_DifferentLengthKeys(t *testing.T) {
	key1 := make([]byte, 17) // 00...0
	val1, _ := hex.DecodeString("bab02e6213dfad3546aa473922bba0")

	key2 := make([]byte, 18)
	key2[0] = byte(1) // 10...0
	val2, _ := hex.DecodeString("62b0326507ebce9d4a242908d20559")

	tree := NewTree()
	_, err := tree.Put(key1, val1)
	assert.NoError(t, err)
	_, err = tree.Put(key2, val2)
	assert.ErrorIs(t, err, ErrorVariableKeyLengths)
}

// Test_Size checks the tree's size computation
func Test_Size(t *testing.T) {
	// random key-value pairs (collisions are unlikely enough to never happen)
	key1, val1 := randomKeyValuePair(32, 128)
	key2, val2a := randomKeyValuePair(32, 128)
	_, val2b := randomKeyValuePair(32, 128)
	key3, val3 := randomKeyValuePair(32, 128)
	unknownKey, _ := randomKeyValuePair(32, 128)

	tree := NewTree()
	replaced, err := tree.Put(key1, val1) // add key-value pair with new key
	assert.NoError(t, err)
	assert.False(t, replaced)
	assert.Equal(t, uint64(1), tree.Size())

	replaced, err = tree.Put(key2, val2a) // add key-value pair with new key
	assert.NoError(t, err)
	assert.False(t, replaced)
	assert.Equal(t, uint64(2), tree.Size())

	replaced, err = tree.Put(key2, val2b) // overwrite value of existing key
	assert.NoError(t, err)
	assert.True(t, replaced)
	assert.Equal(t, uint64(2), tree.Size())

	replaced, err = tree.Put(key3, val3) // add key-value pair with new key
	assert.NoError(t, err)
	assert.False(t, replaced)
	assert.Equal(t, uint64(3), tree.Size())

	removed := tree.Del(key2) // remove existing key-value pair
	assert.True(t, removed)
	assert.Equal(t, uint64(2), tree.Size())

	removed = tree.Del(unknownKey) // remove unknown key-value pair
	assert.False(t, removed)
	assert.Equal(t, uint64(2), tree.Size())

	removed = tree.Del(key1) // remove existing key-value pair
	assert.True(t, removed)
	assert.Equal(t, uint64(1), tree.Size())

	removed = tree.Del(key1) // repeated removal should be no-op
	assert.False(t, removed)
	assert.Equal(t, uint64(1), tree.Size())

	removed = tree.Del(key3) // remove existing key-value pair
	assert.True(t, removed)
	assert.Equal(t, uint64(0), tree.Size())
}

// TestTreeSingle verifies addition, retrieval and deletion operations
// of a _single_ key-value pair to an otherwise empty tree.
func TestTreeSingle(t *testing.T) {

	// initialize the random generator, tree and zero hash
	rand.Seed(time.Now().UnixNano())
	tree := NewTree()
	zero := tree.Hash()

	// for the pre-defined number of times...
	for i := 0; i < TreeTestLength; i++ {

		// insert a random key with a random value and make sure it didn't
		// exist yet; collisions are unlikely enough to never happen
		key, val := randomKeyValuePair(32, 128)
		existed, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, existed)

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

		// get the root hash and make sure it's the zero hash again
		hash := tree.Hash()
		assert.Equal(t, zero, hash)
	}
}

func TestTreeBatch(t *testing.T) {

	// initialize random generator, tree, zero hash
	rand.Seed(time.Now().UnixNano())
	tree := NewTree()
	zero := tree.Hash()

	// insert the given number of random keys and values
	keys := make([][]byte, 0, TreeTestLength)
	vals := make([][]byte, 0, TreeTestLength)
	for i := 0; i < TreeTestLength; i++ {
		key, val := randomKeyValuePair(32, 128)
		keys = append(keys, key)
		vals = append(vals, val)
	}

	// insert all of the key-value pairs and ensure they were unique
	for i, key := range keys {
		val := vals[i]
		existed, err := tree.Put(key, val)
		assert.NoError(t, err)
		assert.False(t, existed)
	}

	// retrieve all of the key-value pairs, ensure they were there and correct
	for i, key := range keys {
		val := vals[i]
		out, retrieved := tree.Get(key)
		if assert.True(t, retrieved) {
			assert.Equal(t, val, out)
		}
	}

	// remove all of the key-value pairs, ensure it worked
	for _, key := range keys {
		deleted := tree.Del(key)
		assert.True(t, deleted)
	}

	// finally, check that the tree hash is zero after removing everything
	hash := tree.Hash()
	assert.Equal(t, zero, hash)
}

func TestRandomOrder(t *testing.T) {

	// initialize random generator, two trees and zero hash
	rand.Seed(time.Now().UnixNano())
	tree1 := NewTree()
	tree2 := NewTree()
	zero := tree1.Hash()

	// generate the desired number of keys and map a value to each key
	keys := make([][]byte, 0, TreeTestLength)
	vals := make(map[string][]byte)
	for i := 0; i < TreeTestLength; i++ {
		key, val := randomKeyValuePair(32, 128)
		keys = append(keys, key)
		vals[string(key)] = val
	}

	// insert all of the values into the first tree
	for _, key := range keys {
		val := vals[string(key)]
		existed, err := tree1.Put(key, val)
		assert.NoError(t, err)
		require.False(t, existed)
	}

	// shuffle the keys and insert them with new order into the second tree
	rand.Shuffle(len(keys), func(i int, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})
	for _, key := range keys {
		val := vals[string(key)]
		existed, err := tree2.Put(key, val)
		assert.NoError(t, err)
		require.False(t, existed)
	}

	// make sure the tree hashes were the same, in spite of random order
	assert.Equal(t, tree1.Hash(), tree2.Hash())

	// remove the key-value pairs from the first tree in random order
	for _, key := range keys {
		deleted := tree1.Del(key)
		require.True(t, deleted)
	}

	// make sure that its hash is zero in the end
	assert.Equal(t, zero, tree1.Hash())
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
	t := NewTree()
	for i := 0; i < n; i++ {
		key, val := randomKeyValuePair(32, 128)
		t.Put(key, val)
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
