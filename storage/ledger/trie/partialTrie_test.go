package trie

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/stretchr/testify/require"
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

	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		retvalues, _, err := smt.Read(keys, true, emptyTree.Commitment())
		require.NoError(t, err, "error reading values")

		proofHldr, err := smt.GetBatchProof(keys, emptyTree.Commitment())
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(emptyTree.Commitment(), trieHeight, keys, retvalues, EncodeProof(proofHldr))
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(psmt.Commitment(), emptyTree.Commitment()) {
			t.Fatal("commitment hash doesn't match [before set]")
		}

		expCom, err := smt.Update(keys, values, emptyTree.Commitment())
		require.NoError(t, err, "error updating trie")

		newCom, _, err := psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newCom, expCom) {
			t.Fatal("rootNode hash doesn't match [after set]")
		}

		keys = make([][]byte, 0)
		values = make([][]byte, 0)
		keys = append(keys, key1)
		values = append(values, updatedValue1)

		expCom, err = smt.Update(keys, values, newCom)
		require.NoError(t, err, "error updating trie")

		newerCom, _, err := psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newerCom, expCom) {
			t.Fatal("commitment doesn't match [after update]")
		}

	})
}

func TestPartialTrieLeafUpdates(t *testing.T) {

	trieHeight := 9 // should be key size (in bits) + 1

	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

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

		newCom, err := smt.Update(keys, values, emptyTree.Commitment())
		require.NoError(t, err, "error updating trie")

		retvalues, _, err := smt.Read(keys, true, newCom)
		require.NoError(t, err, "error reading values")

		proofHldr, err := smt.GetBatchProof(keys, emptyTree.Commitment())
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(newCom, trieHeight, keys, retvalues, EncodeProof(proofHldr))
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(newCom, psmt.Commitment()) {
			t.Fatal("commitment doesn't match [before update]")
		}

		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2)
		newCom2, err := smt.Update(keys, values, newCom)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newCom2, psmt.Commitment()) {
			t.Fatal("commitment doesn't match [after update]")
		}
	})

}

func TestPartialTrieMiddleBranching(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1

	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

		retvalues, _, err := smt.Read(keys, true, emptyTree.Commitment())
		require.NoError(t, err, "error reading values")

		proofHldr, err := smt.GetBatchProof(keys, emptyTree.Commitment())
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(emptyTree.Commitment(), trieHeight, keys, retvalues, EncodeProof(proofHldr))
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(emptyTree.Commitment(), psmt.Commitment()) {
			t.Fatal("commitment hash doesn't match [before update]")
		}
		// first update
		newCom, err := smt.Update(keys, values, emptyTree.Commitment())
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newCom, psmt.Commitment()) {
			t.Fatal("commitment hash doesn't match [before update]")
		}

		// second update
		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2, updatedValue3)
		newCom2, err := smt.Update(keys, values, newCom)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(newCom2, psmt.Commitment()) {
			t.Fatal("commitment doesn't match [after update]")
		}
	})

}

func TestPartialTrieRootUpdates(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1

	withSMT(t, trieHeight, 10, 100, 3, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

		retvalues, _, err := smt.Read(keys, true, emptyTree.Commitment())
		require.NoError(t, err, "error reading values")

		proofHldr, err := smt.GetBatchProof(keys, emptyTree.Commitment())
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(emptyTree.Commitment(), trieHeight, keys, retvalues, EncodeProof(proofHldr))
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(emptyTree.Commitment(), psmt.Commitment()) {
			t.Fatal("commitment doesn't match [before update]")
		}

		// first update
		newCom, err := smt.Update(keys, values, emptyTree.Commitment())
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(newCom, psmt.Commitment()) {
			t.Fatal("commitment doesn't match [after update]")
		}

		// second update
		values = make([][]byte, 0)
		values = append(values, updatedValue1, updatedValue2)
		newCom2, err := smt.Update(keys, values, newCom)
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(keys, values)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(newCom2, psmt.Commitment()) {
			t.Fatal("commitment doesn't match [after update]")
		}
	})

}

func TestMixProof(t *testing.T) {
	trieHeight := 9 // should be key size (in bits) + 1

	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

		newCom, err := smt.Update(keys, values, emptyTree.Commitment())
		require.NoError(t, err, "error updating trie")

		keys = make([][]byte, 0)
		keys = append(keys, key1, key2, key3)

		retvalues, _, err := smt.Read(keys, true, newCom)
		require.NoError(t, err, "error reading values")

		_ = retvalues
		proofHldr, err := smt.GetBatchProof(keys, newCom)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(newCom, trieHeight, keys, retvalues, EncodeProof(proofHldr))
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(newCom, psmt.Commitment()) {
			t.Fatal("commitment doesn't match [before update]")
		}

		keys = make([][]byte, 0)
		keys = append(keys, key2, key3)

		values = make([][]byte, 0)
		values = append(keys, []byte{'X'}, []byte{'Y'})

		newCom2, err := smt.Update(keys, values, newCom)
		require.NoError(t, err, "error updating trie")

		pCom2, _, err := psmt.Update(keys, values)
		require.NoError(t, err, "error updating partial trie")

		if !bytes.Equal(newCom2, pCom2) {
			t.Fatalf("root2 hash doesn't match [%x] != [%x]", newCom2, pCom2)
		}

	})

}

func TestRandomProofs(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1

	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		// insert some values to an empty trie
		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		rand.Seed(time.Now().UnixNano())

		// numberOfKeys := rand.Intn(256)
		numberOfKeys := rand.Intn(100)
		for i := 0; i < numberOfKeys; i++ {
			key := make([]byte, 2)
			rand.Read(key)
			keys = append(keys, key)

			value := make([]byte, 4)
			rand.Read(value)
			values = append(values, value)
		}

		// keep a subset as initial insert and keep the rest as default value read
		split := rand.Intn(numberOfKeys)
		insertKeys := keys[:split]
		insertValues := values[:split]

		com, err := smt.Update(insertKeys, insertValues, emptyTree.Commitment())
		require.NoError(t, err, "error updating trie")

		// shuffle keys for read
		rand.Shuffle(len(keys), func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
			values[i], values[j] = values[j], values[i]
		})

		retvalues, _, err := smt.Read(keys, true, com)
		require.NoError(t, err, "error reading values")

		proofHldr, err := smt.GetBatchProof(keys, com)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(com, trieHeight, keys, retvalues, EncodeProof(proofHldr))
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(com, psmt.Commitment()) {
			t.Fatal("commitment doesn't match")
		}

		// select a subset of shuffled keys for random updates
		split = rand.Intn(numberOfKeys)
		updateKeys := keys[:split]
		updateValues := values[:split]
		// random updates
		rand.Shuffle(len(updateValues), func(i, j int) {
			updateValues[i], updateValues[j] = updateValues[j], updateValues[i]
		})

		com2, err := smt.Update(updateKeys, updateValues, com)
		require.NoError(t, err, "error updating trie")

		pcom2, _, err := psmt.Update(updateKeys, updateValues)
		require.NoError(t, err, "error updating partial trie")

		if !bytes.Equal(com2, pcom2) {
			t.Fatalf("commitment doesn't match [%x] != [%x]", com2, pcom2)
		}

	})

}

// // TODO add test for incompatible proofs [Byzantine milestone]
// // TODO add test key not exist [Byzantine milestone]
