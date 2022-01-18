package merkle

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProofWithASingleKey tests proof generation and verification
// when trie includes only a single value
func TestProofWithASingleKey(t *testing.T) {

	keyLength := 32
	tree1, err := NewTree(keyLength)
	assert.NoError(t, err)

	key, val := randomKeyValuePair(32, 128)

	replaced, err := tree1.Put(key, val)
	assert.NoError(t, err)
	require.False(t, replaced)

	// work for an existing key
	proof, existed := tree1.Prove(key)
	require.True(t, existed)

	isValid, err := proof.Verify(tree1.Hash())
	assert.NoError(t, err)
	require.True(t, isValid)

	// fail for non-existing key
	key2, _ := randomKeyValuePair(32, 128)

	proof, existed = tree1.Prove(key2)
	require.False(t, existed)
	require.Nil(t, proof)

	// malformed proof - issue with the key
	proof, existed = tree1.Prove(key)
	require.True(t, existed)
	proof.Key = key2

	isValid, err = proof.Verify(tree1.Hash())
	assert.Error(t, err)
	require.False(t, isValid)

	// malformed proof - issue with the expected state commitment
	proof.Key = key
	isValid, err = proof.Verify(nil)
	assert.Error(t, err)
	require.False(t, isValid)
}

// TestProofsWithRandomKeys tests proof generation and verification
// when trie includes many random keys. (only a random subset of keys are checked for proofs)
func TestProofsWithRandomKeys(t *testing.T) {
	// initialize random generator, two trees and zero hash
	rand.Seed(time.Now().UnixNano())
	keyLength := 32
	numberOfInsertions := 10000
	numberOfProofsToVerify := 100
	tree1, err := NewTree(keyLength)
	assert.NoError(t, err)

	// generate the desired number of keys and map a value to each key
	keys := make([][]byte, 0, numberOfInsertions)
	vals := make(map[string][]byte)
	for i := 0; i < numberOfInsertions; i++ {
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

	// get proofs for keys and verify for a subset of keys
	for _, key := range keys[:numberOfProofsToVerify] {
		proof, existed := tree1.Prove(key)
		require.True(t, existed)
		isValid, err := proof.Verify(tree1.Hash())
		assert.NoError(t, err)
		require.True(t, isValid)
	}
}
