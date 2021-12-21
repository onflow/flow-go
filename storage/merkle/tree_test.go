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

func TestTreeSingle(t *testing.T) {

	// initialize the random generator, tree and zero hash
	rand.Seed(time.Now().UnixNano())
	tree := NewTree()
	zero := tree.Hash()

	// for the pre-defined number of times...
	for i := 0; i < TreeTestLength; i++ {

		// insert a random key with a random value and make sure it didn't
		// exist yet; collisions are unlikely enough to never happen
		key := make([]byte, 32)
		val := make([]byte, 128)
		_, _ = rand.Read(key)
		_, _ = rand.Read(val)
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
		key := make([]byte, 32)
		val := make([]byte, 128)
		_, _ = rand.Read(key)
		_, _ = rand.Read(val)
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
		key := make([]byte, 32)
		val := make([]byte, 128)
		_, _ = rand.Read(key)
		_, _ = rand.Read(val)
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

func createPair() ([]byte, []byte) {
	key := make([]byte, 32)
	val := make([]byte, 128)
	_, _ = rand.Read(key)
	_, _ = rand.Read(val)
	return key, val
}

func createTree(n int) *Tree {
	t := NewTree()
	for i := 0; i < n; i++ {
		key, val := createPair()
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
			key, val := createPair()
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
			key, val := createPair()
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
			key, val := createPair()
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
