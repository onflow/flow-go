package trie

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// kvdbPath is the path to the key-value database.
	kvdbPath = "db/valuedb"
	// tdbPath is the path to the trie database.
	tdbPath = "db/triedb"

	testHeight     = 256
	testHashLength = 32
	//cacheSize      = 50000
)

func TestSMTInitialization(t *testing.T) {

	withSMT(t, testHeight, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {

		if smt.GetHeight() != testHeight {
			t.Errorf("Height is %d; want %d", smt.GetHeight(), testHeight)
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
	dbDir := unittest.TempDBDir(t)

	defer func() {
		os.RemoveAll(dbDir)
	}()

	_, err := NewSMT(dbDir, -1, 10, 100, 10)

	require.Error(t, err, "Height error should have been thrown")
}

func TestInteriorNode(t *testing.T) {

	withSMT(t, 255, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
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
	withSMT(t, 255, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {

		batcher := emptyTree.database.NewBatcher()

		res := interiorNode(nil, nil, 200, batcher)

		require.Nil(t, res, "Interior node is not nil")
	})

}

func TestInteriorNodeLNil(t *testing.T) {

	withSMT(t, 255, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 255, 10, 100, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

func TestInsertIntoKey(t *testing.T) {
	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 10, 10, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {

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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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
	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 10, 100, 5, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 1, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 2, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 6, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

	withSMT(t, 8, 6, 6, 6, func(t *testing.T, smt *SMT, emptyTree *tree) {
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

		require.Equal(t, DecodeProof(EncodeProof(proofHldr)), proofHldr, "Proof Encoder has an issue")
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

		require.Equal(t, DecodeProof(EncodeProof(proofHldr)), proofHldr, "Proof Encoder has an issue")
	})
}

func withSMT(
	t *testing.T,
	height int,
//cacheSize int,
	interval uint32,
	numHistoricalStates int,
	numFullStates int, f func(t *testing.T, smt *SMT, emptyTree *tree)) {

	dbDir := unittest.TempDBDir(t)

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
