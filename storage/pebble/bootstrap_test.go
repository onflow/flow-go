package pebble

import (
	"crypto/rand"
	"io"
	"path"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/component"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBootstrap_NewBootstrap(t *testing.T) {
	t.Parallel()
	sampleDir := path.Join(unittest.TempDir(t), "checkpoint.checkpoint")
	rootHeight := uint64(1)
	log := zerolog.New(io.Discard)
	cache := pebble.NewCache(1 << 20)
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	unittest.RunWithConfiguredPebbleInstance(t, opts, func(p *pebble.DB) {
		// no issues when pebble instance is blank
		_, err := NewBootstrap(p, sampleDir, rootHeight, log)
		require.NoError(t, err)
		// set heights
		require.NoError(t, p.Set(firstHeightKey(), encodedUint64(rootHeight), nil))
		require.NoError(t, p.Set(latestHeightKey(), encodedUint64(rootHeight), nil))
		// errors if FirstHeight or LastHeight are populated
		_, err = NewBootstrap(p, sampleDir, rootHeight, log)
		require.ErrorContains(t, err, "cannot bootstrap populated DB")
	})
}

func TestBootstrap_IndexCheckpointFile_Random(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	// write empty trie
	unittest.RunWithTempDir(t, func(dir string) {
		t.Parallel()
		fileName := "empty-checkpoint"
		emptyTrie := []*trie.MTrie{trie.NewEmptyMTrie()}
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(emptyTrie, dir, fileName, log), "fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		unittest.RunWithConfiguredPebbleInstance(t, getTestingPebbleOpts(), func(p *pebble.DB) {
			bootstrap, err := NewBootstrap(p, checkpointFile, rootHeight, log)
			require.NoError(t, err)
			cm := component.NewComponentManagerBuilder().AddWorker(bootstrap.IndexCheckpointFile).Build()
			<-cm.Done()
		})
	})

	unittest.RunWithTempDir(t, func(dir string) {
		t.Parallel()
		tries := createSimpleTrie(t)
		fileName := "simple-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log), "fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		unittest.RunWithConfiguredPebbleInstance(t, getTestingPebbleOpts(), func(p *pebble.DB) {
			bootstrap, err := NewBootstrap(p, checkpointFile, rootHeight, log)
			require.NoError(t, err)
			cm := component.NewComponentManagerBuilder().AddWorker(bootstrap.IndexCheckpointFile).Build()
			<-cm.Done()
		})
	})

	unittest.RunWithTempDir(t, func(dir string) {
		t.Parallel()
		tries := createMultipleRandomTriesMini(t)
		fileName := "random-checkpoint"
		checkpointFile := path.Join(dir, fileName)
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log), "fail to store checkpoint")
		unittest.RunWithConfiguredPebbleInstance(t, getTestingPebbleOpts(), func(p *pebble.DB) {
			bootstrap, err := NewBootstrap(p, checkpointFile, rootHeight, log)
			require.NoError(t, err)
			cm := component.NewComponentManagerBuilder().AddWorker(bootstrap.IndexCheckpointFile).Build()
			<-cm.Done()
		})
	})
}

func TestBootstrap_IndexCheckpointFile_Error(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		log := zerolog.New(io.Discard)
		// write trie and remove part of the file
		fileName := "simple-checkpoint"
		tries := createSimpleTrie(t)
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log), "fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)

		unittest.RunWithConfiguredPebbleInstance(t, getTestingPebbleOpts(), func(p *pebble.DB) {

		})
	})
}

func getTestingPebbleOpts() *pebble.Options {
	cache := pebble.NewCache(1 << 20)
	return DefaultPebbleOptions(cache, registers.NewMVCCComparer())
}

// Todo: Move these functions to somewhere common, this is from checkpoint_v6_test.go
func createSimpleTrie(t *testing.T) []*trie.MTrie {
	emptyTrie := trie.NewEmptyMTrie()

	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	tries := []*trie.MTrie{emptyTrie, updatedTrie}
	return tries
}

func randPathPayload() (ledger.Path, ledger.Payload) {
	var path ledger.Path
	_, err := rand.Read(path[:])
	if err != nil {
		panic("randomness failed")
	}
	payload := testutils.RandomPayload(1, 100)
	return path, *payload
}

func randNPathPayloads(n int) ([]ledger.Path, []ledger.Payload) {
	paths := make([]ledger.Path, n)
	payloads := make([]ledger.Payload, n)
	for i := 0; i < n; i++ {
		path, payload := randPathPayload()
		paths[i] = path
		payloads[i] = payload
	}
	return paths, payloads
}

func createMultipleRandomTriesMini(t *testing.T) []*trie.MTrie {
	tries := make([]*trie.MTrie, 0)
	activeTrie := trie.NewEmptyMTrie()

	var err error
	// add tries with no shared paths
	for i := 0; i < 5; i++ {
		paths, payloads := randNPathPayloads(20)
		activeTrie, _, err = trie.NewTrieWithUpdatedRegisters(activeTrie, paths, payloads, false)
		require.NoError(t, err, "update registers")
		tries = append(tries, activeTrie)
	}

	// add trie with some shared path
	sharedPaths, payloads1 := randNPathPayloads(10)
	activeTrie, _, err = trie.NewTrieWithUpdatedRegisters(activeTrie, sharedPaths, payloads1, false)
	require.NoError(t, err, "update registers")
	tries = append(tries, activeTrie)

	_, payloads2 := randNPathPayloads(10)
	activeTrie, _, err = trie.NewTrieWithUpdatedRegisters(activeTrie, sharedPaths, payloads2, false)
	require.NoError(t, err, "update registers")
	tries = append(tries, activeTrie)

	return tries
}
