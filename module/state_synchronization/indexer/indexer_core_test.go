package indexer

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

type indexCoreTest struct {
	t                *testing.T
	indexer          *IndexerCore
	registers        *storagemock.RegisterIndex
	events           *storagemock.Events
	collection       *flow.Collection
	collections      *storagemock.Collections
	transactions     *storagemock.Transactions
	results          *storagemock.LightTransactionResults
	headers          *storagemock.Headers
	ctx              context.Context
	blocks           []*flow.Block
	data             *execution_data.BlockExecutionDataEntity
	lastHeightStore  func(t *testing.T) uint64
	firstHeightStore func(t *testing.T) uint64
	registersStore   func(t *testing.T, entries flow.RegisterEntries, height uint64) error
	eventsStore      func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error
	registersGet     func(t *testing.T, IDs flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

func newIndexCoreTest(
	t *testing.T,
	blocks []*flow.Block,
	exeData *execution_data.BlockExecutionDataEntity,
) *indexCoreTest {

	collection := unittest.CollectionFixture(0)

	return &indexCoreTest{
		t:            t,
		registers:    storagemock.NewRegisterIndex(t),
		events:       storagemock.NewEvents(t),
		collection:   &collection,
		results:      storagemock.NewLightTransactionResults(t),
		collections:  storagemock.NewCollections(t),
		transactions: storagemock.NewTransactions(t),
		blocks:       blocks,
		ctx:          context.Background(),
		data:         exeData,
		headers:      newBlockHeadersStorage(blocks).(*storagemock.Headers), // convert it back to mock type for tests,
	}
}

func (i *indexCoreTest) useDefaultBlockByHeight() *indexCoreTest {
	i.headers.
		On("BlockIDByHeight", mocks.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			for _, b := range i.blocks {
				if b.Height == height {
					return b.ID(), nil
				}
			}
			return flow.ZeroID, fmt.Errorf("not found")
		})

	i.headers.
		On("ByHeight", mocks.AnythingOfType("uint64")).
		Return(func(height uint64) (*flow.Header, error) {
			for _, b := range i.blocks {
				if b.Height == height {
					return b.ToHeader(), nil
				}
			}
			return nil, fmt.Errorf("not found")
		})

	return i
}

func (i *indexCoreTest) setLastHeight(f func(t *testing.T) uint64) *indexCoreTest {
	i.registers.
		On("LatestHeight").
		Return(func() uint64 {
			return f(i.t)
		})
	return i
}

func (i *indexCoreTest) useDefaultHeights() *indexCoreTest {
	i.registers.
		On("FirstHeight").
		Return(func() uint64 {
			return i.blocks[0].Height
		})
	i.registers.
		On("LatestHeight").
		Return(func() uint64 {
			return i.blocks[len(i.blocks)-1].Height
		})
	return i
}

func (i *indexCoreTest) setStoreRegisters(f func(t *testing.T, entries flow.RegisterEntries, height uint64) error) *indexCoreTest {
	i.registers.
		On("Store", mock.AnythingOfType("flow.RegisterEntries"), mock.AnythingOfType("uint64")).
		Return(func(entries flow.RegisterEntries, height uint64) error {
			return f(i.t, entries, height)
		}).Once()
	return i
}

func (i *indexCoreTest) setStoreEvents(f func(*testing.T, flow.Identifier, []flow.EventsList) error) *indexCoreTest {
	i.events.
		On("BatchStore", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList"), mock.Anything).
		Return(func(blockID flow.Identifier, events []flow.EventsList, batch storage.ReaderBatchWriter) error {
			require.NotNil(i.t, batch)
			return f(i.t, blockID, events)
		})
	return i
}

func (i *indexCoreTest) setStoreTransactionResults(f func(*testing.T, flow.Identifier, []flow.LightTransactionResult) error) *indexCoreTest {
	i.results.
		On("BatchStore", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.LightTransactionResult"), mock.Anything).
		Return(func(blockID flow.Identifier, results []flow.LightTransactionResult, batch storage.ReaderBatchWriter) error {
			require.NotNil(i.t, batch)
			return f(i.t, blockID, results)
		})
	return i
}

func (i *indexCoreTest) setGetRegisters(f func(t *testing.T, ID flow.RegisterID, height uint64) (flow.RegisterValue, error)) *indexCoreTest {
	i.registers.
		On("Get", mock.AnythingOfType("flow.RegisterID"), mock.AnythingOfType("uint64")).
		Return(func(IDs flow.RegisterID, height uint64) (flow.RegisterValue, error) {
			return f(i.t, IDs, height)
		})
	return i
}

func (i *indexCoreTest) useDefaultStorageMocks() *indexCoreTest {

	i.collections.On("StoreAndIndexByTransaction", mock.Anything, mock.AnythingOfType("*flow.Collection")).Return(&flow.LightCollection{}, nil).Maybe()
	i.transactions.On("Store", mock.AnythingOfType("*flow.TransactionBody")).Return(nil).Maybe()

	return i
}

func (i *indexCoreTest) useDefaultEvents() *indexCoreTest {
	i.events.
		On("BatchStore", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList"), mock.Anything).
		Return(nil)
	return i
}

func (i *indexCoreTest) useDefaultTransactionResults() *indexCoreTest {
	i.results.
		On("BatchStore", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.LightTransactionResult"), mock.Anything).
		Return(nil)
	return i
}

func (i *indexCoreTest) initIndexer() *indexCoreTest {
	lockManager := storage.NewTestingLockManager()
	pdb, dbDir := unittest.TempPebbleDB(i.t)
	db := pebbleimpl.ToDB(pdb)
	i.t.Cleanup(func() {
		require.NoError(i.t, db.Close())
		require.NoError(i.t, os.RemoveAll(dbDir))
	})

	i.useDefaultHeights()

	collectionsToMarkFinalized := stdmap.NewTimes(100)
	collectionsToMarkExecuted := stdmap.NewTimes(100)
	blocksToMarkExecuted := stdmap.NewTimes(100)
	blockTransactions := stdmap.NewIdentifierMap(100)

	log := zerolog.New(os.Stdout)
	blocks := storagemock.NewBlocks(i.t)

	collectionExecutedMetric, err := NewCollectionExecutedMetricImpl(
		log,
		metrics.NewNoopCollector(),
		collectionsToMarkFinalized,
		collectionsToMarkExecuted,
		blocksToMarkExecuted,
		i.collections,
		blocks,
		blockTransactions,
	)
	require.NoError(i.t, err)

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	require.NoError(i.t, err)

	indexer, err := New(
		log,
		metrics.NewNoopCollector(),
		db,
		i.registers,
		i.headers,
		i.events,
		i.collections,
		i.transactions,
		i.results,
		flow.Testnet.Chain(),
		derivedChainData,
		collectionExecutedMetric,
		lockManager,
	)
	require.NoError(i.t, err)
	i.indexer = indexer
	return i
}

func (i *indexCoreTest) runIndexBlockData() error {
	i.initIndexer()
	return i.indexer.IndexBlockData(i.data)
}

func (i *indexCoreTest) runGetRegister(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	i.initIndexer()
	return i.indexer.RegisterValue(ID, height)
}

func TestExecutionState_IndexBlockData(t *testing.T) {
	blocks := unittest.BlockchainFixture(5)
	block := blocks[len(blocks)-1]
	collection := unittest.CollectionFixture(0)

	// this test makes sure the index block data is correctly calling store register with the
	// same entries we create as a block execution data test, and correctly converts the registers
	t.Run("Index Single Chunk and Single Register", func(t *testing.T) {
		trie := TrieUpdateRandomLedgerPayloadsFixture(t)
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{
					Collection: &collection,
					TrieUpdate: trie,
				},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		err := newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultEvents().
			useDefaultTransactionResults().
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Height)
				assert.Len(t, trie.Payloads, entries.Len())

				// make sure all the registers from the execution data have been stored as well the value matches
				trieRegistersPayloadComparer(t, trie.Payloads, entries)
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	// this test makes sure that if we have multiple trie updates in a single block data
	// and some of those trie updates are for same register but have different values,
	// we only update that register once with the latest value, so this makes sure merging of
	// registers is done correctly.
	t.Run("Index Multiple Chunks and Merge Same Register Updates", func(t *testing.T) {
		tries := []*ledger.TrieUpdate{TrieUpdateRandomLedgerPayloadsFixture(t), TrieUpdateRandomLedgerPayloadsFixture(t)}
		// make sure we have two register updates that are updating the same value, so we can check
		// if the value from the second update is being persisted instead of first
		tries[1].Paths[0] = tries[0].Paths[0]
		testValue := tries[1].Payloads[0]
		key, err := testValue.Key()
		require.NoError(t, err)
		testRegisterID, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)

		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
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
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		testRegisterFound := false
		err = newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultEvents().
			useDefaultStorageMocks().
			useDefaultTransactionResults().
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				for _, entry := range entries {
					if entry.Key.String() == testRegisterID.String() {
						testRegisterFound = true
						assert.True(t, testValue.Value().Equals(entry.Value))
					}
				}
				// we should make sure the register updates are equal to both payloads' length -1 since we don't
				// duplicate the same register
				assert.Equal(t, len(tries[0].Payloads)+len(tries[1].Payloads)-1, len(entries))
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
		assert.True(t, testRegisterFound)
	})

	t.Run("Index Events", func(t *testing.T) {
		expectedEvents := unittest.EventsFixture(20)
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				// split events into 2 chunks
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
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		err := newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultStorageMocks().
			// make sure all events are stored at once in order
			setStoreEvents(func(t *testing.T, actualBlockID flow.Identifier, actualEvents []flow.EventsList) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualEvents, 1)
				require.Len(t, actualEvents[0], len(expectedEvents))
				for i, expected := range expectedEvents {
					assert.Equal(t, expected, actualEvents[0][i])
				}
				return nil
			}).
			// make sure an empty set of transaction results were stored
			setStoreTransactionResults(func(t *testing.T, actualBlockID flow.Identifier, actualResults []flow.LightTransactionResult) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualResults, 0)
				return nil
			}).
			// make sure an empty set of register entries was stored
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Height)
				assert.Equal(t, 0, entries.Len())
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	t.Run("Index Tx Results", func(t *testing.T) {
		expectedResults := unittest.LightTransactionResultsFixture(20)
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				// split events into 2 chunks
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
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		err := newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultStorageMocks().
			// make sure an empty set of events were stored
			setStoreEvents(func(t *testing.T, actualBlockID flow.Identifier, actualEvents []flow.EventsList) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualEvents, 1)
				require.Len(t, actualEvents[0], 0)
				return nil
			}).
			// make sure all results are stored at once in order
			setStoreTransactionResults(func(t *testing.T, actualBlockID flow.Identifier, actualResults []flow.LightTransactionResult) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualResults, len(expectedResults))
				for i, expected := range expectedResults {
					assert.Equal(t, expected, actualResults[i])
				}
				return nil
			}).
			// make sure an empty set of register entries was stored
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Height)
				assert.Equal(t, 0, entries.Len())
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	t.Run("Index Collections", func(t *testing.T) {
		expectedCollections := unittest.CollectionListFixture(2)
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{Collection: expectedCollections[0]},
				{Collection: expectedCollections[1]},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)
		err := newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultStorageMocks().
			// make sure an empty set of events were stored
			setStoreEvents(func(t *testing.T, actualBlockID flow.Identifier, actualEvents []flow.EventsList) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualEvents, 1)
				require.Len(t, actualEvents[0], 0)
				return nil
			}).
			// make sure an empty set of transaction results were stored
			setStoreTransactionResults(func(t *testing.T, actualBlockID flow.Identifier, actualResults []flow.LightTransactionResult) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualResults, 0)
				return nil
			}).
			// make sure an empty set of register entries was stored
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Height)
				assert.Equal(t, 0, entries.Len())
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	t.Run("Index AllTheThings", func(t *testing.T) {
		expectedEvents := unittest.EventsFixture(20)
		expectedResults := unittest.LightTransactionResultsFixture(20)
		expectedCollections := unittest.CollectionListFixture(2)
		expectedTries := []*ledger.TrieUpdate{TrieUpdateRandomLedgerPayloadsFixture(t), TrieUpdateRandomLedgerPayloadsFixture(t)}
		expectedPayloads := make([]*ledger.Payload, 0)
		for _, trie := range expectedTries {
			expectedPayloads = append(expectedPayloads, trie.Payloads...)
		}

		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
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
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)
		err := newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultStorageMocks().
			// make sure all events are stored at once in order
			setStoreEvents(func(t *testing.T, actualBlockID flow.Identifier, actualEvents []flow.EventsList) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualEvents, 1)
				require.Len(t, actualEvents[0], len(expectedEvents))
				for i, expected := range expectedEvents {
					assert.Equal(t, expected, actualEvents[0][i])
				}
				return nil
			}).
			// make sure all results are stored at once in order
			setStoreTransactionResults(func(t *testing.T, actualBlockID flow.Identifier, actualResults []flow.LightTransactionResult) error {
				assert.Equal(t, block.ID(), actualBlockID)
				require.Len(t, actualResults, len(expectedResults))
				for i, expected := range expectedResults {
					assert.Equal(t, expected, actualResults[i])
				}
				return nil
			}).
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, actualHeight uint64) error {
				assert.Equal(t, actualHeight, block.Height)
				assert.Equal(t, entries.Len(), len(expectedPayloads))

				// make sure all the registers from the execution data have been stored as well the value matches
				trieRegistersPayloadComparer(t, expectedPayloads, entries)
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	// this test makes sure we get correct error when we try to index block that is not
	// within the range of indexed heights.
	t.Run("Invalid Heights", func(t *testing.T) {
		last := blocks[len(blocks)-1]
		ed := &execution_data.BlockExecutionData{
			BlockID: last.ID(),
		}
		execData := execution_data.NewBlockExecutionDataEntity(last.ID(), ed)
		latestHeight := blocks[len(blocks)-3].Height

		err := newIndexCoreTest(t, blocks, execData).
			// return a height one smaller than the latest block in storage
			setLastHeight(func(t *testing.T) uint64 {
				return latestHeight
			}).
			runIndexBlockData()

		assert.EqualError(t, err, fmt.Sprintf("must index block data with the next height %d, but got %d", latestHeight+1, last.Height))
	})

	// this test makes sure that if a block we try to index is not found in block storage
	// we get correct error.
	t.Run("Unknown block ID", func(t *testing.T) {
		unknownBlock := unittest.BlockFixture()
		ed := &execution_data.BlockExecutionData{
			BlockID: unknownBlock.ID(),
		}
		execData := execution_data.NewBlockExecutionDataEntity(unknownBlock.ID(), ed)

		err := newIndexCoreTest(t, blocks, execData).runIndexBlockData()

		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

}

func TestExecutionState_RegisterValues(t *testing.T) {
	t.Run("Get value for single register", func(t *testing.T) {
		blocks := unittest.BlockchainFixture(5)
		height := blocks[1].Height
		id := flow.RegisterID{
			Owner: "1",
			Key:   "2",
		}
		val := flow.RegisterValue("0x1")

		values, err := newIndexCoreTest(t, blocks, nil).
			initIndexer().
			setGetRegisters(func(t *testing.T, ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
				return val, nil
			}).
			runGetRegister(id, height)

		assert.NoError(t, err)
		assert.Equal(t, values, val)
	})
}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByID := make(map[flow.Identifier]*flow.Block, 0)
	for _, b := range blocks {
		blocksByID[b.ID()] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByID(blocksByID))
}

// trieRegistersPayloadComparer checks that trie payloads and register payloads are same, used for testing.
func trieRegistersPayloadComparer(t *testing.T, triePayloads []*ledger.Payload, registerPayloads flow.RegisterEntries) {
	assert.Equal(t, len(triePayloads), len(registerPayloads.Values()), "registers length should equal")

	// crate a lookup map that matches flow register ID to index in the payloads slice
	payloadRegID := make(map[flow.RegisterID]int)
	for i, p := range triePayloads {
		k, _ := p.Key()
		regKey, _ := convert.LedgerKeyToRegisterID(k)
		payloadRegID[regKey] = i
	}

	for _, entry := range registerPayloads {
		index, ok := payloadRegID[entry.Key]
		assert.True(t, ok, fmt.Sprintf("register entry not found for key %s", entry.Key.String()))
		val := triePayloads[index].Value()
		assert.True(t, val.Equals(entry.Value), fmt.Sprintf("payload values not same %s - %s", val, entry.Value))
	}
}

func TestIndexerIntegration_StoreAndGet(t *testing.T) {
	lockManager := storage.NewTestingLockManager()
	regOwnerAddress := unittest.RandomAddressFixture()
	regOwner := string(regOwnerAddress.Bytes())
	regKey := "code"
	registerID := flow.NewRegisterID(regOwnerAddress, regKey)

	pdb, dbDir := unittest.TempPebbleDB(t)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(dbDir))
	})

	logger := zerolog.Nop()
	metrics := metrics.NewNoopCollector()

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	require.NoError(t, err)

	// this test makes sure index values for a single register are correctly updated and always last value is returned
	t.Run("Single Index Value Changes", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet.Chain(),
				derivedChainData,
				nil,
				lockManager,
			)
			require.NoError(t, err)

			values := [][]byte{[]byte("1"), []byte("1"), []byte("2"), []byte("3"), []byte("4")}
			for i, val := range values {
				testDesc := fmt.Sprintf("test iteration number %d failed with test value %s", i, val)
				height := uint64(i + 1)
				err := storeRegisterWithValue(index, height, regOwner, regKey, val)
				require.NoError(t, err)

				results, err := index.RegisterValue(registerID, height)
				require.NoError(t, err, testDesc)
				assert.Equal(t, val, results)
			}
		})
	})

	// this test makes sure if a register is not found the value returned is nil and without an error in order for this to be
	// up to the specification script executor requires
	t.Run("Missing Register", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet.Chain(),
				derivedChainData,
				nil,
				lockManager,
			)
			require.NoError(t, err)

			value, err := index.RegisterValue(registerID, 0)
			require.Nil(t, value)
			assert.NoError(t, err)
		})
	})

	// this test makes sure that even if indexed values for a single register are requested with higher height
	// the correct highest height indexed value is returned.
	// e.g. we index A{h(1) -> X}, A{h(2) -> Y}, when we request h(4) we get value Y
	t.Run("Single Index Value At Later Heights", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet.Chain(),
				derivedChainData,
				nil,
				lockManager,
			)
			require.NoError(t, err)

			storeValues := [][]byte{[]byte("1"), []byte("2")}

			require.NoError(t, storeRegisterWithValue(index, 1, regOwner, regKey, storeValues[0]))

			require.NoError(t, index.indexRegisters(nil, 2))

			value, err := index.RegisterValue(registerID, uint64(2))
			require.NoError(t, err)
			assert.Equal(t, storeValues[0], value)

			require.NoError(t, index.indexRegisters(nil, 3))

			err = storeRegisterWithValue(index, 4, regOwner, regKey, storeValues[1])
			require.NoError(t, err)

			value, err = index.RegisterValue(registerID, uint64(4))
			require.NoError(t, err)
			assert.Equal(t, storeValues[1], value)

			value, err = index.RegisterValue(registerID, uint64(3))
			require.NoError(t, err)
			assert.Equal(t, storeValues[0], value)
		})
	})

	// this test makes sure we correctly handle weird payloads
	t.Run("Empty and Nil Payloads", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet.Chain(),
				derivedChainData,
				nil,
				lockManager,
			)
			require.NoError(t, err)

			require.NoError(t, index.indexRegisters(map[ledger.Path]*ledger.Payload{}, 1))
			require.NoError(t, index.indexRegisters(map[ledger.Path]*ledger.Payload{}, 1))
			require.NoError(t, index.indexRegisters(nil, 2))
		})
	})
}

// helper to store register at height and increment index range
func storeRegisterWithValue(indexer *IndexerCore, height uint64, owner string, key string, value []byte) error {
	payload := LedgerPayloadFixture(owner, key, value)
	return indexer.indexRegisters(map[ledger.Path]*ledger.Payload{ledger.DummyPath: payload}, height)
}
