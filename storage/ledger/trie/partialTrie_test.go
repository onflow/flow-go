package trie

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

func TestPartialTrieEmptyTrie(t *testing.T) {

	trieHeight := 9 // should be key size (in bits) + 1
	// add key1 and value1 to the empty trie
	key1 := make([]byte, 1) // 00000000 (0)
	value1 := []byte{'a'}
	updatedValue1 := []byte{'A'}

	keys := make([][]byte, 0)
	values := make([][]byte, 0)
	keys = append(keys, key1)
	values = append(values, value1)

	trie := newTestSMT(t, trieHeight, cacheSize, 10, 100, 5)
	defer trie.SafeClose()
	retvalues, proofHldr, _ := trie.Read(keys, false, trie.GetRoot().value)
	psmt, err := NewPSMT(trie.GetRoot().value, trieHeight, keys, retvalues, EncodeProof(proofHldr))
	require.NoError(t, err, "error building partial trie")
	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [before set]")
	}
	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [after set]")
	}

	keys = make([][]byte, 0)
	values = make([][]byte, 0)
	keys = append(keys, key1)
	values = append(values, updatedValue1)

	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [after update]")
	}

}

func TestPartialTrieLeafUpdates(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1

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

	trie := newTestSMT(t, trieHeight, cacheSize, 10, 100, 5)
	defer trie.SafeClose()
	err := trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	retvalues, proofHldr, _ := trie.Read(keys, false, trie.GetRoot().value)
	psmt, err := NewPSMT(trie.GetRoot().value, trieHeight, keys, retvalues, EncodeProof(proofHldr))
	require.NoError(t, err, "error building partial trie")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [before update]")
	}

	values = make([][]byte, 0)
	values = append(values, updatedValue1, updatedValue2)
	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [after update]")
	}
}
func TestPartialTrieMiddleBranching(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1

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

	trie := newTestSMT(t, trieHeight, cacheSize, 10, 100, 5)
	defer trie.SafeClose()
	retvalues, proofHldr, _ := trie.Read(keys, false, trie.GetRoot().value)
	psmt, err := NewPSMT(trie.GetRoot().value, trieHeight, keys, retvalues, EncodeProof(proofHldr))
	require.NoError(t, err, "error building partial trie")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [before update]")
	}
	// first update
	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [before update]")
	}

	// second update
	values = make([][]byte, 0)
	values = append(values, updatedValue1, updatedValue2, updatedValue3)
	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [after update]")
	}
}

func TestPartialTrieRootUpdates(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1

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

	trie := newTestSMT(t, trieHeight, cacheSize, 10, 100, 5)
	defer trie.SafeClose()
	retvalues, proofHldr, _ := trie.Read(keys, false, trie.GetRoot().value)
	psmt, err := NewPSMT(trie.GetRoot().value, trieHeight, keys, retvalues, EncodeProof(proofHldr))
	require.NoError(t, err, "error building partial trie")

	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [before update]")
	}

	// first update
	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")
	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [before update]")
	}

	// second update
	values = make([][]byte, 0)
	values = append(values, updatedValue1, updatedValue2)
	err = trie.Update(keys, values)
	require.NoError(t, err, "error updating trie")

	_, _, err = psmt.Update(keys, values)
	require.NoError(t, err, "error updating psmt")
	if !bytes.Equal(trie.root.ComputeValue(), psmt.root.ComputeValue()) {
		t.Fatal("root hash doesn't match [after update]")
	}
}

// TODO add test for incompatible proofs [Byzantine milestone]

// TODO add test key not exist [Byzantine milestone]
