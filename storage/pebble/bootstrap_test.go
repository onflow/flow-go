package pebble

import (
	"encoding/binary"
	"io"
	"os"
	"path"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestBootstrap_NewBootstrap(t *testing.T) {
	sampleDir := path.Join(unittest.TempDir(t), "checkpoint.checkpoint")
	rootHeight := uint64(1)
	log := zerolog.New(io.Discard)
	cache := pebble.NewCache(1 << 20)
	defer cache.Unref()
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	p, dir := unittest.TempPebbleDBWithOpts(t, opts)
	// set heights
	require.NoError(t, p.Set(firstHeightKey(), encodedUint64(rootHeight), nil))
	require.NoError(t, p.Set(latestHeightKey(), encodedUint64(rootHeight), nil))
	// errors if FirstHeight or LastHeight are populated
	_, err := NewBootstrap(p, sampleDir, rootHeight, log)
	require.ErrorContains(t, err, "cannot bootstrap populated DB")
	require.NoError(t, os.RemoveAll(dir))
}

func TestBootstrap_IndexCheckpointFile_Happy(t *testing.T) {
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
		bootstrap, err := NewBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile()
		require.NoError(t, err)

		// create registers instance and check values
		reg, err := NewRegisters(pb)
		require.NoError(t, err)

		require.Equal(t, reg.LatestHeight(), rootHeight)
		require.Equal(t, reg.FirstHeight(), rootHeight)

		for _, register := range registerIDs {
			val, err := reg.Get(*register, rootHeight)
			require.NoError(t, err)
			require.Equal(t, val, []byte{uint8(0)})
		}

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

func TestBootstrap_IndexCheckpointFile_Empty(t *testing.T) {
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
		bootstrap, err := NewBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile()
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

func TestBootstrap_IndexCheckpointFile_FormatIssue(t *testing.T) {
	t.Parallel()

}

func TestBootstrap_IndexCheckpointFile_Error(t *testing.T) {
	t.Parallel()

}

func simpleTrieWithValidRegisterIDs(t *testing.T) ([]*trie.MTrie, []*flow.RegisterID) {
	p1 := testutils.PathByUint8(0)
	p2 := testutils.PathByUint8(1)
	paths := []ledger.Path{p1, p2}
	payloads := RandomRegisterPayloads(2)
	// collect register IDs to return
	rID := make([]*flow.RegisterID, 0, 2)
	for _, payload := range payloads {
		key, err := payload.Key()
		require.NoError(t, err)
		regID, err := keyToRegisterID(key)
		require.NoError(t, err)
		rID = append(rID, &regID)
	}
	emptyTrie := trie.NewEmptyMTrie()
	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	tries := []*trie.MTrie{emptyTrie, updatedTrie}
	return tries, rID
}

func RandomRegisterPayloads(n uint64) []ledger.Payload {
	p := make([]ledger.Payload, 0, n)
	for i := uint64(0); i < n; i++ {
		o := make([]byte, 0, 8)
		o = binary.BigEndian.AppendUint64(o, n)
		k := ledger.Key{KeyParts: []ledger.KeyPart{
			{Type: state.KeyPartOwner, Value: o},
			{Type: state.KeyPartKey, Value: o},
		}}
		v := ledger.Value{uint8(0)}
		pl := ledger.NewPayload(k, v)
		p = append(p, *pl)
	}
	return p
}
