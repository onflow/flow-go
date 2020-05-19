package trie

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	testHashLength = 32
)

func TestSMTInitialization(t *testing.T) {

	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {

		if smt.GetHeight() != 257 {
			t.Errorf("Height is %d; want %d", smt.GetHeight(), 257)
		}

		hashes := GetDefaultHashes()

		for i := 0; i < smt.GetHeight(); i++ {
			if len(hashes[i]) != 32 {
				t.Errorf("Length of hash at position %d is %d, should be %d", i, len(hashes[i]), testHashLength)
			}
		}

		if len(emptyTree.rootNode.value) != testHashLength {
			t.Errorf("Root should be a hash")
		}
	})
}

func TestSMTHeightTooSmall(t *testing.T) {
	dbDir := unittest.TempDir(t)

	defer func() {
		os.RemoveAll(dbDir)
	}()

	_, err := NewSMT(dbDir, -1, 10, 100, 10)

	require.Error(t, err, "Height error should have been thrown")
}

func TestInteriorNode(t *testing.T) {

	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
		k1 := make([]byte, 1)
		k2 := make([]byte, 2)
		v1 := make([]byte, 5)
		v2 := make([]byte, 8)

		ln := newNode(HashLeaf(k1, v1), 254)
		rn := newNode(HashLeaf(k2, v2), 254)

		batcher := emptyTree.database.NewBatcher()

		exp := newNode(HashInterNode(HashLeaf(k1, v1), HashLeaf(k2, v2)), 255)
		res := interiorNode(ln, rn, 255, batcher)

		if (res.height != exp.height) && (bytes.Equal(exp.value, res.value)) {
			t.Errorf("Interior node has value %b and height %d, should have value %b and height %d", res.value, res.height, exp.value, res.height)
		}
	})
}

func TestInteriorNodeLRNil(t *testing.T) {
	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {

		batcher := emptyTree.database.NewBatcher()

		res := interiorNode(nil, nil, 200, batcher)

		require.Nil(t, res, "Interior node is not nil")
	})

}

func TestInteriorNodeLNil(t *testing.T) {

	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
		k := make([]byte, 1)
		v := make([]byte, 5)
		h := HashLeaf(k, v)
		rn := newNode(h, 200)
		rn.value = h
		exp := newNode(HashInterNode(h, GetDefaultHashForHeight(201)), 201)

		batcher := emptyTree.database.NewBatcher()

		res := interiorNode(nil, rn, 201, batcher)

		if (res.height != exp.height) && (bytes.Equal(exp.value, res.value)) {
			t.Errorf("Interior node has value %b and height %d, should have value %b and height %d", res.value, res.height, exp.value, res.height)
		}
	})
}

func TestInteriorNodeRNil(t *testing.T) {

	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
		k := make([]byte, 1)
		v := make([]byte, 5)
		h := HashLeaf(k, v)

		ln := newNode(h, 200)
		ln.value = h

		exp := newNode(HashInterNode(h, GetDefaultHashForHeight(201)), 201)

		batcher := emptyTree.database.NewBatcher()

		res := interiorNode(ln, nil, 201, batcher)

		if (res.height != exp.height) && (bytes.Equal(exp.value, res.value)) {
			t.Errorf("Interior node has value %b and height %d, should have value %b and height %d", res.value, res.height, exp.value, res.height)
		}
	})
}

func TestUpdateWithWrongKeySize(t *testing.T) {
	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
		// short key
		key1 := make([]byte, 1)
		utils.SetBit(key1, 5)
		value1 := []byte{'a'}
		keys := [][]byte{key1}
		values := [][]byte{value1}

		_, err := smt.Update(keys, values, emptyTree.root)
		require.Error(t, err)

		// long key
		key2 := make([]byte, 33)
		utils.SetBit(key2, 5)
		value2 := []byte{'a'}
		keys = [][]byte{key2}
		values = [][]byte{value2}

		_, err = smt.Update(keys, values, emptyTree.root)
		require.Error(t, err)
	})

}

func TestReadWithWrongKeySize(t *testing.T) {
	withSMT(t, 257, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
		// setup data
		key1 := make([]byte, 32)
		utils.SetBit(key1, 5)
		value1 := []byte{'a'}
		keys := [][]byte{key1}
		values := [][]byte{value1}

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		// wrong key
		key2 := make([]byte, 33)
		utils.SetBit(key2, 5)
		keys = [][]byte{key2}

		_, _, read_err := smt.Read(keys, true, newRoot)
		require.Error(t, read_err)
	})

}

func TestInsertIntoKey(t *testing.T) {
	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'b'}
		utils.SetBit(key2, 5)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 0)

		key4 := make([]byte, 1)
		value4 := []byte{'d'}
		utils.SetBit(key4, 0)
		utils.SetBit(key4, 5)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4)

		values = append(values, value1, value2, value3, value4)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		keys = make([][]byte, 0)
		values = make([][]byte, 0)
		keys = append(keys, key1, key2, key4)
		values = append(values, value1, value2, value4)

		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		keys, _, err = smt.insertIntoKeys(newTree.database, key3, keys, values)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(keys[2], key3) {
			t.Errorf("Incorrect Insert")
			for _, key := range keys {
				t.Errorf("%b", key)
			}
		}
	})
}

func TestInsertToEndofKeys(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'b'}
		utils.SetBit(key2, 5)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 0)

		key4 := make([]byte, 1)
		value4 := []byte{'d'}
		utils.SetBit(key4, 0)
		utils.SetBit(key4, 5)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4)

		values = append(values, value1, value2, value3, value4)

		//sOldRoot := hex.EncodeToString(trie.GetRoot().value)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		keys = make([][]byte, 0)
		values = make([][]byte, 0)
		keys = append(keys, key1, key2, key3)
		values = append(values, value1, value2, value3)

		keys, _, err = smt.insertIntoKeys(newTree.database, key4, keys, values)
		require.NoError(t, err)

		if !bytes.Equal(keys[3], key4) {
			t.Errorf("Incorrect Insert")
			for _, key := range keys {
				t.Errorf("%b", key)
			}
		}

		keys, _, err = smt.insertIntoKeys(newTree.database, key4, keys, values)
		if err != nil {
			t.Fatal(err)
		}

		if !(len(keys) == 4) {
			t.Errorf("Incorrect Insert")
			for _, key := range keys {
				t.Errorf("%b", key)
			}
		}
	})
}

func TestUpdateAtomicallySingleValUpdateAndRead(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key := make([]byte, 8)
		value := []byte{'a'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key)
		values = append(values, value)

		database := emptyTree.database
		batcher := database.NewBatcher()

		newNode, err := smt.UpdateAtomically(emptyTree.rootNode, keys, values, 7, batcher, database)
		require.NoError(t, err, "Trie Write failed")

		require.NotNil(t, newNode, "ROOT IS NILL")

		//trie.root = res

		_, _, _, inclusion, err := smt.GetProof(key, newNode)
		require.NoError(t, err)

		if inclusion == false {
			t.Fatalf("Trie Read failed")
		}
	})
}

func TestUpdateAtomicallyMultiValUpdateAndRead(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'b'}
		utils.SetBit(key2, 5)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 0)

		key4 := make([]byte, 1)
		value4 := []byte{'d'}
		utils.SetBit(key4, 0)
		utils.SetBit(key4, 5)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4)

		values = append(values, value1, value2, value3, value4)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flags := make([][]byte, 0)

		for _, key := range keys {
			flag, _, _, inclusion, err := smt.GetProof(key, newTree.rootNode)
			require.NoError(t, err)
			require.True(t, inclusion, "Trie Read failed")
			flags = append(flags, flag)
		}

		test_vals, _, read_err := smt.Read(keys, false, newRoot)
		require.NoError(t, read_err)

		for i := 0; i < len(values); i++ {
			if !bytes.Equal(test_vals[i], values[i]) {
				t.Errorf("Value is Incorrect")
			}
		}

		flag1 := flags[0]
		if !CheckFlag(newTree.rootNode, flag1, key1, t) {
			t.Errorf("flag is Incorrect")
		}
	})
}

func TestTrustedRead(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'b'}
		utils.SetBit(key2, 5)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 0)

		key4 := make([]byte, 1)
		value4 := []byte{'d'}
		utils.SetBit(key4, 0)
		utils.SetBit(key4, 5)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4)

		values = append(values, value1, value2, value3, value4)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		test_vals, _, read_err := smt.Read(keys, true, newRoot)
		if read_err != nil {
			t.Fatalf(read_err.Error())
		}

		for i := 0; i < len(values); i++ {
			if !bytes.Equal(test_vals[i], values[i]) {
				t.Errorf("Value is Incorrect")
			}
		}
	})
}

func TestFailedRead(t *testing.T) {
	t.Skip("Current behavior allows reads on non-existant key/value")

	key1 := make([]byte, 1)
	utils.SetBit(key1, 2)

	key2 := make([]byte, 1)
	utils.SetBit(key2, 4)

	keys := make([][]byte, 0)

	keys = append(keys, key1, key2)

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		_, _, read_err := smt.Read(keys, true, emptyTree.root)
		if read_err == nil {
			t.Fatalf("Read an non-existant value without an error")
		}

	})
}

func TestGetProofFlags_MultipleValueTree(t *testing.T) {

	withSMT(t, 9, 10, 10, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)

		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 6)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, _, _, _, err := smt.GetProof(key1, newTree.rootNode)
		require.NoError(t, err)

		f, err := smt.verifyInclusionFlag(key1, flag, newRoot)
		require.NoError(t, err)

		flag, _, _, _, err = smt.GetProof(key2, newTree.rootNode)
		require.NoError(t, err)

		f2, err := smt.verifyInclusionFlag(key2, flag, newRoot)
		require.NoError(t, err)

		flag, _, _, _, err = smt.GetProof(key3, newTree.rootNode)
		require.NoError(t, err)

		f3, err := smt.verifyInclusionFlag(key3, flag, newRoot)
		require.NoError(t, err)

		if !f || !f2 || !f3 {
			t.Errorf("flag(s) are incorrect!")
		}
	})
}

func CheckFlag(trie *node, flag []byte, key []byte, t *testing.T) bool {
	level := 0
	for trie.key == nil {
		if utils.IsBitSet(key, level) {
			if utils.IsBitSet(flag, level) != (trie.lChild != nil) {
				t.Errorf("%d\n", level)
				return false
			} else {
				trie = trie.rChild
			}
		}
		if !utils.IsBitSet(key, level) {
			if utils.IsBitSet(flag, level) != (trie.rChild != nil) {
				t.Errorf("%d\n", level)
				return false
			} else {
				trie = trie.lChild
			}
		}

		level++
	}

	return true
}

func TestGetProofAndVerifyInclusionProof_SingleValueTreeLeft(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1)
		values = append(values, value1)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err := smt.GetProof(keys[0], newTree.rootNode)
		require.NoError(t, err)

		if inclusion == false {
			t.Fatalf("Trie Read failed")
		}

		if !VerifyInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestGetProof_SingleValueTreeConstructedLeft(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1)

		values = append(values, value1)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, _, inclusion, err := smt.GetProof(keys[0], newTree.rootNode)
		require.NoError(t, err)

		if inclusion == false {
			t.Fatalf("Trie Read failed")
		}

		expectedProof := make([][]byte, 0)
		var flagSetBitsNum int

		for i := 0; i < 8; i++ {
			if utils.IsBitSet(flag, i) {
				flagSetBitsNum++
			}
		}

		if !(len(expectedProof) == len(proof) && flagSetBitsNum == 0) {
			t.Errorf("not producing expected proof and flag for trie!")
		}
	})
}

func TestGetProofAndVerifyInclusionProof_SingleValueTreeRight(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		for i := 0; i < 8; i++ {
			utils.SetBit(key1, i)
		}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1)
		values = append(values, value1)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err := smt.GetProof(keys[0], newTree.rootNode)
		require.NoError(t, err)

		require.True(t, inclusion, "Trie Read failed")

		if !VerifyInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestGetProof_MultipleValueTree(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 6)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err := smt.GetProof(key1, newTree.rootNode)
		require.NoError(t, err)
		if inclusion == false {
			t.Fatalf("Trie Read failed")
		}

		verify1 := VerifyInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key2, newTree.rootNode)
		require.NoError(t, err)
		if inclusion == false {
			t.Fatalf("Trie Read failed")
		}

		verify2 := VerifyInclusionProof(key2, value2, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key3, newTree.rootNode)
		require.NoError(t, err)
		if inclusion == false {
			t.Fatalf("Trie Read failed")
		}

		verify3 := VerifyInclusionProof(key3, value3, flag, proof, size, newRoot, smt.height)
		if !(verify1 && verify2 && verify3) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestGetProof_MultipleStaggeredUpdates(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 1)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1)

		values = append(values, value1)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err := smt.GetProof(key1, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Trie Read failed")

		verify1 := VerifyInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height)
		require.True(t, verify1, "not producing expected rootNode for tree!")

		keys = make([][]byte, 0)
		values = make([][]byte, 0)

		keys = append(keys, key2, key3)

		values = append(values, value2, value3)

		newRoot, err = smt.Update(keys, values, newRoot)
		require.NoError(t, err)
		newTree, err = smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err = smt.GetProof(key1, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Trie Read failed")

		verify1 = VerifyInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key2, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Trie Read failed")

		verify2 := VerifyInclusionProof(key2, value2, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key3, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Trie Read failed")

		verify3 := VerifyInclusionProof(key3, value3, flag, proof, size, newRoot, smt.height)

		if !(verify1 && verify2 && verify3) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestGetProof_MultipleValueTreeDeeper(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		utils.SetBit(key1, 0)
		utils.SetBit(key1, 4)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 0)
		utils.SetBit(key2, 1)
		utils.SetBit(key2, 4)
		utils.SetBit(key2, 7)
		value2 := []byte{'b'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2)
		values = append(values, value1, value2)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err := smt.GetProof(key1, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Trie Read failed 1")

		verify1 := VerifyInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key2, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Trie Read failed 2")

		verify2 := VerifyInclusionProof(key2, value2, flag, proof, size, newRoot, smt.height)

		if !(verify1 && verify2) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestNonInclusionProof_MultipleValueTree(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 6)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		nonIncludedKey := make([]byte, 1)
		utils.SetBit(nonIncludedKey, 4)
		nonIncludedValue := []byte{'d'}

		flag, proof, size, inclusion, err := smt.GetProof(nonIncludedKey, newTree.rootNode)
		require.NoError(t, err)
		require.False(t, inclusion, "Key should not be included in the trie!")

		verifyNonInclusion := VerifyNonInclusionProof(nonIncludedKey, nonIncludedValue, flag, proof, size, newRoot, smt.height)

		if !(verifyNonInclusion) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestNonInclusionProof_EmptyTree(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		nonIncludedKey := make([]byte, 1)
		utils.SetBit(nonIncludedKey, 2)
		nonIncludedValue := []byte{'d'}

		flag, proof, size, inclusion, err := smt.GetProof(nonIncludedKey, emptyTree.rootNode)
		require.NoError(t, err)
		require.False(t, inclusion, "Key should not be included in the trie!")

		verifyNonInclusion := VerifyNonInclusionProof(nonIncludedKey, nonIncludedValue, flag, proof, size, emptyTree.root, smt.height)

		if !(verifyNonInclusion) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestNonInclusionProof_SingleValueTree(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1)

		values = append(values, value1)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		nonIncludedKey := make([]byte, 1)
		utils.SetBit(nonIncludedKey, 2)
		nonIncludedValue := []byte{'d'}

		flag, proof, size, inclusion, err := smt.GetProof(nonIncludedKey, newTree.rootNode)
		require.NoError(t, err)
		require.False(t, inclusion, "Key should not be included in the trie!")

		verifyNonInclusion := VerifyNonInclusionProof(nonIncludedKey, nonIncludedValue, flag, proof, size, newRoot, smt.height)

		if !(verifyNonInclusion) {
			t.Errorf("not producing expected rootNode for tree!")
		}
	})
}

func TestNonInclusionProof_IncludedKey(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 1)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		newTree, err := smt.forest.Get(newRoot)
		require.NoError(t, err)

		flag, proof, size, inclusion, err := smt.GetProof(key1, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Key should be included in the trie!")

		verifyNonInclusion1 := VerifyNonInclusionProof(key1, value1, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key2, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Key should be included in the trie!")

		verifyNonInclusion2 := VerifyNonInclusionProof(key2, value2, flag, proof, size, newRoot, smt.height)

		flag, proof, size, inclusion, err = smt.GetProof(key3, newTree.rootNode)
		require.NoError(t, err)
		require.True(t, inclusion, "Key should be included in the trie!")

		verifyNonInclusion3 := VerifyNonInclusionProof(key3, value3, flag, proof, size, newRoot, smt.height)

		if verifyNonInclusion1 || verifyNonInclusion2 || verifyNonInclusion3 {
			t.Errorf("key is included in trie but we are returning that it isn't included 1")
		}
	})

}

func TestHistoricalState(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 1)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		oldTree2, err := smt.forest.Get(oldRoot2)
		require.NoError(t, err)

		newvalue1 := []byte{'d'}
		newvalue2 := []byte{'e'}
		newvalue3 := []byte{'f'}

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot2)
		require.NoError(t, err)

		hv1, err := oldTree2.database.GetKVDB(key1)
		require.NoError(t, err)

		hv2, err := oldTree2.database.GetKVDB(key2)
		require.NoError(t, err)

		hv3, err := oldTree2.database.GetKVDB(key3)
		require.NoError(t, err)

		if !bytes.Equal(hv1, value1) || !bytes.Equal(hv2, value2) || !bytes.Equal(hv3, value3) {
			t.Errorf("Can't retrieve proper values from historical state!")
		}
	})

}

func TestGetHistoricalProofs(t *testing.T) {
	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 1)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		oldTree2, err := smt.forest.Get(oldRoot2)
		require.NoError(t, err)

		flag1, proof1, size1, inclusion1, err := smt.GetProof(key1, oldTree2.rootNode)
		require.NoError(t, err)

		flag2, proof2, size2, inclusion2, err := smt.GetProof(key2, oldTree2.rootNode)
		require.NoError(t, err)

		flag3, proof3, size3, inclusion3, err := smt.GetProof(key3, oldTree2.rootNode)
		require.NoError(t, err)

		newvalue1 := []byte{'d'}
		newvalue2 := []byte{'e'}
		newvalue3 := []byte{'f'}

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot2)
		require.NoError(t, err)

		hflag1, hproof1, hsize1, hinclusion1, err := smt.GetHistoricalProof(key1, oldRoot2, oldTree2.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag2, hproof2, hsize2, hinclusion2, err := smt.GetHistoricalProof(key2, oldRoot2, oldTree2.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag3, hproof3, hsize3, hinclusion3, err := smt.GetHistoricalProof(key3, oldRoot2, oldTree2.database)
		if err != nil {
			t.Fatal(err)
		}

		proofVerifier1 := true
		for i, pf := range hproof1 {
			if !bytes.Equal(pf, proof1[i]) {
				proofVerifier1 = false
			}
		}

		if !bytes.Equal(hflag1, flag1) || hsize1 != size1 || hinclusion1 != inclusion1 || !proofVerifier1 {
			t.Errorf("Can't retrieve proper proof from historical state for key 1!")
		}

		proofVerifier2 := true
		for i, pf := range hproof2 {
			if !bytes.Equal(pf, proof2[i]) {
				proofVerifier2 = false
			}
		}

		if !bytes.Equal(hflag2, flag2) || hsize2 != size2 || hinclusion2 != inclusion2 || !proofVerifier2 {
			t.Errorf("Can't retrieve proper proof from historical state for key 2!")
		}

		proofVerifier3 := true
		for i, pf := range hproof3 {
			if !bytes.Equal(pf, proof3[i]) {
				proofVerifier3 = false
			}
		}

		if !bytes.Equal(hflag3, flag3) || hsize3 != size3 || hinclusion3 != inclusion3 || !proofVerifier3 {
			t.Errorf("Can't retrieve proper proof from historical state for key 3!")
		}
	})

}

func TestGetHistoricalProofs_NonInclusion(t *testing.T) {
	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		utils.SetBit(key2, 1)
		value2 := []byte{'b'}

		key3 := make([]byte, 1)
		utils.SetBit(key3, 0)
		value3 := []byte{'c'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		oldTree2, err := smt.forest.Get(oldRoot2)
		require.NoError(t, err)

		nkey1 := make([]byte, 1)
		utils.SetBit(nkey1, 2)
		utils.SetBit(nkey1, 3)

		nkey2 := make([]byte, 1)
		utils.SetBit(nkey2, 4)

		flag1, proof1, size1, inclusion1, err := smt.GetProof(nkey1, oldTree2.rootNode)
		require.NoError(t, err)

		flag2, proof2, size2, inclusion2, err := smt.GetProof(nkey2, oldTree2.rootNode)
		require.NoError(t, err)

		newvalue1 := []byte{'d'}
		newvalue2 := []byte{'e'}
		newvalue3 := []byte{'f'}

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot2)
		require.NoError(t, err)

		hflag1, hproof1, hsize1, hinclusion1, err := smt.GetHistoricalProof(nkey1, oldRoot2, oldTree2.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag2, hproof2, hsize2, hinclusion2, err := smt.GetHistoricalProof(nkey2, oldRoot2, oldTree2.database)
		if err != nil {
			t.Fatal(err)
		}

		proofVerifier1 := true
		for i, pf := range hproof1 {
			if !bytes.Equal(pf, proof1[i]) {
				proofVerifier1 = false
			}
		}

		if !bytes.Equal(hflag1, flag1) || hsize1 != size1 || hinclusion1 != inclusion1 || !proofVerifier1 {
			t.Errorf("Can't retrieve proper proof from historical state for key 1!")
		}

		proofVerifier2 := true
		for i, pf := range hproof2 {
			if !bytes.Equal(pf, proof2[i]) {
				proofVerifier2 = false
			}
		}

		if !bytes.Equal(hflag2, flag2) || hsize2 != size2 || hinclusion2 != inclusion2 || !proofVerifier2 {
			t.Errorf("Can't retrieve proper proof from historical state for key 2!")
		}
	})

}

func TestRead_HistoricalValues(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'b'}
		utils.SetBit(key2, 5)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 0)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		oldTree2, err := smt.forest.Get(oldRoot2)
		require.NoError(t, err)

		flags := make([][]byte, 0)
		proofs := make([][][]byte, 0)
		proofLens := make([]uint8, 0)

		for _, key := range keys {
			flag, proof, proofLen, inclusion, err := smt.GetProof(key, oldTree2.rootNode)
			require.NoError(t, err)
			require.True(t, inclusion, "Trie Read failed")
			flags = append(flags, flag)
			proofs = append(proofs, proof)
			proofLens = append(proofLens, proofLen)
		}

		newvalue1 := []byte{'d'}
		newvalue2 := []byte{'e'}
		newvalue3 := []byte{'f'}

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot2)
		require.NoError(t, err)

		test_vals, proofHolder, read_err := smt.Read(keys, false, oldRoot2)
		require.NoError(t, read_err)

		for i := 0; i < len(values); i++ {
			if !bytes.Equal(test_vals[i], values[i]) {
				t.Errorf("Value is Incorrect")
			}
		}

		for i := 0; i < len(proofHolder.flags); i++ {
			if !bytes.Equal(flags[i], proofHolder.flags[i]) {
				t.Errorf("Flag is Incorrect")
			}
		}

		for i := 0; i < len(proofHolder.proofs); i++ {
			for j := 0; j < len(proofHolder.proofs[i]); j++ {
				if !bytes.Equal(proofs[i][j], proofHolder.proofs[i][j]) {
					t.Errorf("Proof is Incorrect")
				}

			}
		}

		for i := 0; i < len(proofHolder.sizes); i++ {
			if !(proofHolder.sizes[i] == proofLens[i]) {
				t.Errorf("Proof Size is Incorrect!")
			}

		}
	})

}

func TestRead_HistoricalValuesTrusted(t *testing.T) {

	withSMT(t, 9, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'b'}
		utils.SetBit(key2, 5)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 0)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'d'}
		newvalue2 := []byte{'e'}
		newvalue3 := []byte{'f'}

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot2)
		require.NoError(t, err)

		test_vals, _, read_err := smt.Read(keys, true, oldRoot2)
		if read_err != nil {
			t.Fatalf(read_err.Error())
		}

		for i := 0; i < len(values); i++ {
			if !bytes.Equal(test_vals[i], values[i]) {
				t.Errorf("Value is Incorrect")
			}
		}
	})

}

func TestGetHistoricalValues_Pruned(t *testing.T) {

	withSMT(t, 9, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 4)
		utils.SetBit(key2, 5)
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 2)
		utils.SetBit(key3, 5)
		utils.SetBit(key3, 6)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)
		oldTree3, err := smt.forest.Get(oldRoot3)
		require.NoError(t, err)

		flag1, proof1, size1, inclusion1, err := smt.GetProof(key1, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key1, newvalue1, flag1, proof1, size1, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 1!")
		}

		flag2, proof2, size2, inclusion2, err := smt.GetProof(key2, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key2, newvalue2, flag2, proof2, size2, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 2!")
		}

		flag3, proof3, size3, inclusion3, err := smt.GetProof(key3, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key3, value3, flag3, proof3, size3, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 3!")
		}

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)

		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot5)
		require.NoError(t, err)

		hflag1, hproof1, hsize1, hinclusion1, err := smt.GetHistoricalProof(key1, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		hflag2, hproof2, hsize2, hinclusion2, err := smt.GetHistoricalProof(key2, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		hflag3, hproof3, hsize3, hinclusion3, err := smt.GetHistoricalProof(key3, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		proofVerifier1 := true
		for i, pf := range hproof1 {
			if !bytes.Equal(pf, proof1[i]) {
				proofVerifier1 = false
			}
		}

		if !bytes.Equal(hflag1, flag1) || hsize1 != size1 || hinclusion1 != inclusion1 || !proofVerifier1 {
			t.Errorf("Can't retrieve proper proof from historical state for key 1!")
		}

		proofVerifier2 := true
		for i, pf := range hproof2 {
			if !bytes.Equal(pf, proof2[i]) {
				proofVerifier2 = false
			}
		}

		if !bytes.Equal(hflag2, flag2) || hsize2 != size2 || hinclusion2 != inclusion2 || !proofVerifier2 {
			t.Errorf("Can't retrieve proper proof from historical state for key 2!")
		}

		proofVerifier3 := true
		for i, pf := range hproof3 {
			if !bytes.Equal(pf, proof3[i]) {
				proofVerifier3 = false
			}
		}

		if !bytes.Equal(hflag3, flag3) || hsize3 != size3 || hinclusion3 != inclusion3 || !proofVerifier3 {
			t.Errorf("Can't retrieve proper proof from historical state for key 3!")
		}
	})

}

func TestGetProof_Pruned(t *testing.T) {

	withSMT(t, 9, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 4)
		utils.SetBit(key2, 5)
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 2)
		utils.SetBit(key3, 5)
		utils.SetBit(key3, 6)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3)

		values = append(values, value1, value2, value3)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)
		oldTree3, err := smt.forest.Get(oldRoot3)
		require.NoError(t, err)

		flag1, proof1, size1, inclusion1, err := smt.GetProof(key1, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key1, newvalue1, flag1, proof1, size1, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 1!")
		}

		flag2, proof2, size2, inclusion2, err := smt.GetProof(key2, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key2, newvalue2, flag2, proof2, size2, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 2!")
		}

		flag3, proof3, size3, inclusion3, err := smt.GetProof(key3, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key3, value3, flag3, proof3, size3, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 3!")
		}

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)

		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(keys, newvalues, oldRoot5)
		require.NoError(t, err)

		hflag1, hproof1, hsize1, hinclusion1, err := smt.GetHistoricalProof(key1, oldRoot3, oldTree3.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag2, hproof2, hsize2, hinclusion2, err := smt.GetHistoricalProof(key2, oldRoot3, oldTree3.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag3, hproof3, hsize3, hinclusion3, err := smt.GetHistoricalProof(key3, oldRoot3, oldTree3.database)
		if err != nil {
			t.Fatal(err)
		}

		proofVerifier1 := true
		for i, pf := range hproof1 {
			if !bytes.Equal(pf, proof1[i]) {
				proofVerifier1 = false
			}
		}

		if !bytes.Equal(hflag1, flag1) || hsize1 != size1 || hinclusion1 != inclusion1 || !proofVerifier1 {
			t.Errorf("Can't retrieve proper proof from historical state for key 1!")
		}

		proofVerifier2 := true
		for i, pf := range hproof2 {
			if !bytes.Equal(pf, proof2[i]) {
				proofVerifier2 = false
			}
		}

		if !bytes.Equal(hflag2, flag2) || hsize2 != size2 || hinclusion2 != inclusion2 || !proofVerifier2 {
			t.Errorf("Can't retrieve proper proof from historical state for key 2!")
		}

		proofVerifier3 := true
		for i, pf := range hproof3 {
			if !bytes.Equal(pf, proof3[i]) {
				proofVerifier3 = false
			}
		}

		if !bytes.Equal(hflag3, flag3) || hsize3 != size3 || hinclusion3 != inclusion3 || !proofVerifier3 {
			t.Errorf("Can't retrieve proper proof from historical state for key 3!")
		}
	})

}

func TestGetProof_Pruned_LargerTrie(t *testing.T) {

	withSMT(t, 9, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 4)
		utils.SetBit(key2, 5)
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 3)
		utils.SetBit(key3, 5)
		utils.SetBit(key3, 6)

		key4 := make([]byte, 1)
		value4 := []byte{'t'}
		utils.SetBit(key4, 2)
		utils.SetBit(key4, 4)
		utils.SetBit(key4, 6)

		key5 := make([]byte, 1)
		value5 := []byte{'v'}
		utils.SetBit(key5, 2)
		utils.SetBit(key5, 3)
		utils.SetBit(key5, 4)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4, key5)

		values = append(values, value1, value2, value3, value4, value5)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)
		oldTree3, err := smt.forest.Get(oldRoot3)
		require.NoError(t, err)

		flag1, proof1, size1, inclusion1, err := smt.GetProof(key1, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key1, newvalue1, flag1, proof1, size1, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 1!")
		}

		flag2, proof2, size2, inclusion2, err := smt.GetProof(key2, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key2, newvalue2, flag2, proof2, size2, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 2!")
		}

		flag3, proof3, size3, inclusion3, err := smt.GetProof(key3, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key3, value3, flag3, proof3, size3, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 3!")
		}

		flag4, proof4, size4, inclusion4, err := smt.GetProof(key4, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key4, value4, flag4, proof4, size4, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 4!")
		}

		flag5, proof5, size5, inclusion5, err := smt.GetProof(key5, oldTree3.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key5, value5, flag5, proof5, size5, oldRoot3, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 3!")
		}

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)
		oldTree5, err := smt.forest.Get(oldRoot5)
		require.NoError(t, err)

		nflag1, nproof1, nsize1, ninclusion1, err := smt.GetProof(key1, oldTree5.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key1, newvalue1, nflag1, nproof1, nsize1, oldRoot5, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 1!")
		}

		nflag2, nproof2, nsize2, ninclusion2, err := smt.GetProof(key2, oldTree5.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key2, newvalue2, nflag2, nproof2, nsize2, oldRoot5, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 2!")
		}

		nflag3, nproof3, nsize3, ninclusion3, err := smt.GetProof(key3, oldTree5.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key3, value3, nflag3, nproof3, nsize3, oldRoot5, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 3!")
		}

		nflag4, nproof4, nsize4, ninclusion4, err := smt.GetProof(key4, oldTree5.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key4, value4, nflag4, nproof4, nsize4, oldRoot5, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 4!")
		}

		nflag5, nproof5, nsize5, ninclusion5, err := smt.GetProof(key5, oldTree5.rootNode)
		require.NoError(t, err)

		if !VerifyInclusionProof(key5, value5, nflag5, nproof5, nsize5, oldRoot5, smt.height) {
			t.Errorf("not producing expected rootNode for tree with key 5!")
		}

		newkeys2 := make([][]byte, 0)
		newkeys2 = append(newkeys2, key1, key2, key3)
		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(newkeys2, newvalues, oldRoot5)
		require.NoError(t, err)

		hflag1, hproof1, hsize1, hinclusion1, err := smt.GetHistoricalProof(key1, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		hflag2, hproof2, hsize2, hinclusion2, err := smt.GetHistoricalProof(key2, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		hflag3, hproof3, hsize3, hinclusion3, err := smt.GetHistoricalProof(key3, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		hflag4, hproof4, hsize4, hinclusion4, err := smt.GetHistoricalProof(key4, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		hflag5, hproof5, hsize5, hinclusion5, err := smt.GetHistoricalProof(key5, oldRoot3, oldTree3.database)
		require.NoError(t, err)

		proofVerifier1 := true
		for i, pf := range hproof1 {
			if !bytes.Equal(pf, proof1[i]) {
				proofVerifier1 = false
			}
		}

		if !bytes.Equal(hflag1, flag1) || hsize1 != size1 || hinclusion1 != inclusion1 || !proofVerifier1 {
			t.Errorf("Can't retrieve proper proof from historical state for key 1!")
		}

		proofVerifier2 := true
		for i, pf := range hproof2 {
			if !bytes.Equal(pf, proof2[i]) {
				proofVerifier2 = false
			}
		}

		if !bytes.Equal(hflag2, flag2) || hsize2 != size2 || hinclusion2 != inclusion2 || !proofVerifier2 {
			t.Errorf("Can't retrieve proper proof from historical state for key 2!")
		}

		proofVerifier3 := true
		for i, pf := range hproof3 {
			if !bytes.Equal(pf, proof3[i]) {
				proofVerifier3 = false
			}
		}

		if !bytes.Equal(hflag3, flag3) || hsize3 != size3 || hinclusion3 != inclusion3 || !proofVerifier3 {
			t.Errorf("Can't retrieve proper proof from historical state for key 3!")
		}

		proofVerifier4 := true
		for i, pf := range hproof4 {
			if !bytes.Equal(pf, proof4[i]) {
				proofVerifier4 = false
			}
		}

		if !bytes.Equal(hflag4, flag4) || hsize4 != size4 || hinclusion4 != inclusion4 || !proofVerifier4 {
			t.Errorf("Can't retrieve proper proof from historical state for key 4!")
		}

		proofVerifier5 := true
		for i, pf := range hproof5 {
			if !bytes.Equal(pf, proof5[i]) {
				proofVerifier5 = false
			}
		}

		if !bytes.Equal(hflag5, flag5) || hsize5 != size5 || hinclusion5 != inclusion5 || !proofVerifier5 {
			t.Errorf("Can't retrieve proper proof from historical state for key 5!")
		}

		hflag1, hproof1, hsize1, hinclusion1, err = smt.GetHistoricalProof(key1, oldRoot5, oldTree5.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag2, hproof2, hsize2, hinclusion2, err = smt.GetHistoricalProof(key2, oldRoot5, oldTree5.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag3, hproof3, hsize3, hinclusion3, err = smt.GetHistoricalProof(key3, oldRoot5, oldTree5.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag4, hproof4, hsize4, hinclusion4, err = smt.GetHistoricalProof(key4, oldRoot5, oldTree5.database)
		if err != nil {
			t.Fatal(err)
		}

		hflag5, hproof5, hsize5, hinclusion5, err = smt.GetHistoricalProof(key5, oldRoot5, oldTree5.database)
		if err != nil {
			t.Fatal(err)
		}

		proofVerifier1 = true
		for i, pf := range hproof1 {
			if !bytes.Equal(pf, nproof1[i]) {
				proofVerifier1 = false
			}
		}

		if !bytes.Equal(hflag1, nflag1) || hsize1 != nsize1 || hinclusion1 != ninclusion1 || !proofVerifier1 {
			t.Errorf("Can't retrieve proper proof from historical state for key 1!")
		}

		proofVerifier2 = true
		for i, pf := range hproof2 {
			if !bytes.Equal(pf, nproof2[i]) {
				proofVerifier2 = false
			}
		}

		if !bytes.Equal(hflag2, nflag2) || hsize2 != nsize2 || hinclusion2 != ninclusion2 || !proofVerifier2 {
			t.Errorf("Can't retrieve proper proof from historical state for key 2!")
		}

		proofVerifier3 = true
		for i, pf := range hproof3 {
			if !bytes.Equal(pf, nproof3[i]) {
				proofVerifier3 = false
			}
		}

		if !bytes.Equal(hflag3, nflag3) || hsize3 != nsize3 || hinclusion3 != ninclusion3 || !proofVerifier3 {
			t.Errorf("Can't retrieve proper proof from historical state for key 3!")
		}

		proofVerifier4 = true
		for i, pf := range hproof4 {
			if !bytes.Equal(pf, nproof4[i]) {
				proofVerifier4 = false
			}
		}

		if !bytes.Equal(hflag4, nflag4) || hsize4 != nsize4 || hinclusion4 != ninclusion4 || !proofVerifier4 {
			t.Errorf("Can't retrieve proper proof from historical state for key 4!")
		}

		proofVerifier5 = true
		for i, pf := range hproof5 {
			if !bytes.Equal(pf, nproof5[i]) {
				proofVerifier5 = false
			}
		}

		if !bytes.Equal(hflag5, nflag5) || hsize5 != nsize5 || hinclusion5 != ninclusion5 || !proofVerifier5 {
			t.Errorf("Can't retrieve proper proof from historical state for key 5!")
		}
	})

}

func TestTrustedRead_Pruned(t *testing.T) {

	withSMT(t, 9, 6, 6, 1, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 5)
		utils.SetBit(key3, 6)

		key4 := make([]byte, 1)
		value4 := []byte{'t'}
		utils.SetBit(key4, 2)
		utils.SetBit(key4, 4)
		utils.SetBit(key4, 6)

		key5 := make([]byte, 1)
		value5 := []byte{'v'}
		utils.SetBit(key5, 2)
		utils.SetBit(key5, 3)
		utils.SetBit(key5, 4)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4, key5)

		values = append(values, value1, value2, value3, value4, value5)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)

		expectedValues := make([][]byte, 0)
		expectedValues = append(expectedValues, newvalue1, newvalue2, value3, value4, value5)

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)

		newkeys2 := make([][]byte, 0)
		newkeys2 = append(newkeys2, key1, key2, key3)
		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(newkeys2, newvalues, oldRoot5)
		if err != nil {
			t.Fatal(err)
		}

		test_vals, _, read_err := smt.Read(keys, true, oldRoot3)
		require.NoError(t, read_err)

		for i := 0; i < len(expectedValues); i++ {
			if !bytes.Equal(test_vals[i], expectedValues[i]) {
				t.Errorf("Value is Incorrect")
			}
		}
	})
}

func TestKVDB_Pruned(t *testing.T) {

	withSMT(t, 9, 2, 6, 6, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 4)
		utils.SetBit(key2, 5)
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 3)
		utils.SetBit(key3, 5)
		utils.SetBit(key3, 6)

		key4 := make([]byte, 1)
		value4 := []byte{'t'}
		utils.SetBit(key4, 2)
		utils.SetBit(key4, 4)
		utils.SetBit(key4, 6)

		key5 := make([]byte, 1)
		value5 := []byte{'v'}
		utils.SetBit(key5, 2)
		utils.SetBit(key5, 3)
		utils.SetBit(key5, 4)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4, key5)

		values = append(values, value1, value2, value3, value4, value5)

		expectedValues1 := make([][]byte, 0)
		expectedValues1 = append(expectedValues1, value1, value2, value3, value4, value5)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)

		expectedValues2 := make([][]byte, 0)
		expectedValues2 = append(expectedValues2, newvalue1, newvalue2)

		if err != nil {
			t.Fatal(err)
		}

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		expectedValues3 := make([][]byte, 0)
		expectedValues3 = append(expectedValues3, newvalue1, newvalue2)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)

		newkeys2 := make([][]byte, 0)
		newkeys2 = append(newkeys2, key1, key2, key3)
		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(newkeys2, newvalues, oldRoot5)
		require.NoError(t, err)

		oldTree2, err := smt.forest.Get(oldRoot2)
		require.NoError(t, err)

		db := oldTree2.database

		val1, err := db.GetKVDB(key1)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val1, expectedValues1[0]) {
			t.Errorf("Got wrong value from db!")
		}

		val2, err := db.GetKVDB(key2)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val2, expectedValues1[1]) {
			t.Errorf("Got wrong value from db!")
		}

		val3, err := db.GetKVDB(key3)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val3, expectedValues1[2]) {
			t.Errorf("Got wrong value from db!")
		}

		val4, err := db.GetKVDB(key4)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val4, expectedValues1[3]) {
			t.Errorf("Got wrong value from db!")
		}

		val5, err := db.GetKVDB(key5)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val5, expectedValues1[4]) {
			t.Errorf("Got wrong value from db!")
		}

		// THIS SHOULD BE PRUNED
		oldTree3, err := smt.forest.Get(oldRoot3)
		require.NoError(t, err)

		db = oldTree3.database

		val1, err = db.GetKVDB(key1)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val1, expectedValues2[0]) {
			t.Errorf("Got wrong value from db!")
		}

		val2, err = db.GetKVDB(key2)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val2, expectedValues2[1]) {
			t.Errorf("Got wrong value from db!")
		}

		_, err = db.GetKVDB(key3)
		if err == nil {
			t.Errorf("key3 should be pruned!")
		}

		_, err = db.GetKVDB(key4)
		if err == nil {
			t.Errorf("key4 should be pruned!")
		}

		_, err = db.GetKVDB(key5)
		if err == nil {
			t.Errorf("key5 should be pruned!")
		}

		// THIS SHOULD BE A FULL SNAPSHOT
		oldTree4, err := smt.forest.Get(oldRoot4)
		require.NoError(t, err)

		db = oldTree4.database

		val1, err = db.GetKVDB(key1)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val1, expectedValues3[0]) {
			t.Errorf("Got wrong value from db!")
		}

		val2, err = db.GetKVDB(key2)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val2, expectedValues3[1]) {
			t.Errorf("Got wrong value from db!")
		}

		if !bytes.Equal(val3, expectedValues1[2]) {
			t.Errorf("Got wrong value from db!")
		}

		val4, err = db.GetKVDB(key4)
		require.NoError(t, err)

		if !bytes.Equal(val4, expectedValues1[3]) {
			t.Errorf("Got wrong value from db!")
		}

		val5, err = db.GetKVDB(key5)
		require.NoError(t, err)

		if !bytes.Equal(val5, expectedValues1[4]) {
			t.Errorf("Got wrong value from db!")
		}
	})
}

func TestKVDB_Pruned2(t *testing.T) {

	withSMT(t, 9, 6, 6, 6, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 6)

		key4 := make([]byte, 1)
		value4 := []byte{'t'}
		utils.SetBit(key4, 2)
		utils.SetBit(key4, 5)

		key5 := make([]byte, 1)
		value5 := []byte{'v'}
		utils.SetBit(key5, 2)
		utils.SetBit(key5, 3)
		utils.SetBit(key5, 4)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4, key5)

		values = append(values, value1, value2, value3, value4, value5)

		expectedValues1 := make([][]byte, 0)
		expectedValues1 = append(expectedValues1, value1, value2, value3, value4, value5)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		oldTree2, err := smt.forest.Get(oldRoot2)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)

		expectedValues2 := make([][]byte, 0)
		expectedValues2 = append(expectedValues2, newvalue1, newvalue2)

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		expectedValues3 := make([][]byte, 0)
		expectedValues3 = append(expectedValues3, newvalue1, newvalue2)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)

		newkeys2 := make([][]byte, 0)
		newkeys2 = append(newkeys2, key1, key2, key3)
		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(newkeys2, newvalues, oldRoot5)
		require.NoError(t, err)

		db := oldTree2.database

		val1, err := db.GetKVDB(key1)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val1, expectedValues1[0]) {
			t.Errorf("Got wrong value from db!")
		}

		val2, err := db.GetKVDB(key2)
		require.NoError(t, err)

		if !bytes.Equal(val2, expectedValues1[1]) {
			t.Errorf("Got wrong value from db!")
		}

		val3, err := db.GetKVDB(key3)
		require.NoError(t, err)

		if !bytes.Equal(val3, expectedValues1[2]) {
			t.Errorf("Got wrong value from db!")
		}

		val4, err := db.GetKVDB(key4)
		require.NoError(t, err)

		if !bytes.Equal(val4, expectedValues1[3]) {
			t.Errorf("Got wrong value from db!")
		}

		val5, err := db.GetKVDB(key5)
		require.NoError(t, err)

		if !bytes.Equal(val5, expectedValues1[4]) {
			t.Errorf("Got wrong value from db!")
		}

		// THIS SHOULD NOT BE PRUNED
		oldTree3, err := smt.forest.Get(oldRoot3)
		require.NoError(t, err)

		db = oldTree3.database

		val1, err = db.GetKVDB(key1)
		require.NoError(t, err)

		if !bytes.Equal(val1, expectedValues2[0]) {
			t.Errorf("Got wrong value from db!")
		}

		val2, err = db.GetKVDB(key2)
		require.NoError(t, err)

		if !bytes.Equal(val2, expectedValues2[1]) {
			t.Errorf("Got wrong value from db!")
		}

		if !bytes.Equal(val3, expectedValues1[2]) {
			t.Errorf("Got wrong value from db!")
		}

		val4, err = db.GetKVDB(key4)
		require.NoError(t, err)

		if !bytes.Equal(val4, expectedValues1[3]) {
			t.Errorf("Got wrong value from db!")
		}

		val5, err = db.GetKVDB(key5)
		require.NoError(t, err)

		if !bytes.Equal(val5, expectedValues1[4]) {
			t.Errorf("Got wrong value from db!")
		}

		// THIS SHOULD NOT BE PRUNED
		oldTree4, err := smt.forest.Get(oldRoot4)
		require.NoError(t, err)

		db = oldTree4.database

		val1, err = db.GetKVDB(key1)
		require.NoError(t, err)

		if !bytes.Equal(val1, expectedValues3[0]) {
			t.Errorf("Got wrong value from db!")
		}

		val2, err = db.GetKVDB(key2)
		require.NoError(t, err)

		if !bytes.Equal(val2, expectedValues3[1]) {
			t.Errorf("Got wrong value from db!")
		}

		val3, err = db.GetKVDB(key3)
		require.NoError(t, err)

		if !bytes.Equal(val3, expectedValues1[2]) {
			t.Errorf("Got wrong value from db!")
		}

		val4, err = db.GetKVDB(key4)
		require.NoError(t, err)

		if !bytes.Equal(val4, expectedValues1[3]) {
			t.Errorf("Got wrong value from db!")
		}

		val5, err = db.GetKVDB(key5)
		require.NoError(t, err)

		if !bytes.Equal(val5, expectedValues1[4]) {
			t.Errorf("Got wrong value from db!")
		}
	})
}

func TestComputeCompactValue(t *testing.T) {
	trieHeight := 9

	key := make([]byte, 1) // 01010101 (1)
	utils.SetBit(key, 1)
	utils.SetBit(key, 3)
	utils.SetBit(key, 5)
	utils.SetBit(key, 7)
	value := []byte{'V'}

	level0 := HashLeaf(key, value)
	level1 := HashInterNode(GetDefaultHashForHeight(0), level0)
	level2 := HashInterNode(level1, GetDefaultHashForHeight(1))
	level3 := HashInterNode(GetDefaultHashForHeight(2), level2)
	level4 := HashInterNode(level3, GetDefaultHashForHeight(3))
	level5 := HashInterNode(GetDefaultHashForHeight(4), level4)
	level6 := HashInterNode(level5, GetDefaultHashForHeight(5))
	level7 := HashInterNode(GetDefaultHashForHeight(6), level6)

	// leaf node
	assert.Equal(t, ComputeCompactValue(key, value, 0, trieHeight), level0)
	// intermediate levels
	assert.Equal(t, ComputeCompactValue(key, value, 1, trieHeight), level1)
	assert.Equal(t, ComputeCompactValue(key, value, 2, trieHeight), level2)
	assert.Equal(t, ComputeCompactValue(key, value, 3, trieHeight), level3)
	assert.Equal(t, ComputeCompactValue(key, value, 4, trieHeight), level4)
	assert.Equal(t, ComputeCompactValue(key, value, 5, trieHeight), level5)
	assert.Equal(t, ComputeCompactValue(key, value, 6, trieHeight), level6)
	// rootNode node
	assert.Equal(t, ComputeCompactValue(key, value, 7, trieHeight), level7)
}

func TestRead_HistoricalValuesPruned(t *testing.T) {

	withSMT(t, 9, 6, 6, 6, func(t *testing.T, smt *SMT, emptyTree *tree) {
		key1 := make([]byte, 1)
		value1 := []byte{'a'}

		key2 := make([]byte, 1)
		value2 := []byte{'p'}
		utils.SetBit(key2, 4)
		utils.SetBit(key2, 5)
		utils.SetBit(key2, 7)

		key3 := make([]byte, 1)
		value3 := []byte{'c'}
		utils.SetBit(key3, 3)
		utils.SetBit(key3, 5)
		utils.SetBit(key3, 6)

		key4 := make([]byte, 1)
		value4 := []byte{'t'}
		utils.SetBit(key4, 2)
		utils.SetBit(key4, 4)
		utils.SetBit(key4, 6)

		key5 := make([]byte, 1)
		value5 := []byte{'v'}
		utils.SetBit(key5, 2)
		utils.SetBit(key5, 3)
		utils.SetBit(key5, 4)

		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		keys = append(keys, key1, key2, key3, key4, key5)

		values = append(values, value1, value2, value3, value4, value5)

		oldRoot2, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		newvalue1 := []byte{'z'}
		newvalue2 := []byte{'e'}

		newkeys := make([][]byte, 0)
		newkeys = append(newkeys, key1, key2)

		newvalues := make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot3, err := smt.Update(newkeys, newvalues, oldRoot2)
		require.NoError(t, err)
		oldTree3, err := smt.forest.Get(oldRoot3)
		require.NoError(t, err)

		flags := make([][]byte, 0)
		proofs := make([][][]byte, 0)
		proofLens := make([]uint8, 0)

		for _, key := range keys {
			flag, proof, proofLen, inclusion, err := smt.GetProof(key, oldTree3.rootNode)
			require.NoError(t, err)
			require.True(t, inclusion, "Trie Read failed")
			flags = append(flags, flag)
			proofs = append(proofs, proof)
			proofLens = append(proofLens, proofLen)
		}

		expectedValues := make([][]byte, 0)
		expectedValues = append(expectedValues, newvalue1, newvalue2, value3, value4, value5)

		newvalue1 = []byte{'g'}
		newvalue2 = []byte{'h'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot4, err := smt.Update(newkeys, newvalues, oldRoot3)
		require.NoError(t, err)

		newvalue1 = []byte{'u'}
		newvalue2 = []byte{'a'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2)

		oldRoot5, err := smt.Update(newkeys, newvalues, oldRoot4)
		require.NoError(t, err)

		newkeys2 := make([][]byte, 0)
		newkeys2 = append(newkeys2, key1, key2, key3)
		newvalue1 = []byte{'m'}
		newvalue2 = []byte{'q'}
		newvalue3 := []byte{'o'}

		newvalues = make([][]byte, 0)
		newvalues = append(newvalues, newvalue1, newvalue2, newvalue3)

		_, err = smt.Update(newkeys2, newvalues, oldRoot5)
		require.NoError(t, err)

		test_vals, proofHolder, read_err := smt.Read(keys, false, oldRoot3)
		require.NoError(t, read_err)

		for i := 0; i < len(expectedValues); i++ {
			if !bytes.Equal(test_vals[i], expectedValues[i]) {
				t.Errorf("Value is Incorrect")
			}
		}

		for i := 0; i < len(proofHolder.flags); i++ {
			if !bytes.Equal(flags[i], proofHolder.flags[i]) {
				t.Errorf("Flag is Incorrect")
			}
		}

		for i := 0; i < len(proofHolder.proofs); i++ {
			for j := 0; j < len(proofHolder.proofs[i]); j++ {
				if !bytes.Equal(proofs[i][j], proofHolder.proofs[i][j]) {
					t.Errorf("Proof is Incorrect")
				}

			}
		}

		for i := 0; i < len(proofHolder.sizes); i++ {
			if !(proofHolder.sizes[i] == proofLens[i]) {
				t.Errorf("Proof Size is Incorrect!")
			}
		}
	})
}

func TestProofEncoderDecoder(t *testing.T) {

	trieHeight := 9
	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		// add key1 and value1 to the empty trie
		key1 := make([]byte, 1) // 00000000 (0)
		value1 := []byte{'a'}

		key2 := make([]byte, 1) // 00000001 (1)
		utils.SetBit(key2, 7)
		value2 := []byte{'b'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1, key2)
		values = append(values, value1, value2)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		_, proofHldr, err := smt.Read(keys, false, newRoot)
		require.NoError(t, err)

		p, err := DecodeProof(EncodeProof(proofHldr))
		require.NoError(t, err)
		require.Equal(t, p, proofHldr, "Proof encoder and/or decoder has an issue")
	})

	trieHeight = 257
	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		// add key1 and value1 to the empty trie
		key1 := make([]byte, 32) // 00000000 (0)
		value1 := []byte{'a'}

		key2 := make([]byte, 32) // 00000001 (1)
		utils.SetBit(key2, 7)
		value2 := []byte{'b'}

		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		keys = append(keys, key1, key2)
		values = append(values, value1, value2)

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)
		_, proofHldr, err := smt.Read(keys, false, newRoot)
		require.NoError(t, err)

		p, err := DecodeProof(EncodeProof(proofHldr))
		require.NoError(t, err)
		require.Equal(t, p, proofHldr, "Proof encoder and/or decoder has an issue")
	})
}
func TestTrieConstructionCase1(t *testing.T) {

	trieHeight := 257
	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
		// 0001111001101001...
		// [30 105 156 202 161 209 27 123 89 40 174 255 91 148 246 122 86 30 72 252 203 180 164 95 96 111 164 127 31 232 165 99]
		key1, _ := hex.DecodeString("1e699ccaa1d11b7b5928aeff5b94f67a561e48fccbb4a45f606fa47f1fe8a563")
		value1 := []byte{'a'}

		// 0100101101110011...
		// [75 115 94 30 83 178 28 62 236 122 205 55 42 80 98 41 130 82 198 150 140 223 93 195 51 172 216 40 87 194 41 47]
		key2, _ := hex.DecodeString("4b735e1e53b21c3eec7acd372a5062298252c6968cdf5dc333acd82857c2292f")
		value2 := []byte{'b'}

		// 0110011011011011...
		// [102 219 49 15 46 242 56 112 222 196 199 126 23 84 69 16 146 84 195 238 143 1 9 252 237 26 40 91 189 183 128 52]
		key3, _ := hex.DecodeString("66db310f2ef23870dec4c77e175445109254c3ee8f0109fced1a285bbdb78034")
		value3, _ := hex.DecodeString("01")

		// 1000010101111000...
		// [133 120 84 0 236 182 113 244 61 164 14 85 11 78 71 210 104 193 112 144 118 71 113 8 181 248 42 101 206 60 172 35]
		key4, _ := hex.DecodeString("85785400ecb671f43da40e550b4e47d268c1709076477108b5f82a65ce3cac23")
		value4, _ := hex.DecodeString("01")

		// 1100011010110010...
		// [198 178 3 70 165 140 220 177 201 138 211 113 139 136 177 159 168 94 39 63 17 54 241 70 65 113 35 219 118 94 7 217]
		key5, _ := hex.DecodeString("c6b20346a58cdcb1c98ad3718b88b19fa85e273f1136f146417123db765e07d9")
		value5 := []byte{'e'}

		// 1000100101101011...
		// [137 107 114 103 127 66 78 29 245 251 184 114 188 52 189 152 213 188 241 208 116 119 85 191 22 78 180 63 44 160 38 59]
		key6, _ := hex.DecodeString("896b72677f424e1df5fbb872bc34bd98d5bcf1d0747755bf164eb43f2ca0263b")
		value6 := []byte{'f'}

		// 1001011010010110...
		// [150 150 209 192 186 239 247 115 79 117 170 17 239 248 191 67 250 10 39 181 48 241 154 52 125 139 99 28 178 151 3 126]
		key7, _ := hex.DecodeString("9696d1c0baeff7734f75aa11eff8bf43fa0a27b530f19a347d8b631cb297037e")
		value7 := []byte{'g'}

		// 1110111110110011...
		// [239 179 85 63 28 189 73 141 88 84 129 83 43 239 117 134 122 226 174 84 247 120 179 210 243 99 134 91 230 40 123 105]
		key8, _ := hex.DecodeString("efb3553f1cbd498d585481532bef75867ae2ae54f778b3d2f363865be6287b69")
		value8 := []byte{'h'}

		// 1000100101101011...
		// [137 107 114 103 127 66 78 29 245 251 184 114 188 52 189 152 213 188 241 208 116 119 85 191 22 78 180 63 44 160 38 59] [10001001 1101011 1110010 1100111 1111111 1000010 1001110 11101 11110101 11111011 10111000 1110010 10111100 110100 10111101 10011000 11010101 10111100 11110001 11010000 1110100 1110111 1010101 10111111 10110 1001110 10110100 111111 101100 10100000 100110 111011]
		key9, _ := hex.DecodeString("896b72677f424e1df5fbb872bc34bd98d5bcf1d0747755bf164eb43f2ca0263b")
		// value9 := []byte{'i'}

		keys := [][]byte{key1, key2, key3, key4, key5}
		values := [][]byte{value1, value2, value3, value4, value5}

		newRoot, err := smt.Update(keys, values, emptyTree.root)
		require.NoError(t, err)

		keys = [][]byte{key1, key2, key3, key4, key5}
		_, _, err = smt.Read(keys, true, newRoot)
		require.NoError(t, err)

		_, _, err = smt.Read([][]byte{key4}, true, newRoot)
		require.NoError(t, err)

		_, _, err = smt.Read([][]byte{key6}, true, newRoot)
		require.NoError(t, err)

		_, _, err = smt.Read([][]byte{key8}, true, newRoot)
		require.NoError(t, err)

		keys = [][]byte{key6, key7, key8}
		values = [][]byte{value6, value7, value8}

		newRoot2, err := smt.Update(keys, values, newRoot)
		require.NoError(t, err)

		keys = [][]byte{key4, key5, key8, key9}
		_, _, err = smt.Read(keys, false, newRoot2)
		pholder, _ := smt.GetBatchProof(keys, newRoot2)
		require.NoError(t, err)
		// TODO furthure checks
		require.NotNil(t, pholder)

	})
}

func TestRandomUpdateRead(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1

	withSMT(t, trieHeight, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

		// insert some values to an empty trie
		keys := make([][]byte, 0)
		values := make([][]byte, 0)

		rand.Seed(time.Now().UnixNano())

		// numberOfKeys := rand.Intn(256) + 1
		numberOfKeys := rand.Intn(30) + 1
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

		root, err := smt.Update(insertKeys, insertValues, emptyTree.root)
		require.NoError(t, err, "error updating trie")

		retvalues, _, err := smt.Read(insertKeys, true, root)
		require.NoError(t, err, "error reading values")

		for i := range retvalues {
			if !bytes.Equal(retvalues[i], insertValues[i]) {
				t.Fatalf("returned values doesn't match for key [%v], expected [%v] got [%v]", insertKeys[i], insertValues[i], retvalues[i])
			}
		}
	})

}

func withSMT(
	t *testing.T,
	height int,
	interval uint64,
	numHistoricalStates int,
	numFullStates int, f func(t *testing.T, smt *SMT, emptyTree *tree)) {

	dbDir := unittest.TempDir(t)

	trie, err := NewSMT(dbDir, height, interval, numHistoricalStates, numFullStates)
	require.NoError(t, err)

	defer func() {
		trie.SafeClose()
		os.RemoveAll(dbDir)
	}()

	emptyTree, err := trie.forest.Get(GetDefaultHashForHeight(height - 1))
	require.NoError(t, err)
	f(t, trie, emptyTree)

}
