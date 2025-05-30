package indexer

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewInMemoryIndexer(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	block := unittest.BlockWithParentFixture(header)
	exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
	indexer := createInMemoryIndexer(exeResult, header)

	assert.NotNil(t, indexer)
	assert.Equal(t, header.Height, indexer.registers.LatestHeight())
}

func TestInMemoryIndexer_IndexBlockData(t *testing.T) {
	t.Run("Index Single Chunk and Single Register", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()
		block := unittest.BlockWithParentFixture(header)
		blockID := block.ID()
		exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		indexer := createInMemoryIndexer(exeResult, header)

		trie := createTestTrieUpdate(t)
		require.NotEmpty(t, trie.Payloads)
		collection := unittest.CollectionFixture(0)

		ed := &execution_data.BlockExecutionData{
			BlockID: blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{
					Collection: &collection,
					TrieUpdate: trie,
				},
			},
		}

		execData := execution_data.NewBlockExecutionDataEntity(blockID, ed)

		err := indexer.IndexBlockData(execData)
		assert.NoError(t, err)

		// Verify registers were indexed
		for _, payload := range trie.Payloads {
			k, err := payload.Key()
			require.NoError(t, err)

			id, err := convert.LedgerKeyToRegisterID(k)
			require.NoError(t, err)

			value, err := indexer.registers.Get(id, header.Height)
			require.NoError(t, err)

			// Compare byte slices directly instead of comparing types
			assert.ElementsMatch(t, []byte(payload.Value()), value, "Register values should match")
		}
	})

	t.Run("Index Multiple Chunks and Merge Same Register Updates", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()
		block := unittest.BlockWithParentFixture(header)
		blockID := block.ID()
		exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		indexer := createInMemoryIndexer(exeResult, header)

		tries := []*ledger.TrieUpdate{createTestTrieUpdate(t), createTestTrieUpdate(t)}
		// Make sure we have two register updates that are updating the same value
		tries[1].Paths[0] = tries[0].Paths[0]
		testValue := tries[1].Payloads[0]
		key, err := testValue.Key()
		require.NoError(t, err)
		testRegisterID, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)

		collection := unittest.CollectionFixture(0)

		ed := &execution_data.BlockExecutionData{
			BlockID: blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{
					Collection: &collection,
					TrieUpdate: tries[0],
				},
				{
					Collection: &collection,
					TrieUpdate: tries[1],
				},
			},
		}

		execData := execution_data.NewBlockExecutionDataEntity(blockID, ed)

		err = indexer.IndexBlockData(execData)
		assert.NoError(t, err)

		// Verify register was indexed
		value, err := indexer.registers.Get(testRegisterID, header.Height)
		require.NoError(t, err)
		// Compare byte slices directly
		assert.ElementsMatch(t, []byte(testValue.Value()), value, "Register values should match")
	})

	t.Run("Index Events", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()
		block := unittest.BlockWithParentFixture(header)
		blockID := block.ID()
		exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		indexer := createInMemoryIndexer(exeResult, header)

		expectedEvents := unittest.EventsFixture(20)
		collection := unittest.CollectionFixture(0)

		ed := &execution_data.BlockExecutionData{
			BlockID: blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{
					Collection: &collection,
					Events:     expectedEvents[:10],
				},
				{
					Collection: &collection,
					Events:     expectedEvents[10:],
				},
			},
		}

		execData := execution_data.NewBlockExecutionDataEntity(blockID, ed)

		err := indexer.IndexBlockData(execData)
		assert.NoError(t, err)

		// Verify events were indexed correctly
		events, err := indexer.events.ByBlockID(blockID)
		require.NoError(t, err)
		assert.ElementsMatch(t, expectedEvents, events)
	})

	t.Run("Index Tx Results", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()
		block := unittest.BlockWithParentFixture(header)
		blockID := block.ID()
		exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		indexer := createInMemoryIndexer(exeResult, header)

		expectedResults := unittest.LightTransactionResultsFixture(20)
		collection := unittest.CollectionFixture(0)

		ed := &execution_data.BlockExecutionData{
			BlockID: blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				// Split results into 2 chunks
				{
					Collection:         &collection,
					TransactionResults: expectedResults[:10],
				},
				{
					Collection:         &collection,
					TransactionResults: expectedResults[10:],
				},
			},
		}

		execData := execution_data.NewBlockExecutionDataEntity(blockID, ed)

		err := indexer.IndexBlockData(execData)
		assert.NoError(t, err)

		// Verify results were indexed correctly
		results, err := indexer.results.ByBlockID(blockID)
		require.NoError(t, err)
		assert.ElementsMatch(t, expectedResults, results)
	})

	t.Run("Index Collections", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()
		block := unittest.BlockWithParentFixture(header)
		blockID := block.ID()
		exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		indexer := createInMemoryIndexer(exeResult, header)

		// Create collections and store them directly first
		expectedCollections := unittest.CollectionListFixture(2)
		systemChunkCollection := unittest.CollectionFixture(1)

		ed := &execution_data.BlockExecutionData{
			BlockID: blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{Collection: expectedCollections[0]},
				{Collection: expectedCollections[1]},
				{Collection: &systemChunkCollection},
			},
		}

		execData := execution_data.NewBlockExecutionDataEntity(blockID, ed)

		err := indexer.IndexBlockData(execData)
		assert.NoError(t, err)

		// Verify collections can be retrieved
		for _, expectedCollection := range expectedCollections {
			coll, err := indexer.collections.ByID(expectedCollection.ID())
			require.NoError(t, err)
			assert.Equal(t, expectedCollection.Transactions, coll.Transactions)

			lightColl, err := indexer.collections.LightByID(expectedCollection.ID())
			require.NoError(t, err)
			assert.Equal(t, expectedCollection.Light().Transactions, lightColl.Transactions)

			// Verify transactions were indexed
			for _, tx := range expectedCollection.Transactions {
				storedTx, err := indexer.transactions.ByID(tx.ID())
				require.NoError(t, err)
				assert.Equal(t, tx.ID(), storedTx.ID())
			}
		}
	})

	t.Run("Index AllTheThings", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()
		block := unittest.BlockWithParentFixture(header)
		blockID := block.ID()
		exeResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		indexer := createInMemoryIndexer(exeResult, header)

		expectedEvents := unittest.EventsFixture(20)
		expectedResults := unittest.LightTransactionResultsFixture(20)
		expectedCollections := unittest.CollectionListFixture(2)
		systemChunkCollection := unittest.CollectionFixture(1)
		expectedTries := []*ledger.TrieUpdate{createTestTrieUpdate(t), createTestTrieUpdate(t)}

		ed := &execution_data.BlockExecutionData{
			BlockID: blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{
					Collection:         expectedCollections[0],
					Events:             expectedEvents[:10],
					TransactionResults: expectedResults[:10],
					TrieUpdate:         expectedTries[0],
				},
				{
					Collection:         expectedCollections[1],
					TransactionResults: expectedResults[10:],
					Events:             expectedEvents[10:],
					TrieUpdate:         expectedTries[1],
				},
				{
					Collection: &systemChunkCollection,
				},
			},
		}

		execData := execution_data.NewBlockExecutionDataEntity(blockID, ed)

		err := indexer.IndexBlockData(execData)
		assert.NoError(t, err)

		// Verify all events were indexed
		events, err := indexer.events.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Len(t, events, len(expectedEvents))

		// Verify all results were indexed
		results, err := indexer.results.ByBlockID(blockID)
		require.NoError(t, err)
		assert.Len(t, results, len(expectedResults))

		// Verify collections were indexed
		for _, expectedCollection := range expectedCollections {
			lightColl, err := indexer.collections.LightByID(expectedCollection.ID())
			require.NoError(t, err)
			assert.Equal(t, expectedCollection.Light().Transactions, lightColl.Transactions)
		}

		// Verify registers were indexed
		// Collect all payloads across all tries
		payloads := make(map[flow.RegisterID][]byte)
		for _, trie := range expectedTries {
			for _, payload := range trie.Payloads {
				k, err := payload.Key()
				require.NoError(t, err)
				id, err := convert.LedgerKeyToRegisterID(k)
				require.NoError(t, err)
				payloads[id] = []byte(payload.Value())
			}
		}

		// Check each register has the correct value
		for id, expectedValue := range payloads {
			value, err := indexer.registers.Get(id, header.Height)
			require.NoError(t, err)
			assert.ElementsMatch(t, expectedValue, value, "Register values should match")
		}
	})
}

// Helper functions

func createInMemoryIndexer(executionResult *flow.ExecutionResult, header *flow.Header) *InMemoryIndexer {
	return NewInMemoryIndexer(zerolog.Nop(),
		metrics.NewNoopCollector(),
		unsynchronized.NewRegisters(header.Height),
		unsynchronized.NewEvents(),
		unsynchronized.NewCollections(),
		unsynchronized.NewTransactions(),
		unsynchronized.NewLightTransactionResults(),
		executionResult,
		header)
}

func createTestTrieWithPayloads(payloads []*ledger.Payload) *ledger.TrieUpdate {
	keys := make([]ledger.Key, 0)
	values := make([]ledger.Value, 0)
	for _, payload := range payloads {
		key, _ := payload.Key()
		keys = append(keys, key)
		values = append(values, payload.Value())
	}

	update, _ := ledger.NewUpdate(ledger.DummyState, keys, values)
	trie, _ := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
	return trie
}

func createTestTrieUpdate(t *testing.T) *ledger.TrieUpdate {
	return createTestTrieWithPayloads(
		[]*ledger.Payload{
			createTestPayload(t),
			createTestPayload(t),
			createTestPayload(t),
			createTestPayload(t),
		})
}

func createTestPayload(t *testing.T) *ledger.Payload {
	owner := unittest.RandomAddressFixture()
	key := make([]byte, 8)
	_, err := rand.Read(key)
	require.NoError(t, err)
	val := make([]byte, 8)
	_, err = rand.Read(val)
	require.NoError(t, err)
	return createTestLedgerPayload(owner.String(), fmt.Sprintf("%x", key), val)
}

func createTestLedgerPayload(owner string, key string, value []byte) *ledger.Payload {
	k := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(owner),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte(key),
			},
		},
	}

	return ledger.NewPayload(k, value)
}
