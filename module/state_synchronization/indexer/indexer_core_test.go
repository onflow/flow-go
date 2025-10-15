package indexer

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	rpcconvert "github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/testutil"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type indexCoreTest struct {
	t                     *testing.T
	g                     *fixtures.GeneratorSuite
	indexer               *IndexerCore
	registers             *storagemock.RegisterIndex
	events                *storagemock.Events
	collections           *storagemock.Collections
	transactions          *storagemock.Transactions
	results               *storagemock.LightTransactionResults
	headers               *storagemock.Headers
	scheduledTransactions *storagemock.ScheduledTransactions
	ctx                   context.Context
	blocks                []*flow.Block
	data                  *execution_data.BlockExecutionDataEntity
	lastHeightStore       func(t *testing.T) uint64
	firstHeightStore      func(t *testing.T) uint64
	registersStore        func(t *testing.T, entries flow.RegisterEntries, height uint64) error
	eventsStore           func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error
	registersGet          func(t *testing.T, IDs flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

func newIndexCoreTest(
	t *testing.T,
	g *fixtures.GeneratorSuite,
	blocks []*flow.Block,
	exeData *execution_data.BlockExecutionDataEntity,
) *indexCoreTest {
	return &indexCoreTest{
		t:                     t,
		g:                     g,
		registers:             storagemock.NewRegisterIndex(t),
		events:                storagemock.NewEvents(t),
		results:               storagemock.NewLightTransactionResults(t),
		collections:           storagemock.NewCollections(t),
		transactions:          storagemock.NewTransactions(t),
		scheduledTransactions: storagemock.NewScheduledTransactions(t),
		blocks:                blocks,
		ctx:                   context.Background(),
		data:                  exeData,
		headers:               newBlockHeadersStorage(blocks).(*storagemock.Headers), // convert it back to mock type for tests,
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
		On("BatchStore", mock.Anything, mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.LightTransactionResult")).
		Return(func(lctx lockctx.Proof, batch storage.ReaderBatchWriter, blockID flow.Identifier, results []flow.LightTransactionResult) error {
			require.True(i.t, lctx.HoldsLock(storage.LockInsertLightTransactionResult))
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

func (i *indexCoreTest) useDefaultEvents() *indexCoreTest {
	i.events.
		On("BatchStore", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList"), mock.Anything).
		Return(nil)
	return i
}

func (i *indexCoreTest) useDefaultTransactionResults() *indexCoreTest {
	i.results.
		On("BatchStore", mock.Anything, mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.LightTransactionResult")).
		Return(func(lctx lockctx.Proof, batch storage.ReaderBatchWriter, _ flow.Identifier, _ []flow.LightTransactionResult) error {
			require.True(i.t, lctx.HoldsLock(storage.LockInsertLightTransactionResult))
			require.NotNil(i.t, batch)
			return nil
		})
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

	log := unittest.Logger()
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

	i.indexer = New(
		log,
		metrics.NewNoopCollector(),
		db,
		i.registers,
		i.headers,
		i.events,
		i.collections,
		i.transactions,
		i.results,
		i.scheduledTransactions,
		i.g.ChainID(),
		derivedChainData,
		collectionExecutedMetric,
		lockManager,
	)
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
	g := fixtures.NewGeneratorSuite()
	blocks := g.Blocks().List(4)

	tf := testutil.CompleteFixture(t, g, blocks[len(blocks)-1].ToHeader())
	blockID := tf.Block.ID()

	blocks = append(blocks, tf.Block)

	t.Run("Index AllTheThings", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).initIndexer()

		test.events.On("BatchStore", mock.Anything, []flow.EventsList{tf.ExpectedEvents}, mock.Anything).
			Return(func(blockID flow.Identifier, events []flow.EventsList, batch storage.ReaderBatchWriter) error {
				require.NotNil(t, batch)
				// Events BatchStore doesn't require specific locks, but we validate the batch is provided
				return nil
			})
		test.results.On("BatchStore", mock.Anything, mock.Anything, blockID, tf.ExpectedResults).
			Return(func(lctx lockctx.Proof, batch storage.ReaderBatchWriter, blockID flow.Identifier, results []flow.LightTransactionResult) error {
				require.True(t, lctx.HoldsLock(storage.LockInsertLightTransactionResult))
				require.NotNil(t, batch)
				return nil
			})
		test.registers.
			On("Store", mock.Anything, tf.Block.Height).
			Run(func(args mock.Arguments) {
				// registers collected with a map, so will be in random order
				entries := args[0].(flow.RegisterEntries)
				assert.ElementsMatch(t, tf.ExpectedRegisterEntries, entries)
			}).
			Return(nil)
		for _, collection := range tf.ExpectedCollections {
			test.collections.On("StoreAndIndexByTransaction", mock.Anything, collection).Return(&flow.LightCollection{}, nil)
		}
		for txID, scheduledTxID := range tf.ExpectedScheduledTransactions {
			test.scheduledTransactions.On("BatchIndex", mock.Anything, blockID, txID, scheduledTxID, mock.Anything).
				Return(func(lctx lockctx.Proof, blockID flow.Identifier, txID flow.Identifier, scheduledTxID uint64, batch storage.ReaderBatchWriter) error {
					require.True(t, lctx.HoldsLock(storage.LockIndexScheduledTransaction))
					require.NotNil(t, batch)
					return nil
				})
		}

		err := test.indexer.IndexBlockData(tf.ExecutionDataEntity())

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

		err := newIndexCoreTest(t, g, blocks, execData).
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

		err := newIndexCoreTest(t, g, blocks, execData).runIndexBlockData()

		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

}

func TestExecutionState_RegisterValues(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	t.Run("Get value for single register", func(t *testing.T) {
		blocks := g.Blocks().List(5)
		height := blocks[1].Height
		id := flow.RegisterID{
			Owner: "1",
			Key:   "2",
		}
		val := flow.RegisterValue("0x1")

		values, err := newIndexCoreTest(t, g, blocks, nil).
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
			index := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet,
				derivedChainData,
				nil,
				lockManager,
			)

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
			index := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet,
				derivedChainData,
				nil,
				lockManager,
			)

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
			index := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet,
				derivedChainData,
				nil,
				lockManager,
			)

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
			index := New(
				logger,
				module.ExecutionStateIndexerMetrics(metrics),
				pebbleimpl.ToDB(pdb),
				registers,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				flow.Testnet,
				derivedChainData,
				nil,
				lockManager,
			)

			require.NoError(t, index.indexRegisters(map[ledger.Path]*ledger.Payload{}, 1))
			require.NoError(t, index.indexRegisters(map[ledger.Path]*ledger.Payload{}, 1))
			require.NoError(t, index.indexRegisters(nil, 2))
		})
	})
}

func TestCollectScheduledTransactions(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	blocks := g.Blocks().List(5)
	tf := testutil.CompleteFixture(t, g, blocks[len(blocks)-1].ToHeader())

	chainID := g.ChainID()
	fvmEnv := systemcontracts.SystemContractsForChain(chainID).AsTemplateEnv()

	systemChunk := tf.ExecutionData.ChunkExecutionDatas[len(tf.ExecutionData.ChunkExecutionDatas)-1]
	systemResults := systemChunk.TransactionResults
	pendingExecutionEvents := systemChunk.Events

	t.Run("happy path - with scheduled transactions", func(t *testing.T) {
		actual, err := collectScheduledTransactions(fvmEnv, chainID, systemResults, pendingExecutionEvents)
		require.NoError(t, err)
		require.Equal(t, tf.ExpectedScheduledTransactions, actual)
	})

	t.Run("happy path - no scheduled transactions", func(t *testing.T) {
		defaultSystemResults := append([]flow.LightTransactionResult{systemResults[0]}, systemResults[len(systemResults)-1])
		actual, err := collectScheduledTransactions(fvmEnv, chainID, defaultSystemResults, nil)
		require.NoError(t, err)
		require.Empty(t, actual)
	})

	t.Run("empty system chunk returns error", func(t *testing.T) {
		actual, err := collectScheduledTransactions(fvmEnv, chainID, []flow.LightTransactionResult{}, []flow.Event{})
		require.ErrorContains(t, err, "system chunk contained 0 transaction results")
		require.Nil(t, actual)
	})

	t.Run("error parsing system events", func(t *testing.T) {
		events, err := rpcconvert.CcfEventsToJsonEvents(pendingExecutionEvents)
		require.NoError(t, err)

		actual, err := collectScheduledTransactions(fvmEnv, chainID, systemResults, events)
		require.ErrorContains(t, err, "could not get callback details from event")
		require.Nil(t, actual)
	})

	t.Run("no scheduled transactions and incorrect number of results", func(t *testing.T) {
		actual, err := collectScheduledTransactions(fvmEnv, chainID, systemResults, []flow.Event{})
		require.ErrorContains(t, err, "system chunk contained 7 results, and 0 scheduled transactions")
		require.Nil(t, actual)
	})

	t.Run("incorrect number of results", func(t *testing.T) {
		invalidSystemResults := append(systemResults, g.LightTransactionResults().Fixture())
		actual, err := collectScheduledTransactions(fvmEnv, chainID, invalidSystemResults, pendingExecutionEvents)
		require.ErrorContains(t, err, "system chunk contained 8 results, but found 5 scheduled callbacks")
		require.Nil(t, actual)
	})

	t.Run("out of order system collection results", func(t *testing.T) {
		invalidSystemResults := make([]flow.LightTransactionResult, len(systemResults))
		copy(invalidSystemResults, systemResults)
		invalidSystemResults[0], invalidSystemResults[1] = invalidSystemResults[1], invalidSystemResults[0]
		actual, err := collectScheduledTransactions(fvmEnv, chainID, invalidSystemResults, pendingExecutionEvents)
		require.ErrorContains(t, err, "system chunk result at index 0 does not match expected.")
		require.Nil(t, actual)
	})
}

// helper to store register at height and increment index range
func storeRegisterWithValue(indexer *IndexerCore, height uint64, owner string, key string, value []byte) error {
	payload := LedgerPayloadFixture(owner, key, value)
	return indexer.indexRegisters(map[ledger.Path]*ledger.Payload{ledger.DummyPath: payload}, height)
}
