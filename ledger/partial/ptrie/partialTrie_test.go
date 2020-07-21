package ptrie

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie"
	"github.com/dapperlabs/flow-go/module/metrics"
)

func withForest(
	t *testing.T,
	pathByteSize int,
	numberOfActiveTries int, f func(t *testing.T, f *mtrie.Forest)) {

	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := mtrie.NewForest(pathByteSize, dir, numberOfActiveTries, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	f(t, forest)
}

func TestPartialTrieEmptyTrie(t *testing.T) {

	pathByteSize := 2
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		// add path1 to the empty trie
		// 00000000...0 (0)
		path1 := common.TwoBytesPath(0)
		payload1 := common.LightPayload('A', 'a')

		paths := []ledger.Path{path1}
		payloads := []*ledger.Payload{payload1}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{rootHash, paths})
		require.NoError(t, err, "error getting proofs values")

		encBP := common.EncodeTrieBatchProof(bp)
		psmt, err := NewPSMT(rootHash, pathByteSize, encBP)

		require.NoError(t, err, "error building partial trie")
		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before set]")
		}
		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after set]")
		}

		updatedPayload1 := common.LightPayload('B', 'b')
		payloads = []*ledger.Payload{updatedPayload1}

		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}

	})
}

func TestPartialTrieLeafUpdates(t *testing.T) {

	pathByteSize := 2
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := common.TwoBytesPath(0)
		payload1 := common.LightPayload('A', 'a')
		updatedPayload1 := common.LightPayload('B', 'b')

		path2 := common.TwoBytesPath(1)
		payload2 := common.LightPayload('C', 'c')
		updatedPayload2 := common.LightPayload('D', 'd')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		rootHash, err := f.Update(&ledger.TrieUpdate{f.GetEmptyRootHash(), paths, payloads})
		require.NoError(t, err, "error updating trie")

		bp, err := f.Proofs(&ledger.TrieRead{rootHash, paths})
		require.NoError(t, err, "error getting batch proof")

		encBP := common.EncodeTrieBatchProof(bp)
		psmt, err := NewPSMT(rootHash, pathByteSize, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2}
		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieMiddleBranching(t *testing.T) {

	pathByteSize := 2
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := common.TwoBytesPath(0)
		payload1 := common.LightPayload('A', 'a')
		updatedPayload1 := common.LightPayload('B', 'b')

		path2 := common.TwoBytesPath(2)
		payload2 := common.LightPayload('C', 'c')
		updatedPayload2 := common.LightPayload('D', 'd')

		path3 := common.TwoBytesPath(8)
		payload3 := common.LightPayload('E', 'e')
		updatedPayload3 := common.LightPayload('F', 'f')

		paths := []ledger.Path{path1, path2, path3}
		payloads := []*ledger.Payload{payload1, payload2, payload3}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{rootHash, paths})
		require.NoError(t, err, "error getting batch proof")

		encBP := common.EncodeTrieBatchProof(bp)
		psmt, err := NewPSMT(rootHash, pathByteSize, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(f.GetEmptyRootHash(), psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}
		// first update
		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// second update
		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2, updatedPayload3}
		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		_, _, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieRootUpdates(t *testing.T) {

	pathByteSize := 2
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := common.TwoBytesPath(0)
		payload1 := common.LightPayload('A', 'a')
		updatedPayload1 := common.LightPayload('B', 'b')
		//  10000....0
		path2 := common.TwoBytesPath(32768)
		payload2 := common.LightPayload('C', 'c')
		updatedPayload2 := common.LightPayload('D', 'd')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{rootHash, paths})
		require.NoError(t, err, "error getting batch proof")

		encBP := common.EncodeTrieBatchProof(bp)
		psmt, err := NewPSMT(rootHash, pathByteSize, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// first update
		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, _, err := psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(rootHash, pRootHash) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}

		// second update
		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2}
		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, _, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(rootHash, pRootHash) {
			t.Fatal("rootNode hash doesn't match [after second update]")
		}
	})

}

func TestMixProof(t *testing.T) {
	pathByteSize := 2
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := common.TwoBytesPath(0)
		payload1 := common.LightPayload('A', 'a')

		path2 := common.TwoBytesPath(2)
		payload2 := common.LightPayload('C', 'c')
		updatedPayload2 := common.LightPayload('D', 'd')

		path3 := common.TwoBytesPath(8)
		payload3 := common.LightPayload('E', 'e')

		paths := []ledger.Path{path1, path3}
		payloads := []*ledger.Payload{payload1, payload3}

		rootHash := f.GetEmptyRootHash()
		rootHash, err := f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		paths = []ledger.Path{path1, path2, path3}
		payloads = []*ledger.Payload{payload1, payload2, payload3}

		bp, err := f.Proofs(&ledger.TrieRead{rootHash, paths})
		require.NoError(t, err, "error getting batch proof")

		encBP := common.EncodeTrieBatchProof(bp)
		psmt, err := NewPSMT(rootHash, pathByteSize, encBP)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		paths = []ledger.Path{path2, path3}
		payloads = []*ledger.Payload{updatedPayload2, updatedPayload2}

		rootHash, err = f.Update(&ledger.TrieUpdate{rootHash, paths, payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, _, err := psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating partial trie")

		if !bytes.Equal(rootHash, pRootHash) {
			t.Fatalf("root2 hash doesn't match [%x] != [%x]", rootHash, pRootHash)
		}

	})

}

func TestRandomProofs(t *testing.T) {
	pathByteSize := 2 // key size of 16 bits
	experimentRep := 20
	for e := 0; e < experimentRep; e++ {
		withForest(t, pathByteSize, experimentRep+1, func(t *testing.T, f *mtrie.Forest) {

			// generate some random paths and payloads
			rand.Seed(time.Now().UnixNano())
			numberOfPaths := rand.Intn(256) + 1
			paths := common.GetRandomPaths(numberOfPaths, pathByteSize)
			payloads := common.RandomPayloads(numberOfPaths)
			// keep a subset as initial insert and keep the rest for reading default values
			split := rand.Intn(numberOfPaths)
			insertPaths := paths[:split]
			insertPayloads := payloads[:split]

			rootHash, err := f.Update(&ledger.TrieUpdate{f.GetEmptyRootHash(), insertPaths, insertPayloads})
			require.NoError(t, err, "error updating trie")

			// shuffle paths for read
			rand.Shuffle(len(paths), func(i, j int) {
				paths[i], paths[j] = paths[j], paths[i]
				payloads[i], payloads[j] = payloads[j], payloads[i]
			})

			bp, err := f.Proofs(&ledger.TrieRead{rootHash, paths})
			require.NoError(t, err, "error getting batch proof")

			encBP := common.EncodeTrieBatchProof(bp)
			psmt, err := NewPSMT(rootHash, pathByteSize, encBP)
			require.NoError(t, err, "error building partial trie")

			if !bytes.Equal(rootHash, psmt.root.HashValue()) {
				t.Fatal("root hash doesn't match")
			}

			// select a subset of shuffled paths for random updates
			split = rand.Intn(numberOfPaths)
			updatePaths := paths[:split]
			updatePayloads := payloads[:split]
			// random updates
			rand.Shuffle(len(updatePayloads), func(i, j int) {
				updatePayloads[i], updatePayloads[j] = updatePayloads[j], updatePayloads[i]
			})

			rootHash2, err := f.Update(&ledger.TrieUpdate{rootHash, updatePaths, updatePayloads})
			require.NoError(t, err, "error updating trie")

			pRootHash2, _, err := psmt.Update(updatePaths, updatePayloads)
			require.NoError(t, err, "error updating partial trie")

			if !bytes.Equal(rootHash2, pRootHash2) {
				t.Fatalf("root2 hash doesn't match [%x] != [%x]", rootHash2, pRootHash2)
			}

		})
	}
}

// TODO add test for incompatible proofs [Byzantine milestone]
// TODO add test key not exist [Byzantine milestone]
