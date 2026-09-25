package pebble

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
)

// TestRegisterDigest checks the properties of the register digest: it does not depend on the order
// registers are mixed in, it detects modified, added and removed registers, and it distinguishes
// sets of registers that only differ in their size.
func TestRegisterDigest(t *testing.T) {
	t.Parallel()

	key := func(owner string, key string) ledger.Key {
		return ledger.Key{KeyParts: []ledger.KeyPart{
			{Type: ledger.KeyPartOwner, Value: []byte(owner)},
			{Type: ledger.KeyPartKey, Value: []byte(key)},
		}}
	}

	digestOf := func(registers []ledger.Key, value []byte) RegisterDigest {
		digest := RegisterDigest{}
		for _, registerKey := range registers {
			digest.add(registerKey, value, nil)
		}
		return digest
	}

	a := key("owner-a", "key")
	b := key("owner-b", "key")
	c := key("owner-c", "key")
	value := []byte("value")

	// the digest does not depend on the order the registers are mixed in
	expected := digestOf([]ledger.Key{a, b, c}, value)
	require.True(t, expected.Equal(digestOf([]ledger.Key{c, a, b}, value)))
	require.Equal(t, uint64(3), expected.RegisterCount)
	require.NotEqual(t, hash.DummyHash, expected.Digest)

	// merging partial digests gives the same digest, in any order
	first := digestOf([]ledger.Key{a}, value)
	second := digestOf([]ledger.Key{b, c}, value)
	merged := first
	merged.Merge(second)
	require.True(t, expected.Equal(merged))
	merged = second
	merged.Merge(first)
	require.True(t, expected.Equal(merged))

	// a different value, an added register and a removed register are all detected
	require.False(t, expected.Equal(digestOf([]ledger.Key{a, b, c}, []byte("other value"))))
	require.False(t, expected.Equal(digestOf([]ledger.Key{a, b, c, a}, value)))
	require.False(t, expected.Equal(digestOf([]ledger.Key{a, b}, value)))

	// a register mixed in an odd and an even number of times has the same digest, but a different
	// register count; that is why both fields are compared
	onceMore := digestOf([]ledger.Key{a, b, c, a, a}, value)
	require.Equal(t, expected.Digest, onceMore.Digest)
	require.False(t, expected.Equal(onceMore))
}

// TestComputeStoreRegisterDigest checks that the digest computed from a register store equals the
// digest of the checkpoint the store was bootstrapped from, and that it detects modified, added and
// removed registers.
func TestComputeStoreRegisterDigest(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)

	t.Run("empty store", func(t *testing.T) {
		t.Parallel()
		store := storeWithRegisters(t, nil, rootHeight)
		defer store.close(t)

		digest, err := ComputeStoreRegisterDigest(context.Background(), log, store.registers, rootHeight, 4)
		require.NoError(t, err)
		require.Equal(t, RegisterDigest{}, digest)
	})

	t.Run("store bootstrapped from a checkpoint", func(t *testing.T) {
		t.Parallel()
		checkpointTrie, _ := rootHashTestTrie(t, manyRootHashTestRegisters(8, 100))
		store := bootstrapStoreFromTrie(t, checkpointTrie, rootHeight)
		defer store.close(t)

		// the digest does not depend on the number of workers
		storeRegistersDigest := computeStoreDigest(t, store, rootHeight, 1)
		require.Equal(t, storeRegistersDigest, computeStoreDigest(t, store, rootHeight, 4))

		// the store digest equals the digest of the checkpoint it was bootstrapped from
		checkpointRegistersDigest := computeCheckpointDigest(t, checkpointTrie, rootHeight, 4)
		require.True(t, storeRegistersDigest.Equal(checkpointRegistersDigest),
			"store %s, checkpoint %s", storeRegistersDigest, checkpointRegistersDigest)
		require.Equal(t, uint64(801), storeRegistersDigest.RegisterCount)
	})

	t.Run("modified, added and removed registers change the digest", func(t *testing.T) {
		t.Parallel()
		checkpointTrie, registerIDs := rootHashTestTrie(t, manyRootHashTestRegisters(4, 50))
		store := bootstrapStoreFromTrie(t, checkpointTrie, rootHeight)
		defer store.close(t)

		expected := computeCheckpointDigest(t, checkpointTrie, rootHeight, 4)
		require.True(t, computeStoreDigest(t, store, rootHeight, 4).Equal(expected))

		// a register with a different value
		require.NoError(t, store.db.Set(
			newLookupKey(rootHeight, registerIDs[0]).Bytes(), []byte("modified value"), nil))
		require.False(t, computeStoreDigest(t, store, rootHeight, 4).Equal(expected))

		// an extra register that is not part of the checkpoint
		extra := flow.RegisterID{Owner: string(addressBytes(t, 99)), Key: "extra key"}
		require.NoError(t, store.db.Set(
			newLookupKey(rootHeight, extra).Bytes(), []byte("extra value"), nil))
		require.False(t, computeStoreDigest(t, store, rootHeight, 4).Equal(expected))

		// a removed register (empty value)
		require.NoError(t, store.db.Set(
			newLookupKey(rootHeight, registerIDs[1]).Bytes(), nil, nil))
		require.False(t, computeStoreDigest(t, store, rootHeight, 4).Equal(expected))
	})
}

// TestComputeStoreRegisterDigest_InvalidWorkerCount checks that a worker count below one is
// rejected.
func TestComputeStoreRegisterDigest_InvalidWorkerCount(t *testing.T) {
	t.Parallel()
	store := storeWithRegisters(t, nil, uint64(10))
	defer store.close(t)

	_, err := ComputeStoreRegisterDigest(context.Background(), zerolog.New(io.Discard), store.registers, uint64(10), 0)
	require.ErrorContains(t, err, "worker count must be at least 1")

	_, err = ComputeCheckpointRegisterDigest(
		context.Background(), zerolog.New(io.Discard), t.TempDir(), "checkpoint", ledger.RootHash{}, 0)
	require.ErrorContains(t, err, "worker count must be at least 1")
}

// computeStoreDigest computes the digest of the given register store.
func computeStoreDigest(t *testing.T, store *rootHashTestStore, rootHeight uint64, workerCount int) RegisterDigest {
	digest, err := ComputeStoreRegisterDigest(
		context.Background(), zerolog.New(io.Discard), store.registers, rootHeight, workerCount)
	require.NoError(t, err)
	return digest
}

// computeCheckpointDigest stores a checkpoint of the given trie and computes its digest.
func computeCheckpointDigest(t *testing.T, checkpointTrie *trie.MTrie, rootHeight uint64, workerCount int) RegisterDigest {
	checkpointDir := t.TempDir()
	fileName := "checkpoint"
	log := zerolog.New(io.Discard)
	require.NoError(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, checkpointDir, fileName, log))

	digest, err := ComputeCheckpointRegisterDigest(
		context.Background(), log, checkpointDir, fileName, checkpointTrie.RootHash(), workerCount)
	require.NoError(t, err)
	return digest
}
