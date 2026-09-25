package pebble

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
)

// TestComputeRegisterRootHash checks that the root hash computed from a register store equals
// the root hash of the trie holding the same registers, and that the computation detects
// registers that are modified, added or removed.
func TestComputeRegisterRootHash(t *testing.T) {
	t.Parallel()
	rootHeight := uint64(10000)

	t.Run("empty store", func(t *testing.T) {
		t.Parallel()
		store := storeWithRegisters(t, nil, rootHeight)
		defer store.close(t)

		require.Equal(t,
			ledger.RootHash(ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight)),
			computeRootHash(t, store, rootHeight, 4))
	})

	t.Run("single register", func(t *testing.T) {
		t.Parallel()
		registers := []rootHashTestRegister{
			{owner: addressBytes(t, 1), key: "key", value: []byte("value")},
		}
		checkpointTrie, _ := rootHashTestTrie(t, registers)
		store := storeWithRegisters(t, registers, rootHeight)
		defer store.close(t)

		require.Equal(t, checkpointTrie.RootHash(), computeRootHash(t, store, rootHeight, 1))
	})

	t.Run("registers sharing a sub tree", func(t *testing.T) {
		t.Parallel()
		// registers of one owner are spread over the trie by the hash of their key, but a
		// handful of registers still exercises the folding of single-child nodes
		registers := manyRootHashTestRegisters(1, 10)
		checkpointTrie, _ := rootHashTestTrie(t, registers)
		store := storeWithRegisters(t, registers, rootHeight)
		defer store.close(t)

		require.Equal(t, checkpointTrie.RootHash(), computeRootHash(t, store, rootHeight, 4))
	})

	t.Run("store bootstrapped from a checkpoint", func(t *testing.T) {
		t.Parallel()
		checkpointTrie, _ := rootHashTestTrie(t, manyRootHashTestRegisters(8, 100))
		store := bootstrapStoreFromTrie(t, checkpointTrie, rootHeight)
		defer store.close(t)

		// the root hash does not depend on how many workers fold the buckets
		require.Equal(t, checkpointTrie.RootHash(), computeRootHash(t, store, rootHeight, 1))
		require.Equal(t, checkpointTrie.RootHash(), computeRootHash(t, store, rootHeight, 4))
	})

	t.Run("modified, added and removed registers change the root hash", func(t *testing.T) {
		t.Parallel()
		checkpointTrie, registerIDs := rootHashTestTrie(t, manyRootHashTestRegisters(4, 50))
		store := bootstrapStoreFromTrie(t, checkpointTrie, rootHeight)
		defer store.close(t)

		expected := checkpointTrie.RootHash()
		require.Equal(t, expected, computeRootHash(t, store, rootHeight, 4))

		// a register with a different value
		require.NoError(t, store.db.Set(
			newLookupKey(rootHeight, registerIDs[0]).Bytes(), []byte("modified value"), nil))
		require.NotEqual(t, expected, computeRootHash(t, store, rootHeight, 4))

		// an extra register that is not part of the checkpoint
		extra := flow.RegisterID{Owner: string(addressBytes(t, 99)), Key: "extra key"}
		require.NoError(t, store.db.Set(
			newLookupKey(rootHeight, extra).Bytes(), []byte("extra value"), nil))
		require.NotEqual(t, expected, computeRootHash(t, store, rootHeight, 4))

		// a removed register (empty value)
		require.NoError(t, store.db.Set(
			newLookupKey(rootHeight, registerIDs[1]).Bytes(), nil, nil))
		require.NotEqual(t, expected, computeRootHash(t, store, rootHeight, 4))
	})
}

// TestComputeRegisterRootHash_InvalidWorkerCount checks that a worker count below one is
// rejected.
func TestComputeRegisterRootHash_InvalidWorkerCount(t *testing.T) {
	t.Parallel()
	store := storeWithRegisters(t, nil, uint64(10))
	defer store.close(t)

	_, err := ComputeRegisterRootHash(
		context.Background(), zerolog.New(io.Discard), store.registers, uint64(10), store.dir,
		completeLedger.DefaultPathFinderVersion, 0)
	require.ErrorContains(t, err, "worker count must be at least 1")
}

// rootHashTestStore is a register store open for the duration of a test.
type rootHashTestStore struct {
	db        *pebble.DB
	registers *Registers
	dir       string
}

func (s *rootHashTestStore) close(t *testing.T) {
	require.NoError(t, s.db.Close())
	require.NoError(t, os.RemoveAll(s.dir))
}

// bootstrapStoreFromTrie stores a checkpoint of the given trie and bootstraps a register store
// from it.
func bootstrapStoreFromTrie(t *testing.T, checkpointTrie *trie.MTrie, rootHeight uint64) *rootHashTestStore {
	log := zerolog.New(io.Discard)
	checkpointDir := t.TempDir()
	fileName := "checkpoint"
	require.NoError(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, checkpointDir, fileName, log))

	pb, dbDir := createPebbleForTest(t)
	bootstrap, err := NewRegisterBootstrapSSTables(
		pb, dbDir, filepath.Join(checkpointDir, fileName), rootHeight, checkpointTrie.RootHash(), log)
	require.NoError(t, err)
	require.NoError(t, bootstrap.IndexCheckpointFile(context.Background(), 4))

	registersStore, err := NewRegisters(pb, PruningDisabled)
	require.NoError(t, err)

	return &rootHashTestStore{db: pb, registers: registersStore, dir: dbDir}
}

// storeWithRegisters returns a register store holding the given registers at the given height,
// written directly to the store. It is used for registers whose trie is too shallow to be read
// back from a checkpoint: the checkpoint leaf node reader only reads the trie nodes below the
// subtrie level, so the leaves of a trie with a handful of registers are not in the checkpoint.
func storeWithRegisters(t *testing.T, registers []rootHashTestRegister, height uint64) *rootHashTestStore {
	pb, dbDir := createPebbleForTest(t)
	require.NoError(t, initHeights(pb, height))

	for _, register := range registers {
		registerID, err := convert.LedgerKeyToRegisterID(rootHashTestLedgerKey(register))
		require.NoError(t, err)
		require.NoError(t, pb.Set(newLookupKey(height, registerID).Bytes(), register.value, nil))
	}

	registersStore, err := NewRegisters(pb, PruningDisabled)
	require.NoError(t, err)
	return &rootHashTestStore{db: pb, registers: registersStore, dir: dbDir}
}

// computeRootHash computes the root hash of the given register store with the given worker count.
func computeRootHash(t *testing.T, store *rootHashTestStore, rootHeight uint64, workerCount int) ledger.RootHash {
	rootHash, err := ComputeRegisterRootHash(
		context.Background(), zerolog.New(io.Discard), store.registers, rootHeight, store.dir,
		completeLedger.DefaultPathFinderVersion, workerCount)
	require.NoError(t, err)
	return rootHash
}

// rootHashTestRegister is a register of a test trie. The owner must be either empty (a global
// register) or exactly flow.AddressLength bytes, like the register owners of a real state.
type rootHashTestRegister struct {
	owner []byte
	key   string
	value []byte
}

// addressBytes returns the bytes of a test account address.
func addressBytes(t *testing.T, address uint64) []byte {
	ownerBytes := make([]byte, flow.AddressLength)
	binary.BigEndian.PutUint64(ownerBytes, address)
	require.NotEqual(t, flow.EmptyAddress[:], ownerBytes)
	return ownerBytes
}

// manyRootHashTestRegisters returns registers of the given number of owners, with the given
// number of registers per owner, and one global register without an owner.
func manyRootHashTestRegisters(owners int, registersPerOwner int) []rootHashTestRegister {
	registers := make([]rootHashTestRegister, 0, owners*registersPerOwner+1)
	for owner := range owners {
		ownerBytes := make([]byte, flow.AddressLength)
		binary.BigEndian.PutUint64(ownerBytes, uint64(owner)+1)
		for register := range registersPerOwner {
			registers = append(registers, rootHashTestRegister{
				owner: ownerBytes,
				key:   fmt.Sprintf("storage/reg%d", register),
				value: []byte(fmt.Sprintf("value-%d-%d", owner, register)),
			})
		}
	}
	// a global register, without an owner
	return append(registers, rootHashTestRegister{owner: nil, key: "uuid", value: []byte("global value")})
}

// rootHashTestTrie builds a trie holding the given registers and returns it together with the
// register IDs of its registers.
func rootHashTestTrie(t *testing.T, registers []rootHashTestRegister) (*trie.MTrie, []flow.RegisterID) {
	keys := make([]ledger.Key, 0, len(registers))
	payloads := make([]ledger.Payload, 0, len(registers))
	registerIDs := make([]flow.RegisterID, 0, len(registers))
	for _, register := range registers {
		key := rootHashTestLedgerKey(register)
		keys = append(keys, key)
		payloads = append(payloads, *ledger.NewPayload(key, ledger.Value(register.value)))

		registerID, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)
		registerIDs = append(registerIDs, registerID)
	}

	paths, err := pathfinder.KeysToPaths(keys, completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	checkpointTrie, _, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), paths, payloads, false)
	require.NoError(t, err)
	require.Len(t, checkpointTrie.AllPayloads(), len(registers))

	return checkpointTrie, registerIDs
}

// rootHashTestLedgerKey returns the ledger key of the given test register.
func rootHashTestLedgerKey(register rootHashTestRegister) ledger.Key {
	return ledger.Key{KeyParts: []ledger.KeyPart{
		{Type: ledger.KeyPartOwner, Value: register.owner},
		{Type: ledger.KeyPartKey, Value: []byte(register.key)},
	}}
}
