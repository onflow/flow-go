package ptrie

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/module/metrics"
)

func withForest(
	t *testing.T,
	pathByteSize int,
	numberOfActiveTries int, f func(t *testing.T, f *mtrie.Forest)) {

	forest, err := mtrie.NewForest(pathByteSize, numberOfActiveTries, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	f(t, forest)
}

func TestPartialTrieEmptyTrie(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		// add path1 to the empty trie
		// 00000000...0 (0)
		path1 := utils.PathByUint16(0)
		payload1 := utils.LightPayload('A', 'a')

		paths := []ledger.Path{path1}
		payloads := []*ledger.Payload{payload1}

		rootHash := f.GetEmptyRootHash()
		r := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
		bp, err := f.Proofs(r)
		require.NoError(t, err, "error getting proofs values")

		psmt, err := NewPSMT(rootHash, pathByteSize, bp)

		require.NoError(t, err, "error building partial trie")
		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before set]")
		}
		u := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
		rootHash, err = f.Update(u)
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after set]")
		}

		updatedPayload1 := utils.LightPayload('B', 'b')
		payloads = []*ledger.Payload{updatedPayload1}

		u = &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
		rootHash, err = f.Update(u)
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}

	})
}

func TestPartialTrieLeafUpdates(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := utils.PathByUint16(0)
		payload1 := utils.LightPayload('A', 'a')
		updatedPayload1 := utils.LightPayload('B', 'b')

		path2 := utils.PathByUint16(1)
		payload2 := utils.LightPayload('C', 'c')
		updatedPayload2 := utils.LightPayload('D', 'd')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		u := &ledger.TrieUpdate{RootHash: f.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
		rootHash, err := f.Update(u)
		require.NoError(t, err, "error updating trie")

		r := &ledger.TrieRead{RootHash: rootHash, Paths: paths}
		bp, err := f.Proofs(r)
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, pathByteSize, bp)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2}
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieMiddleBranching(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := utils.PathByUint16(0)
		payload1 := utils.LightPayload('A', 'a')
		updatedPayload1 := utils.LightPayload('B', 'b')

		path2 := utils.PathByUint16(2)
		payload2 := utils.LightPayload('C', 'c')
		updatedPayload2 := utils.LightPayload('D', 'd')

		path3 := utils.PathByUint16(8)
		payload3 := utils.LightPayload('E', 'e')
		updatedPayload3 := utils.LightPayload('F', 'f')

		paths := []ledger.Path{path1, path2, path3}
		payloads := []*ledger.Payload{payload1, payload2, payload3}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, pathByteSize, bp)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(f.GetEmptyRootHash(), psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}
		// first update
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// second update
		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2, updatedPayload3}
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		_, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}
	})

}

func TestPartialTrieRootUpdates(t *testing.T) {

	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := utils.PathByUint16(0)
		payload1 := utils.LightPayload('A', 'a')
		updatedPayload1 := utils.LightPayload('B', 'b')
		//  10000....0
		path2 := utils.PathByUint16(32768)
		payload2 := utils.LightPayload('C', 'c')
		updatedPayload2 := utils.LightPayload('D', 'd')

		paths := []ledger.Path{path1, path2}
		payloads := []*ledger.Payload{payload1, payload2}

		rootHash := f.GetEmptyRootHash()
		bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, pathByteSize, bp)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		// first update
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, err := psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(rootHash, pRootHash) {
			t.Fatal("rootNode hash doesn't match [after update]")
		}

		// second update
		payloads = []*ledger.Payload{updatedPayload1, updatedPayload2}
		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, err = psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating psmt")
		if !bytes.Equal(rootHash, pRootHash) {
			t.Fatal("rootNode hash doesn't match [after second update]")
		}
	})

}

func TestMixProof(t *testing.T) {
	pathByteSize := 32
	withForest(t, pathByteSize, 10, func(t *testing.T, f *mtrie.Forest) {

		path1 := utils.PathByUint16(0)
		payload1 := utils.LightPayload('A', 'a')

		path2 := utils.PathByUint16(2)
		updatedPayload2 := utils.LightPayload('D', 'd')

		path3 := utils.PathByUint16(8)
		payload3 := utils.LightPayload('E', 'e')

		paths := []ledger.Path{path1, path3}
		payloads := []*ledger.Payload{payload1, payload3}

		rootHash := f.GetEmptyRootHash()
		rootHash, err := f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		paths = []ledger.Path{path1, path2, path3}

		bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
		require.NoError(t, err, "error getting batch proof")

		psmt, err := NewPSMT(rootHash, pathByteSize, bp)
		require.NoError(t, err, "error building partial trie")

		if !bytes.Equal(rootHash, psmt.root.HashValue()) {
			t.Fatal("rootNode hash doesn't match [before update]")
		}

		paths = []ledger.Path{path2, path3}
		payloads = []*ledger.Payload{updatedPayload2, updatedPayload2}

		rootHash, err = f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads})
		require.NoError(t, err, "error updating trie")

		pRootHash, err := psmt.Update(paths, payloads)
		require.NoError(t, err, "error updating partial trie")

		if !bytes.Equal(rootHash, pRootHash) {
			t.Fatalf("root2 hash doesn't match [%x] != [%x]", rootHash, pRootHash)
		}

	})

}

func TestRandomProofs(t *testing.T) {
	pathByteSize := 32 // key size of 16 bits
	minPayloadSize := 2
	maxPayloadSize := 10
	experimentRep := 20
	for e := 0; e < experimentRep; e++ {
		withForest(t, pathByteSize, experimentRep+1, func(t *testing.T, f *mtrie.Forest) {

			// generate some random paths and payloads
			seed := time.Now().UnixNano()
			rand.Seed(seed)
			t.Logf("rand seed is %x", seed)
			numberOfPaths := rand.Intn(256) + 1
			paths := utils.RandomPaths(numberOfPaths, pathByteSize)
			payloads := utils.RandomPayloads(numberOfPaths, minPayloadSize, maxPayloadSize)
			// keep a subset as initial insert and keep the rest for reading default values
			split := rand.Intn(numberOfPaths)
			insertPaths := paths[:split]
			insertPayloads := payloads[:split]

			rootHash, err := f.Update(&ledger.TrieUpdate{RootHash: f.GetEmptyRootHash(), Paths: insertPaths, Payloads: insertPayloads})
			require.NoError(t, err, "error updating trie")

			// shuffle paths for read
			rand.Shuffle(len(paths), func(i, j int) {
				paths[i], paths[j] = paths[j], paths[i]
				payloads[i], payloads[j] = payloads[j], payloads[i]
			})

			bp, err := f.Proofs(&ledger.TrieRead{RootHash: rootHash, Paths: paths})
			require.NoError(t, err, "error getting batch proof")

			psmt, err := NewPSMT(rootHash, pathByteSize, bp)
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

			rootHash2, err := f.Update(&ledger.TrieUpdate{RootHash: rootHash, Paths: updatePaths, Payloads: updatePayloads})
			require.NoError(t, err, "error updating trie")

			pRootHash2, err := psmt.Update(updatePaths, updatePayloads)
			require.NoError(t, err, "error updating partial trie")

			if !bytes.Equal(rootHash2, pRootHash2) {
				t.Fatalf("root2 hash doesn't match [%x] != [%x]", rootHash2, pRootHash2)
			}

		})
	}
}

// TODO add test for incompatible proofs [Byzantine milestone]
// TODO add test key not exist [Byzantine milestone]
