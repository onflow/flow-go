package pebble

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
)

const defaultRegisterValue = byte('v')

func TestRegisterBootstrap_NewBootstrap(t *testing.T) {
	t.Parallel()
	unittest.RunWithTempDir(t, func(dir string) {
		rootHeight := uint64(1)
		log := zerolog.New(io.Discard)
		cache := pebble.NewCache(1 << 20)
		defer cache.Unref()
		opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
		p, err := pebble.Open(dir, opts)
		require.NoError(t, err)
		// set heights
		require.NoError(t, p.Set(firstHeightKey(), encodedUint64(rootHeight), nil))
		require.NoError(t, p.Set(latestHeightKey(), encodedUint64(rootHeight), nil))
		// errors if FirstHeight or LastHeight are populated
		_, err = NewRegisterBootstrap(p, dir, rootHeight, log)
		require.ErrorContains(t, err, "cannot bootstrap populated DB")
	})
}

func TestRegisterBootstrap_IndexCheckpointFile_Happy(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	unittest.RunWithTempDir(t, func(dir string) {
		tries, registerIDs := simpleTrieWithValidRegisterIDs(t)
		fileName := "simple-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log), "fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)

		cache := pebble.NewCache(1 << 20)
		opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
		defer cache.Unref()
		pb, dbDir := unittest.TempPebbleDBWithOpts(t, opts)

		bootstrap, err := NewRegisterBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background())
		require.NoError(t, err)

		// create registers instance and check values
		reg, err := NewRegisters(pb)
		require.NoError(t, err)

		require.Equal(t, reg.LatestHeight(), rootHeight)
		require.Equal(t, reg.FirstHeight(), rootHeight)

		for _, register := range registerIDs {
			val, err := reg.Get(*register, rootHeight)
			require.NoError(t, err)
			require.Equal(t, val, []byte{defaultRegisterValue})
		}

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

func TestRegisterBootstrap_IndexCheckpointFile_Empty(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	unittest.RunWithTempDir(t, func(dir string) {
		tries := []*trie.MTrie{trie.NewEmptyMTrie()}
		fileName := "empty-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log), "fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		cache := pebble.NewCache(1 << 20)
		opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
		defer cache.Unref()
		pb, dbDir := unittest.TempPebbleDBWithOpts(t, opts)
		bootstrap, err := NewRegisterBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background())
		require.NoError(t, err)

		// create registers instance and check values
		reg, err := NewRegisters(pb)
		require.NoError(t, err)

		require.Equal(t, reg.LatestHeight(), rootHeight)
		require.Equal(t, reg.FirstHeight(), rootHeight)

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

func TestRegisterBootstrap_IndexCheckpointFile_FormatIssue(t *testing.T) {
	t.Parallel()
	pa1 := testutils.PathByUint8(0)
	pa2 := testutils.PathByUint8(1)
	rootHeight := uint64(666)
	pl1 := testutils.LightPayload8('A', 'A')
	pl2 := testutils.LightPayload('B', 'B')
	paths := []ledger.Path{pa1, pa2}
	payloads := []ledger.Payload{*pl1, *pl2}
	emptyTrie := trie.NewEmptyMTrie()
	trieWithInvalidEntry, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	log := zerolog.New(io.Discard)

	unittest.RunWithTempDir(t, func(dir string) {
		fileName := "invalid-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{trieWithInvalidEntry}, dir, fileName, log),
			"fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		cache := pebble.NewCache(1 << 20)
		opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
		defer cache.Unref()

		pb, dbDir := unittest.TempPebbleDBWithOpts(t, opts)
		bootstrap, err := NewRegisterBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background())
		require.ErrorContains(t, err, "unexpected ledger key format")
		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

}

func TestRegisterBootstrap_IndexCheckpointFile_CorruptedCheckpointFile(t *testing.T) {
	t.Parallel()
	rootHeight := uint64(666)
	log := zerolog.New(io.Discard)
	unittest.RunWithTempDir(t, func(dir string) {
		tries, _ := largeTrieWithValidRegisterIDs(t)
		checkpointFileName := "large-checkpoint-incomplete"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, checkpointFileName, log), "fail to store checkpoint")
		// delete 2nd part of the file (2nd subtrie)
		fileToDelete := path.Join(dir, fmt.Sprintf("%v.%03d", checkpointFileName, 2))
		err := os.RemoveAll(fileToDelete)
		require.NoError(t, err)
		cache := pebble.NewCache(1 << 20)
		opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
		defer cache.Unref()
		pb, _ := unittest.TempPebbleDBWithOpts(t, opts)
		bootstrap, err := NewRegisterBootstrap(pb, checkpointFileName, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background())
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestRegisterBootstrap_IndexCheckpointFile_MultipleBatch(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	unittest.RunWithTempDir(t, func(dir string) {
		tries, registerIDs := largeTrieWithValidRegisterIDs(t)
		fileName := "large-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log), "fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		cache := pebble.NewCache(1 << 20)
		opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
		defer cache.Unref()
		pb, dbDir := unittest.TempPebbleDBWithOpts(t, opts)
		bootstrap, err := NewRegisterBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background())
		require.NoError(t, err)

		// create registers instance and check values
		reg, err := NewRegisters(pb)
		require.NoError(t, err)

		require.Equal(t, reg.LatestHeight(), rootHeight)
		require.Equal(t, reg.FirstHeight(), rootHeight)

		for _, register := range registerIDs {
			val, err := reg.Get(*register, rootHeight)
			require.NoError(t, err)
			require.Equal(t, val, []byte{defaultRegisterValue})
		}

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

}

func simpleTrieWithValidRegisterIDs(t *testing.T) ([]*trie.MTrie, []*flow.RegisterID) {
	return trieWithValidRegisterIDs(t, 2)
}

func largeTrieWithValidRegisterIDs(t *testing.T) ([]*trie.MTrie, []*flow.RegisterID) {
	// large enough trie so every worker should have something to index
	largeTrieSize := 2 * pebbleBootstrapRegisterBatchLen * pebbleBootstrapWorkerCount
	return trieWithValidRegisterIDs(t, uint16(largeTrieSize))
}

func trieWithValidRegisterIDs(t *testing.T, n uint16) ([]*trie.MTrie, []*flow.RegisterID) {
	emptyTrie := trie.NewEmptyMTrie()
	resultRegisterIDs := make([]*flow.RegisterID, 0, n)
	paths := randomRegisterPaths(n)
	payloads := randomRegisterPayloads(n)
	for _, payload := range payloads {
		key, err := payload.Key()
		require.NoError(t, err)
		regID, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)
		resultRegisterIDs = append(resultRegisterIDs, &regID)
	}
	populatedTrie, depth, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	// make sure it has at least 1 leaf node
	require.GreaterOrEqual(t, depth, uint16(1))
	require.NoError(t, err)
	resultTries := []*trie.MTrie{emptyTrie, populatedTrie}
	return resultTries, resultRegisterIDs
}

func randomRegisterPayloads(n uint16) []ledger.Payload {
	p := make([]ledger.Payload, 0, n)
	for i := uint16(0); i < n; i++ {
		o := make([]byte, 0, 8)
		o = binary.BigEndian.AppendUint16(o, n)
		k := ledger.Key{KeyParts: []ledger.KeyPart{
			{Type: convert.KeyPartOwner, Value: o},
			{Type: convert.KeyPartKey, Value: o},
		}}
		// values are always 'v' for ease of testing/checking
		v := ledger.Value{defaultRegisterValue}
		pl := ledger.NewPayload(k, v)
		p = append(p, *pl)
	}
	return p
}

func randomRegisterPaths(n uint16) []ledger.Path {
	p := make([]ledger.Path, 0, n)
	for i := uint16(0); i < n; i++ {
		p = append(p, testutils.PathByUint16(i))
	}
	return p
}
