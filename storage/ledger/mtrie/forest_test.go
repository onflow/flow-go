package mtrie_test

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestEmptyInsert(t *testing.T) {
	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(53), uint8(74)})
	v1 := []byte{'A'}
	keys := [][]byte{k1}
	values := [][]byte{v1}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)
	retValues, err := fStore.Read(keys, rootHash)
	require.NoError(t, err)
	require.True(t, bytes.Equal(retValues[0], values[0]))
}

func TestLeftEmptyInsert(t *testing.T) {
	//////////////////////
	//     insert X     //
	//       ()         //
	//      /  \        //
	//    (X)  [~]      //
	//////////////////////
	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash1 := fStore.GetEmptyRootHash()

	// key: 1000...
	k1 := []byte([]uint8{uint8(129), uint8(1)})
	// key: 1100....
	k2 := []byte([]uint8{uint8(193), uint8(1)})

	v1 := []byte{'A'}
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash2, err := fStore.Update(keys, values, rootHash1)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([],)[1]
	// 			14: ([129 1],41)[10]
	// 			14: ([193 1],42)[11]

	k3 := []byte([]uint8{uint8(1), uint8(1)})
	v3 := []byte{'C'}
	keys = [][]byte{k3}
	values = [][]byte{v3}
	rootHash3, err := fStore.Update(keys, values, rootHash2)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([1 1],43)[0]
	// 		15: ([],)[1]
	// 			14: ([129 1],41)[10]
	// 			14: ([193 1],42)[11]

	keys = [][]byte{k1, k2, k3}
	values = [][]byte{v1, v2, v3}
	retValues, err := fStore.Read(keys, rootHash3)
	require.NoError(t, err)
	for i := range keys {
		require.True(t, bytes.Equal(retValues[i], values[i]))
	}
}

func TestRightEmptyInsert(t *testing.T) {
	///////////////////////
	//     insert X      //
	//       ()          //
	//      /  \         //
	//    [~]  (X)       //
	///////////////////////
	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash1 := fStore.GetEmptyRootHash()

	// key: 1000...
	k1 := []byte([]uint8{uint8(129), uint8(1)})
	// key: 1100....
	k2 := []byte([]uint8{uint8(193), uint8(1)})

	v1 := []byte{'A'}
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash2, err := fStore.Update(keys, values, rootHash1)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([],)[1]
	// 			14: ([129 1],41)[10]
	// 			14: ([193 1],42)[11]

	k3 := []byte([]uint8{uint8(1), uint8(1)})
	v3 := []byte{'C'}
	keys = [][]byte{k3}
	values = [][]byte{v3}
	rootHash3, err := fStore.Update(keys, values, rootHash2)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([1 1],43)[0]
	// 		15: ([],)[1]
	// 			14: ([129 1],41)[10]
	// 			14: ([193 1],42)[11]

	keys = [][]byte{k1, k2, k3}
	values = [][]byte{v1, v2, v3}
	retValues, err := fStore.Read(keys, rootHash3)
	require.NoError(t, err)
	for i := range keys {
		require.True(t, bytes.Equal(retValues[i], values[i]))
	}
}

func TestExpansionInsert(t *testing.T) {
	//////////////////////
	//  insert ~'       //
	//       ()         //
	//      /  \        //
	//     ()  [~]      //
	//////////////////////

	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash1 := fStore.GetEmptyRootHash()

	// key: 1000...
	k1 := []byte([]uint8{uint8(129), uint8(1)})
	v1 := []byte{'A'}
	keys := [][]byte{k1}
	values := [][]byte{v1}
	rootHash2, err := fStore.Update(keys, values, rootHash1)
	require.NoError(t, err)
	// expected trie:
	// 16: ([129 1],41)[]

	// key: 1111....
	k2 := []byte([]uint8{uint8(130), uint8(1)})
	v2 := []byte{'B'}
	keys = [][]byte{k2}
	values = [][]byte{v2}
	rootHash3, err := fStore.Update(keys, values, rootHash2)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([],)[1]
	// 			14: ([],)[10]
	// 				13: ([],)[100]
	// 					12: ([],)[1000]
	// 						11: ([],)[10000]
	// 							10: ([],)[100000]
	// 								9: ([129 1],41)[1000000]
	// 								9: ([130 1],42)[1000001]

	keys = [][]byte{k1, k2}
	values = [][]byte{v1, v2}
	retValues, err := fStore.Read(keys, rootHash3)
	require.NoError(t, err)
	for i := range keys {
		require.True(t, bytes.Equal(retValues[i], values[i]))
	}
}

func TestFullHouseInsert(t *testing.T) {
	///////////////////////
	//   insert ~1<X<~2  //
	//       ()          //
	//      /  \         //
	//    [~1]  [~2]     //
	///////////////////////

	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash1 := fStore.GetEmptyRootHash()

	// key: 1000...
	k1 := []byte([]uint8{uint8(129), uint8(1)})
	// key: 1100....
	k2 := []byte([]uint8{uint8(193), uint8(1)})

	v1 := []byte{'A'}
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash2, err := fStore.Update(keys, values, rootHash1)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([],)[1]
	// 			14: ([129 1],41)[10]
	// 			14: ([193 1],42)[11]

	k3 := []byte([]uint8{uint8(160), uint8(1)})
	v3 := []byte{'C'}
	keys = [][]byte{k3}
	values = [][]byte{v3}
	rootHash3, err := fStore.Update(keys, values, rootHash2)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([],)[1]
	// 			14: ([],)[10]
	// 				13: ([129 1],41)[100]
	// 				13: ([160 1],43)[101]
	// 			14: ([193 1],42)[11]

	keys = [][]byte{k1, k2, k3}
	values = [][]byte{v1, v2, v3}
	retValues, err := fStore.Read(keys, rootHash3)
	require.NoError(t, err)
	for i := range keys {
		require.True(t, bytes.Equal(retValues[i], values[i]))
	}
}

func TestLeafInsert(t *testing.T) {
	///////////////////////
	//   insert 1, 2     //
	//       ()          //
	//      /  \         //
	//     ()   ...      //
	//          /  \     //
	//         ()  ()    //
	///////////////////////
	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash1 := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(1), uint8(0)})
	k2 := []byte([]uint8{uint8(1), uint8(1)})

	v1 := []byte{'A'}
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash2, err := fStore.Update(keys, values, rootHash1)
	require.NoError(t, err)
	// expected trie:
	// 16: ([],)[]
	// 		15: ([],)[0]
	// 			14: ([],)[00]
	//               ...
	// 					1: ([],)[000000010000000]
	// 						0: ([1 0],41)[0000000100000000]
	// 						0: ([1 1],42)[0000000100000001]

	retValues, err := fStore.Read(keys, rootHash2)
	require.NoError(t, err)
	for i := range keys {
		require.True(t, bytes.Equal(retValues[i], values[i]))
	}
}

func TestSameKeyInsert(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(53), uint8(74)})
	v1 := []byte{'A'}
	k2 := []byte([]uint8{uint8(116), uint8(129)})
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)
	k3 := []byte([]uint8{uint8(53), uint8(74)})
	v3 := []byte{'C'}
	keys = [][]byte{k3}
	values = [][]byte{v3}
	rootHash, err = fStore.Update(keys, values, rootHash)
	require.NoError(t, err)

	retValues, err := fStore.Read(keys, rootHash)
	require.NoError(t, err)
	require.True(t, bytes.Equal(retValues[0], values[0]))
}

func TestUpdateWithWrongKeySize(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	// short key
	key1 := make([]byte, 1)
	utils.SetBit(key1, 5)
	value1 := []byte{'a'}
	keys := [][]byte{key1}
	values := [][]byte{value1}

	rootHash, err := fStore.Update(keys, values, rootHash)
	require.Error(t, err)

	// long key
	key2 := make([]byte, 33)
	utils.SetBit(key2, 5)
	value2 := []byte{'a'}
	keys = [][]byte{key2}
	values = [][]byte{value2}

	rootHash, err = fStore.Update(keys, values, rootHash)
	require.Error(t, err)
}

// TODO insert with duplicated keys

func TestReadOrder(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(116), uint8(74)})
	v1 := []byte{'A'}
	k2 := []byte([]uint8{uint8(53), uint8(129)})
	v2 := []byte{'B'}
	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)

	retValues, err := fStore.Read(keys, rootHash)
	require.NoError(t, err)
	require.True(t, bytes.Equal(retValues[0], values[0]))
	require.True(t, bytes.Equal(retValues[1], values[1]))
}

func TestReadWithDupplicatedKeys(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(53), uint8(74)})
	v1 := []byte{'A'}
	k2 := []byte([]uint8{uint8(116), uint8(129)})
	v2 := []byte{'B'}
	k3 := []byte([]uint8{uint8(53), uint8(74)})

	keys := [][]byte{k1, k2}
	values := [][]byte{v1, v2}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)

	keys = [][]byte{k1, k2, k3}
	values = [][]byte{v1, v2, v1}

	retValues, err := fStore.Read(keys, rootHash)
	require.NoError(t, err)
	require.True(t, bytes.Equal(retValues[0], values[0]))
	require.True(t, bytes.Equal(retValues[1], values[1]))
	require.True(t, bytes.Equal(retValues[2], values[2]))
}

func TestReadNonExistKey(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(53), uint8(74)})
	v1 := []byte{'A'}
	keys := [][]byte{k1}
	values := [][]byte{v1}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)

	k2 := []byte([]uint8{uint8(116), uint8(129)})
	v2 := []byte{'B'}

	keys = [][]byte{k2}
	values = [][]byte{v2}
	retValues, err := fStore.Read(keys, rootHash)
	require.NoError(t, err)
	require.Equal(t, len(retValues[0]), 0)
}

func TestReadWithWrongKeySize(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	// setup
	key1 := make([]byte, 2)
	utils.SetBit(key1, 5)
	value1 := []byte{'a'}
	keys := [][]byte{key1}
	values := [][]byte{value1}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)

	// wrong key size
	key2 := make([]byte, 33)
	utils.SetBit(key2, 5)
	keys = [][]byte{key2}
	_, err = fStore.Read(keys, rootHash)
	require.Error(t, err)
}

// TODO test read (multiple non exist in a branch)

func TestRandomUpdateReadProof(t *testing.T) {
	keyByteSize := 2
	trieHeight := keyByteSize*8 + 1
	rep := 100
	maxNumKeysPerStep := 10
	maxValueSize := 6 // bytes
	rand.Seed(time.Now().UnixNano())
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()
	// map store
	latestValueByKey := make(map[string][]byte, 0)

	for e := 0; e < rep; e++ {
		keys := mtrie.GetRandomKeysRandN(maxNumKeysPerStep, keyByteSize)
		values := mtrie.GetRandomValues(len(keys), maxValueSize)

		// update map store with key values
		// we use this at the end of each step to check all existing keys
		for i, k := range keys {
			latestValueByKey[string(k)] = values[i]
		}

		// test reading for non-existing keys
		nonExistingKeys := make([][]byte, 0)
		for _, k := range keys {
			if _, ok := latestValueByKey[string(k)]; !ok {
				nonExistingKeys = append(nonExistingKeys, k)
			}
		}
		retValues, err := fStore.Read(nonExistingKeys, rootHash)
		require.NoError(t, err, "error reading - non existing keys")
		for i := range retValues {
			require.True(t, len(retValues[i]) == 0)
		}

		// test update
		rootHash, err = fStore.Update(keys, values, rootHash)
		require.NoError(t, err, "error updating")

		// test read
		retValues, err = fStore.Read(keys, rootHash)
		require.NoError(t, err, "error reading")
		for i := range values {
			require.True(t, bytes.Equal(values[i], retValues[i]))
		}

		// test proof
		batchProof, err := fStore.Proofs(keys, rootHash)
		require.NoError(t, err, "error generating proofs")
		require.True(t, batchProof.Verify(keys, values, rootHash, trieHeight))

		psmt, err := trie.NewPSMT(rootHash, trieHeight, keys, values, mtrie.EncodeBatchProof(batchProof))
		require.True(t, bytes.Equal(psmt.GetRootHash(), rootHash))
		require.NoError(t, err, "error building partial trie")

		// check values for all existing keys
		allKeys := make([][]byte, 0, len(latestValueByKey))
		allValues := make([][]byte, 0, len(latestValueByKey))
		for k, v := range latestValueByKey {
			allKeys = append(allKeys, []byte(k))
			allValues = append(allValues, v)
		}
		retValues, err = fStore.Read(allKeys, rootHash)
		for i, v := range allValues {
			require.True(t, bytes.Equal(v, retValues[i]))
		}

		// TODO test mix of proofs (existing and non existing)

	}
}

func TestProofGenerationInclusion(t *testing.T) {
	trieHeight := 17
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	k1 := []byte([]uint8{uint8(1), uint8(74)})
	v1 := []byte{'A'}

	k2 := []byte([]uint8{uint8(2), uint8(74)})
	v2 := []byte{'B'}

	k3 := []byte([]uint8{uint8(130), uint8(74)})
	v3 := []byte{'C'}

	k4 := []byte([]uint8{uint8(131), uint8(74)})
	v4 := []byte{'D'}

	keys := [][]byte{k1, k2, k3, k4}
	values := [][]byte{v1, v2, v3, v4}
	rootHash, err := fStore.Update(keys, values, rootHash)
	require.NoError(t, err)
	proof, err := fStore.Proofs(keys, rootHash)
	require.NoError(t, err)
	require.True(t, proof.Verify(keys, values, rootHash, trieHeight))
}

func TestMForestAccuracy(t *testing.T) {
	trieHeight := 17 // should be key size (in bits) + 1
	experimentRep := 10
	fStore := mtrie.NewMForest(trieHeight)
	rootHash := fStore.GetEmptyRootHash()

	dbDir := unittest.TempDir(t)
	smt, err := trie.NewSMT(dbDir, trieHeight, 10, 100, experimentRep)
	require.NoError(t, err)
	defer func() {
		smt.SafeClose()
		os.RemoveAll(dbDir)
	}()

	emptyTree := trie.GetDefaultHashForHeight(trieHeight - 1)
	require.NoError(t, err)
	rootHashForSMT := emptyTree
	for e := 0; e < experimentRep; e++ {
		// insert some values to an empty trie
		keys := make([][]byte, 0)
		values := make([][]byte, 0)
		rand.Seed(time.Now().UnixNano())

		// rejection sampling
		numberOfKeys := rand.Intn(20) + 1
		keyValueMap := make(map[string][]byte)
		i := 0
		for i < numberOfKeys {
			key := make([]byte, 2)
			rand.Read(key)
			// deduplicate
			if _, found := keyValueMap[string(key)]; !found {
				keys = append(keys, key)
				value := make([]byte, 4)
				rand.Read(value)
				keyValueMap[string(key)] = value
				values = append(values, value)
				i++
			}
		}

		newRootHash, err := fStore.Update(keys, values, rootHash)
		require.NoError(t, err, "error commiting changes")
		rootHash = newRootHash

		// check values
		retValues, err := fStore.Read(keys, rootHash)
		require.NoError(t, err)
		for i, k := range keys {
			require.True(t, bytes.Equal(keyValueMap[string(k)], retValues[i]))
		}

		// Test eqaulity to SMT
		newRootHashForSMT, err := smt.Update(keys, values, rootHashForSMT)
		require.NoError(t, err)
		rootHashForSMT = newRootHashForSMT
		require.True(t, bytes.Equal(newRootHashForSMT, newRootHash))

		// TODO test proofs for non-existing keys
		batchProof, err := fStore.Proofs(keys, rootHash)
		require.NoError(t, err, "error generating proofs")

		batchProofSMT, err := smt.GetBatchProof(keys, rootHashForSMT)
		require.NoError(t, err, "error generating proofs (SMT)")

		encodedProof := mtrie.EncodeBatchProof(batchProof)
		encodedProofSMT := trie.EncodeProof(batchProofSMT)

		for i := range encodedProof {
			require.True(t, bytes.Equal(encodedProof[i], encodedProofSMT[i]))
		}

		psmt, err := trie.NewPSMT(rootHash, trieHeight, keys, values, encodedProof)
		require.True(t, bytes.Equal(psmt.GetRootHash(), rootHash))
		require.NoError(t, err, "error building partial trie")

	}
}
