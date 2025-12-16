package storehouse

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func TestValidateWithCheckpoint_AllMatching(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	workerCount := 2
	registerCount := 10

	unittest.RunWithTempDir(t, func(dir string) {
		// create generator suite for random register entries
		suite := fixtures.NewGeneratorSuite()

		// generate random register entries using unittest fixtures
		registerEntries := suite.RegisterEntries().List(registerCount)

		// create checkpoint from register entries
		tries, rootHash := createTrieFromRegisterEntries(t, registerEntries)
		fileName := "root.checkpoint"
		require.NoError(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log))

		// create pebble store and populate with matching registers
		dbDir := unittest.TempPebblePath(t)
		defer func() {
			require.NoError(t, os.RemoveAll(dbDir))
		}()

		// bootstrap DB at rootHeight
		db := pebble.NewBootstrappedRegistersWithPathForTest(t, dbDir, rootHeight, rootHeight)
		defer func() {
			require.NoError(t, db.Close())
		}()

		// create Registers instance
		pb, err := pebble.NewRegisters(db, pebble.PruningDisabled)
		require.NoError(t, err)

		// store registers at rootHeight + 1
		storeHeight := rootHeight + 1
		require.NoError(t, pb.Store(registerEntries, storeHeight))

		// verify registers are stored at rootHeight + 1
		require.Equal(t, storeHeight, pb.LatestHeight())
		for _, entry := range registerEntries {
			value, err := pb.Get(entry.Key, storeHeight)
			require.NoError(t, err)
			require.Equal(t, entry.Value, value)
		}

		// create mocks for validation at storeHeight
		headers, results := createMocks(t, storeHeight, rootHash)

		// validate at storeHeight - should return no error
		err = ValidateWithCheckpoint(log, context.Background(), pb, results, headers, dir, storeHeight, workerCount)
		require.NoError(t, err)
	})
}

func TestValidateWithCheckpoint_WithMismatches(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	workerCount := 2

	unittest.RunWithTempDir(t, func(dir string) {
		// create generator suite for random register entries
		suite := fixtures.NewGeneratorSuite()

		// generate random register entries using unittest fixtures
		registerEntries := suite.RegisterEntries().List(5)

		// create checkpoint from register entries
		tries, rootHash := createTrieFromRegisterEntries(t, registerEntries)
		fileName := "root.checkpoint"
		require.NoError(t, wal.StoreCheckpointV6Concurrently(tries, dir, fileName, log))

		// create pebble store and populate with mismatched registers
		dbDir := unittest.TempPebblePath(t)
		defer func() {
			require.NoError(t, os.RemoveAll(dbDir))
		}()

		// bootstrap DB at rootHeight
		db := pebble.NewBootstrappedRegistersWithPathForTest(t, dbDir, rootHeight, rootHeight)
		defer func() {
			require.NoError(t, db.Close())
		}()

		// create Registers instance
		pb, err := pebble.NewRegisters(db, pebble.PruningDisabled)
		require.NoError(t, err)

		// store registers at rootHeight + 1 with wrong values
		storeHeight := rootHeight + 1
		mismatchedEntries := make(flow.RegisterEntries, 0, len(registerEntries))
		for _, entry := range registerEntries {
			mismatchedEntries = append(mismatchedEntries, flow.RegisterEntry{
				Key:   entry.Key,
				Value: []byte{'x'}, // different value from checkpoint
			})
		}
		require.NoError(t, pb.Store(mismatchedEntries, storeHeight))

		// create mocks for validation at storeHeight
		headers, results := createMocks(t, storeHeight, rootHash)

		// validate at storeHeight - should return error with mismatch count
		err = ValidateWithCheckpoint(log, context.Background(), pb, results, headers, dir, storeHeight, workerCount)
		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed: found")
		require.Contains(t, err.Error(), "register value mismatches")
	})
}

// createTrieFromRegisterEntries creates a trie from register entries for checkpoint creation
func createTrieFromRegisterEntries(t *testing.T, entries flow.RegisterEntries) ([]*trie.MTrie, ledger.RootHash) {
	// convert register entries to payloads
	payloads := make([]*ledger.Payload, 0, len(entries))
	for _, entry := range entries {
		key := convert.RegisterIDToLedgerKey(entry.Key)
		payload := ledger.NewPayload(key, ledger.Value(entry.Value))
		payloads = append(payloads, payload)
	}

	// get paths from payloads
	paths, err := pathfinder.PathsFromPayloads(payloads, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	// create trie
	emptyTrie := trie.NewEmptyMTrie()
	derefPayloads := make([]ledger.Payload, len(payloads))
	for i, p := range payloads {
		derefPayloads[i] = *p
	}

	populatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, derefPayloads, true)
	require.NoError(t, err)
	return []*trie.MTrie{populatedTrie}, populatedTrie.RootHash()
}


func createMocks(t *testing.T, height uint64, rootHash ledger.RootHash) (storage.Headers, storage.ExecutionResults) {
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.BlockID = header.ID()
		result.Chunks = flow.ChunkList{
			{
				EndState: flow.StateCommitment(rootHash),
			},
		}
	})

	mockHeaders := storagemock.NewHeaders(t)
	mockHeaders.On("BlockIDByHeight", height).Return(header.ID(), nil)

	mockResults := storagemock.NewExecutionResults(t)
	mockResults.On("ByBlockID", header.ID()).Return(result, nil)

	return mockHeaders, mockResults
}

