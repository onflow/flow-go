package indexer

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	collectionsmock "github.com/onflow/flow-go/engine/access/ingestion/collections/mock"
	rpcconvert "github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/access"
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
	"github.com/onflow/flow-go/utils/unittest/mocks"
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
	accountTxIndex        *storagemock.AccountTransactions
	collectionIndexer     *collectionsmock.CollectionIndexer
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
		collectionIndexer:     collectionsmock.NewCollectionIndexer(t),
		blocks:                blocks,
		ctx:                   context.Background(),
		data:                  exeData,
		headers:               newBlockHeadersStorage(blocks).(*storagemock.Headers), // convert it back to mock type for tests,
	}
}

func (i *indexCoreTest) useDefaultBlockByHeight() *indexCoreTest {
	i.headers.
		On("BlockIDByHeight", mock.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			for _, b := range i.blocks {
				if b.Height == height {
					return b.ID(), nil
				}
			}
			return flow.ZeroID, fmt.Errorf("not found")
		})

	i.headers.
		On("ByHeight", mock.AnythingOfType("uint64")).
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
		On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertEvent) }),
			mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList"), mock.Anything).
		Return(func(lctx lockctx.Proof, blockID flow.Identifier, events []flow.EventsList, batch storage.ReaderBatchWriter) error {
			require.NotNil(i.t, batch)
			return f(i.t, blockID, events)
		})
	return i
}

func (i *indexCoreTest) setStoreTransactionResults(f func(*testing.T, flow.Identifier, []flow.LightTransactionResult) error) *indexCoreTest {
	i.results.
		On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertLightTransactionResult) }),
			mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.LightTransactionResult")).
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
		On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertEvent) }),
			mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList"), mock.Anything).
		Return(nil)
	return i
}

func (i *indexCoreTest) useDefaultTransactionResults() *indexCoreTest {
	i.results.
		On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertLightTransactionResult) }),
			mock.Anything, mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.LightTransactionResult")).
		Return(func(lctx lockctx.Proof, batch storage.ReaderBatchWriter, _ flow.Identifier, _ []flow.LightTransactionResult) error {
			require.True(i.t, lctx.HoldsLock(storage.LockInsertLightTransactionResult))
			require.NotNil(i.t, batch)
			return nil
		})
	return i
}

func (i *indexCoreTest) useAccountTxIndex() *indexCoreTest {
	i.accountTxIndex = storagemock.NewAccountTransactions(i.t)
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

	// Handle Go interface nil gotcha: a typed nil pointer stored in an interface is not nil.
	// We need to pass an actual nil interface value when accountTxIndex is not configured.
	var accountTxIndex storage.AccountTransactions
	if i.accountTxIndex != nil {
		accountTxIndex = i.accountTxIndex
	}

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
		i.collectionIndexer,
		collectionExecutedMetric,
		lockManager,
		accountTxIndex,
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

	tf := testutil.CompleteFixture(t, g, blocks[len(blocks)-1])
	blockID := tf.Block.ID()

	blocks = append(blocks, tf.Block)

	t.Run("Index AllTheThings", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).initIndexer()

		test.events.
			On("BatchStore", mocks.MatchLock(storage.LockInsertEvent), blockID, []flow.EventsList{tf.ExpectedEvents}, mock.Anything).
			Return(func(lctx lockctx.Proof, blockID flow.Identifier, events []flow.EventsList, batch storage.ReaderBatchWriter) error {
				require.True(t, lctx.HoldsLock(storage.LockInsertEvent))
				require.NotNil(t, batch)
				return nil
			})
		test.results.
			On("BatchStore", mocks.MatchLock(storage.LockInsertLightTransactionResult), mock.Anything, blockID, tf.ExpectedResults).
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
		test.collectionIndexer.On("IndexCollections", tf.ExpectedCollections).Return(nil).Once()
		for txID, scheduledTxID := range tf.ExpectedScheduledTransactions {
			test.scheduledTransactions.
				On("BatchIndex", mocks.MatchLock(storage.LockIndexScheduledTransaction), blockID, txID, scheduledTxID, mock.Anything).
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

	// test that reindexing the last block does not return an error and does not write any data
	t.Run("Reindexing last block", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).
			initIndexer().
			setLastHeight(func(t *testing.T) uint64 {
				return tf.Block.Height
			})

		// reset all mocks to avoid false positives
		test.events.On("BatchStore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Unset()
		test.results.On("BatchStore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Unset()
		test.scheduledTransactions.On("BatchIndex", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Unset()
		test.registers.On("Store", mock.Anything, mock.Anything).Unset()
		test.collectionIndexer.On("IndexCollections", mock.Anything).Unset()

		// setup mocks to behave as they would if the block was already indexed.
		// tx results and scheduled transactions will not be called since events returned an error.
		test.events.
			On("BatchStore", mocks.MatchLock(storage.LockInsertEvent), blockID, []flow.EventsList{tf.ExpectedEvents}, mock.Anything).
			Return(storage.ErrAlreadyExists).
			Once()
		test.collectionIndexer.
			On("IndexCollections", tf.ExpectedCollections).
			Return(nil).
			Once()
		test.registers.
			On("Store", mock.Anything, tf.Block.Height).
			Return(nil).
			Once()

		err := test.indexer.IndexBlockData(tf.ExecutionDataEntity())
		assert.NoError(t, err)
	})

	// test that account transactions are indexed when accountTxIndex is enabled
	t.Run("Index account transactions", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).
			useAccountTxIndex().
			initIndexer()

		test.events.
			On("BatchStore", mocks.MatchLock(storage.LockInsertEvent), blockID, []flow.EventsList{tf.ExpectedEvents}, mock.Anything).
			Return(nil)
		test.results.
			On("BatchStore", mocks.MatchLock(storage.LockInsertLightTransactionResult), mock.Anything, blockID, tf.ExpectedResults).
			Return(nil)
		test.registers.
			On("Store", mock.Anything, tf.Block.Height).
			Return(nil)
		test.collectionIndexer.On("IndexCollections", tf.ExpectedCollections).Return(nil).Once()
		for txID, scheduledTxID := range tf.ExpectedScheduledTransactions {
			test.scheduledTransactions.
				On("BatchIndex", mocks.MatchLock(storage.LockIndexScheduledTransaction), blockID, txID, scheduledTxID, mock.Anything).
				Return(nil)
		}

		// Expect account transactions to be stored
		var capturedEntries []access.AccountTransaction
		test.accountTxIndex.
			On("Store", tf.Block.Height, mock.AnythingOfType("[]access.AccountTransaction")).
			Run(func(args mock.Arguments) {
				capturedEntries = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		err := test.indexer.IndexBlockData(tf.ExecutionDataEntity())
		require.NoError(t, err)

		// Verify account transactions were captured
		require.NotEmpty(t, capturedEntries, "expected account transactions to be indexed")

		// Verify entries have correct block height
		for _, entry := range capturedEntries {
			assert.Equal(t, tf.Block.Height, entry.BlockHeight)
		}

		// Verify we have at least some entries with non-empty addresses (from real tx participants)
		hasNonEmptyAddr := false
		for _, entry := range capturedEntries {
			if entry.Address != flow.EmptyAddress {
				hasNonEmptyAddr = true
				break
			}
		}
		assert.True(t, hasNonEmptyAddr, "expected at least one entry with non-empty address")
	})

	// test that account transactions are NOT indexed when accountTxIndex is nil
	t.Run("Skip account transactions when disabled", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).
			initIndexer() // Note: no useAccountTxIndex()

		test.events.
			On("BatchStore", mocks.MatchLock(storage.LockInsertEvent), blockID, []flow.EventsList{tf.ExpectedEvents}, mock.Anything).
			Return(nil)
		test.results.
			On("BatchStore", mocks.MatchLock(storage.LockInsertLightTransactionResult), mock.Anything, blockID, tf.ExpectedResults).
			Return(nil)
		test.registers.
			On("Store", mock.Anything, tf.Block.Height).
			Return(nil)
		test.collectionIndexer.On("IndexCollections", tf.ExpectedCollections).Return(nil).Once()
		for txID, scheduledTxID := range tf.ExpectedScheduledTransactions {
			test.scheduledTransactions.
				On("BatchIndex", mocks.MatchLock(storage.LockIndexScheduledTransaction), blockID, txID, scheduledTxID, mock.Anything).
				Return(nil)
		}

		// accountTxIndex.Store should NOT be called since it's nil

		err := test.indexer.IndexBlockData(tf.ExecutionDataEntity())
		assert.NoError(t, err)
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
				collectionsmock.NewCollectionIndexer(t),
				nil,
				lockManager,
				nil, // accountTxIndex
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
				collectionsmock.NewCollectionIndexer(t),
				nil,
				lockManager,
				nil, // accountTxIndex
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
				collectionsmock.NewCollectionIndexer(t),
				nil,
				lockManager,
				nil, // accountTxIndex
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
				collectionsmock.NewCollectionIndexer(t),
				nil,
				lockManager,
				nil, // accountTxIndex
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
	tf := testutil.CompleteFixture(t, g, blocks[len(blocks)-1])

	chainID := g.ChainID()
	fvmEnv := systemcontracts.SystemContractsForChain(chainID).AsTemplateEnv()

	systemChunk := tf.ExecutionData.ChunkExecutionDatas[len(tf.ExecutionData.ChunkExecutionDatas)-1]
	systemResults := systemChunk.TransactionResults
	pendingExecutionEvents := systemChunk.Events

	t.Run("happy path - with scheduled transactions", func(t *testing.T) {
		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, systemResults, pendingExecutionEvents)
		require.NoError(t, err)
		require.Equal(t, tf.ExpectedScheduledTransactions, actual)
	})

	t.Run("happy path - no scheduled transactions", func(t *testing.T) {
		defaultSystemResults := append([]flow.LightTransactionResult{systemResults[0]}, systemResults[len(systemResults)-1])
		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, defaultSystemResults, nil)
		require.NoError(t, err)
		require.Empty(t, actual)
	})

	t.Run("empty system chunk returns error", func(t *testing.T) {
		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, []flow.LightTransactionResult{}, []flow.Event{})
		require.ErrorContains(t, err, "system chunk contained 0 transaction results")
		require.Nil(t, actual)
	})

	t.Run("error parsing system events", func(t *testing.T) {
		events, err := rpcconvert.CcfEventsToJsonEvents(pendingExecutionEvents)
		require.NoError(t, err)

		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, systemResults, events)
		require.ErrorContains(t, err, "could not get callback details from event")
		require.Nil(t, actual)
	})

	t.Run("no scheduled transactions and incorrect number of results", func(t *testing.T) {
		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, systemResults, []flow.Event{})
		require.ErrorContains(t, err, "system chunk contained 7 results, and 0 scheduled transactions")
		require.Nil(t, actual)
	})

	t.Run("incorrect number of results", func(t *testing.T) {
		invalidSystemResults := append(systemResults, g.LightTransactionResults().Fixture())
		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, invalidSystemResults, pendingExecutionEvents)
		require.ErrorContains(t, err, "system chunk contained 8 results, but found 5 scheduled transactions")
		require.Nil(t, actual)
	})

	t.Run("out of order system collection results", func(t *testing.T) {
		invalidSystemResults := make([]flow.LightTransactionResult, len(systemResults))
		copy(invalidSystemResults, systemResults)
		invalidSystemResults[0], invalidSystemResults[1] = invalidSystemResults[1], invalidSystemResults[0]
		actual, err := collectScheduledTransactions(fvmEnv, chainID, tf.Block.Height, invalidSystemResults, pendingExecutionEvents)
		require.ErrorContains(t, err, "system chunk result at index 0 does not match expected.")
		require.Nil(t, actual)
	})
}

// helper to store register at height and increment index range
func storeRegisterWithValue(indexer *IndexerCore, height uint64, owner string, key string, value []byte) error {
	payload := LedgerPayloadFixture(owner, key, value)
	return indexer.indexRegisters(map[ledger.Path]*ledger.Payload{ledger.DummyPath: payload}, height)
}

// newMinimalIndexerCore creates a minimal IndexerCore for testing buildAccountTxIndexEntries.
// It only initializes the fields required for the method (chain ID and event types).
func newMinimalIndexerCore(chainID flow.ChainID) *IndexerCore {
	sc := systemcontracts.SystemContractsForChain(chainID)
	ftAddr := sc.FungibleToken.Address.Hex()
	nftAddr := sc.NonFungibleToken.Address.Hex()

	return &IndexerCore{
		log:              zerolog.Nop(),
		chainID:          chainID,
		ftDepositedType:  flow.EventType(fmt.Sprintf("A.%s.FungibleToken.Deposited", ftAddr)),
		ftWithdrawnType:  flow.EventType(fmt.Sprintf("A.%s.FungibleToken.Withdrawn", ftAddr)),
		nftDepositedType: flow.EventType(fmt.Sprintf("A.%s.NonFungibleToken.Deposited", nftAddr)),
		nftWithdrawnType: flow.EventType(fmt.Sprintf("A.%s.NonFungibleToken.Withdrawn", nftAddr)),
	}
}

func TestBuildAccountTxIndexEntries(t *testing.T) {
	const testHeight = uint64(100)
	indexer := newMinimalIndexerCore(flow.Testnet)

	t.Run("empty block", func(t *testing.T) {
		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{Collection: nil}, // system chunk only
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("single transaction with distinct accounts", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		proposer := unittest.RandomAddressFixture()
		auth1 := unittest.RandomAddressFixture()
		auth2 := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = payer
				tb.ProposalKey = flow.ProposalKey{Address: proposer}
				tb.Authorizers = []flow.Address{auth1, auth2}
			},
		)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{
							Transactions: []*flow.TransactionBody{&tx},
						},
					},
					{Collection: nil}, // system chunk
				},
			},
		}

		// One entry per (account, transaction) pair = 4 entries for 4 distinct accounts
		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 4)

		// Build map to check results
		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, false)
		assertAccountEntry(t, addrMap, proposer, false)
		assertAccountEntry(t, addrMap, auth1, true)
		assertAccountEntry(t, addrMap, auth2, true)
	})

	t.Run("payer is also authorizer", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		proposer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = payer
				tb.ProposalKey = flow.ProposalKey{Address: proposer}
				tb.Authorizers = []flow.Address{payer} // payer is also authorizer
			},
		)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{
							Transactions: []*flow.TransactionBody{&tx},
						},
					},
					{Collection: nil}, // system chunk
				},
			},
		}

		// 2 unique accounts: payer (also authorizer) and proposer
		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 2)

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, proposer, false)
	})

	t.Run("multiple transactions across multiple chunks", func(t *testing.T) {
		account1 := unittest.RandomAddressFixture()
		account2 := unittest.RandomAddressFixture()
		account3 := unittest.RandomAddressFixture()

		tx1 := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = account1
				tb.ProposalKey = flow.ProposalKey{Address: account1}
				tb.Authorizers = []flow.Address{account1}
			},
		)

		tx2 := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = account2
				tb.ProposalKey = flow.ProposalKey{Address: account2}
				tb.Authorizers = []flow.Address{account2}
			},
		)

		tx3 := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = account3
				tb.ProposalKey = flow.ProposalKey{Address: account3}
				tb.Authorizers = []flow.Address{account3}
			},
		)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{
							Transactions: []*flow.TransactionBody{&tx1, &tx2},
						},
					},
					{
						Collection: &flow.Collection{
							Transactions: []*flow.TransactionBody{&tx3},
						},
					},
					{Collection: nil}, // system chunk
				},
			},
		}

		// Each transaction has 1 unique account (payer=proposer=authorizer)
		// So 3 transactions = 3 entries
		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 3)

		// Group entries by transaction
		entriesByTx := make(map[flow.Identifier][]access.AccountTransaction)
		for _, entry := range entries {
			entriesByTx[entry.TransactionID] = append(entriesByTx[entry.TransactionID], entry)
		}

		// Verify each transaction has one entry with correct tx index
		tx1Addrs := toAddressMap(entriesByTx[tx1.ID()])
		require.Len(t, tx1Addrs, 1)
		assertAccountEntry(t, tx1Addrs, account1, true)

		tx2Addrs := toAddressMap(entriesByTx[tx2.ID()])
		require.Len(t, tx2Addrs, 1)
		assertAccountEntry(t, tx2Addrs, account2, true)

		tx3Addrs := toAddressMap(entriesByTx[tx3.ID()])
		require.Len(t, tx3Addrs, 1)
		assertAccountEntry(t, tx3Addrs, account3, true)
	})
}

// Helper functions for creating CCF-encoded transfer events

// createFTDepositedEvent creates a CCF-encoded FungibleToken.Deposited event
func createFTDepositedEvent(t *testing.T, eventType flow.EventType, txIndex uint32, toAddr *flow.Address) flow.Event {
	// Build the "to" field as optional String (hex address)
	var toValue cadence.Value
	if toAddr != nil {
		addrStr, err := cadence.NewString(toAddr.Hex())
		require.NoError(t, err)
		toValue = cadence.NewOptional(addrStr)
	} else {
		toValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"FungibleToken.Deposited",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "amount", Type: cadence.UFix64Type},
			{Identifier: "to", Type: cadence.NewOptionalType(cadence.StringType)},
			{Identifier: "toUUID", Type: cadence.UInt64Type},
			{Identifier: "depositedUUID", Type: cadence.UInt64Type},
			{Identifier: "balanceAfter", Type: cadence.UFix64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.FlowToken.Vault"),
		cadence.UFix64(100_00000000), // 100.0
		toValue,
		cadence.UInt64(1),
		cadence.UInt64(2),
		cadence.UFix64(200_00000000),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createFTWithdrawnEvent creates a CCF-encoded FungibleToken.Withdrawn event
func createFTWithdrawnEvent(t *testing.T, eventType flow.EventType, txIndex uint32, fromAddr *flow.Address) flow.Event {
	var fromValue cadence.Value
	if fromAddr != nil {
		addrStr, err := cadence.NewString(fromAddr.Hex())
		require.NoError(t, err)
		fromValue = cadence.NewOptional(addrStr)
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"FungibleToken.Withdrawn",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "amount", Type: cadence.UFix64Type},
			{Identifier: "from", Type: cadence.NewOptionalType(cadence.StringType)},
			{Identifier: "fromUUID", Type: cadence.UInt64Type},
			{Identifier: "withdrawnUUID", Type: cadence.UInt64Type},
			{Identifier: "balanceAfter", Type: cadence.UFix64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.FlowToken.Vault"),
		cadence.UFix64(50_00000000),
		fromValue,
		cadence.UInt64(1),
		cadence.UInt64(3),
		cadence.UFix64(150_00000000),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createNFTDepositedEvent creates a CCF-encoded NonFungibleToken.Deposited event
func createNFTDepositedEvent(t *testing.T, eventType flow.EventType, txIndex uint32, toAddr *flow.Address) flow.Event {
	var toValue cadence.Value
	if toAddr != nil {
		addrStr, err := cadence.NewString(toAddr.Hex())
		require.NoError(t, err)
		toValue = cadence.NewOptional(addrStr)
	} else {
		toValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"NonFungibleToken.Deposited",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "uuid", Type: cadence.UInt64Type},
			{Identifier: "to", Type: cadence.NewOptionalType(cadence.StringType)},
			{Identifier: "collectionUUID", Type: cadence.UInt64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.TopShot.NFT"),
		cadence.UInt64(42),
		cadence.UInt64(100),
		toValue,
		cadence.UInt64(5),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createNFTWithdrawnEvent creates a CCF-encoded NonFungibleToken.Withdrawn event
func createNFTWithdrawnEvent(t *testing.T, eventType flow.EventType, txIndex uint32, fromAddr *flow.Address) flow.Event {
	var fromValue cadence.Value
	if fromAddr != nil {
		addrStr, err := cadence.NewString(fromAddr.Hex())
		require.NoError(t, err)
		fromValue = cadence.NewOptional(addrStr)
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"NonFungibleToken.Withdrawn",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "uuid", Type: cadence.UInt64Type},
			{Identifier: "from", Type: cadence.NewOptionalType(cadence.StringType)},
			{Identifier: "providerUUID", Type: cadence.UInt64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.TopShot.NFT"),
		cadence.UInt64(42),
		cadence.UInt64(100),
		fromValue,
		cadence.UInt64(5),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

func TestBuildAccountTxIndexEntries_TransferEvents(t *testing.T) {
	const testHeight = uint64(100)
	indexer := newMinimalIndexerCore(flow.Testnet)

	t.Run("FT deposited event adds recipient", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		depositEvent := createFTDepositedEvent(t, indexer.ftDepositedType, 0, &recipient)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{depositEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 2) // payer + recipient

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, recipient, false)
	})

	t.Run("FT withdrawn event adds sender", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		withdrawEvent := createFTWithdrawnEvent(t, indexer.ftWithdrawnType, 0, &sender)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{withdrawEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 2) // payer + sender

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, sender, false)
	})

	t.Run("NFT deposited event adds recipient", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		depositEvent := createNFTDepositedEvent(t, indexer.nftDepositedType, 0, &recipient)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{depositEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 2)

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, recipient, false)
	})

	t.Run("NFT withdrawn event adds sender", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		withdrawEvent := createNFTWithdrawnEvent(t, indexer.nftWithdrawnType, 0, &sender)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{withdrawEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 2)

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, sender, false)
	})

	t.Run("transfer event with nil address is skipped", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		// Create event with nil address
		depositEvent := createFTDepositedEvent(t, indexer.ftDepositedType, 0, nil)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{depositEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 1) // only payer, no recipient from nil address

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
	})

	t.Run("transfer recipient same as payer does not duplicate", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		// Transfer to the same address as payer
		depositEvent := createFTDepositedEvent(t, indexer.ftDepositedType, 0, &payer)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{depositEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 1) // deduplicated
		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
	})

	t.Run("transfer sender does not override authorizer status", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{sender} // sender is an authorizer
		})

		// Transfer event to the authorizer
		depositEvent := createFTDepositedEvent(t, indexer.ftDepositedType, 0, &sender)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{depositEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 2) // payer + auth

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, sender, true)
		assertAccountEntry(t, addrMap, payer, false)
	})

	t.Run("multiple transfer events in same transaction", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		withdrawEvent := createFTWithdrawnEvent(t, indexer.ftWithdrawnType, 0, &sender)
		depositEvent := createFTDepositedEvent(t, indexer.ftDepositedType, 0, &recipient)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{withdrawEvent, depositEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 3) // payer + sender + recipient

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, sender, false)
		assertAccountEntry(t, addrMap, recipient, false)
	})

	t.Run("events across multiple chunks with correct txIndex", func(t *testing.T) {
		account1 := unittest.RandomAddressFixture()
		account2 := unittest.RandomAddressFixture()
		recipient1 := unittest.RandomAddressFixture()
		recipient2 := unittest.RandomAddressFixture()

		tx1 := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = account1
			tb.ProposalKey = flow.ProposalKey{Address: account1}
			tb.Authorizers = []flow.Address{account1}
		})

		tx2 := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = account2
			tb.ProposalKey = flow.ProposalKey{Address: account2}
			tb.Authorizers = []flow.Address{account2}
		})

		// Event for tx1 (txIndex=0) in chunk 0
		event1 := createFTDepositedEvent(t, indexer.ftDepositedType, 0, &recipient1)
		// Event for tx2 (txIndex=1) in chunk 1
		event2 := createNFTDepositedEvent(t, indexer.nftDepositedType, 1, &recipient2)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
						Events:     []flow.Event{event1},
					},
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx2}},
						Events:     []flow.Event{event2},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 4) // account1+recipient1 + account2+recipient2

		// Group by transaction
		entriesByTx := make(map[flow.Identifier][]access.AccountTransaction)
		for _, e := range entries {
			entriesByTx[e.TransactionID] = append(entriesByTx[e.TransactionID], e)
		}

		// tx1 entries should have txIndex=0 and include recipient1
		tx1Addrs := toAddressMap(entriesByTx[tx1.ID()])
		require.Len(t, tx1Addrs, 2)
		assertAccountEntry(t, tx1Addrs, account1, true)
		assertAccountEntry(t, tx1Addrs, recipient1, false)

		// tx2 entries should have txIndex=1 and include recipient2
		tx2Addrs := toAddressMap(entriesByTx[tx2.ID()])
		require.Len(t, tx2Addrs, 2)
		assertAccountEntry(t, tx2Addrs, account2, true)
		assertAccountEntry(t, tx2Addrs, recipient2, false)
	})

	t.Run("malformed event payload returns error", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		// Create event with invalid CCF payload
		badEvent := flow.Event{
			Type:             indexer.ftDepositedType,
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          []byte("not valid ccf"),
		}

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{badEvent},
					},
					{Collection: nil},
				},
			},
		}

		_, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to extract address from event")
	})

	t.Run("non-transfer events are ignored", func(t *testing.T) {
		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		// Create a random non-transfer event
		randomEvent := unittest.EventFixture(
			unittest.Event.WithTransactionIndex(0),
		)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx}},
						Events:     []flow.Event{randomEvent},
					},
					{Collection: nil},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)
		require.Len(t, entries, 1) // only payer

		addrMap := toAddressMap(entries)
		assertAccountEntry(t, addrMap, payer, true)
	})

	t.Run("transfer events in system chunk (scheduled transactions) are indexed", func(t *testing.T) {
		// User chunk: 2 transactions
		userAccount1 := unittest.RandomAddressFixture()
		userAccount2 := unittest.RandomAddressFixture()

		userTx1 := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = userAccount1
			tb.ProposalKey = flow.ProposalKey{Address: userAccount1}
			tb.Authorizers = []flow.Address{userAccount1}
		})
		userTx2 := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = userAccount2
			tb.ProposalKey = flow.ProposalKey{Address: userAccount2}
			tb.Authorizers = []flow.Address{userAccount2}
		})

		// System chunk: 1 scheduled transaction (txIndex=2)
		scheduledTxPayer := unittest.RandomAddressFixture()
		scheduledTx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = scheduledTxPayer
			tb.ProposalKey = flow.ProposalKey{Address: scheduledTxPayer}
			tb.Authorizers = []flow.Address{scheduledTxPayer}
		})

		// Transfer event in the scheduled transaction (txIndex=2)
		scheduledTxRecipient := unittest.RandomAddressFixture()
		scheduledTxTransferEvent := createFTDepositedEvent(t, indexer.ftDepositedType, 2, &scheduledTxRecipient)

		execData := &execution_data.BlockExecutionDataEntity{
			BlockExecutionData: &execution_data.BlockExecutionData{
				BlockID: unittest.IdentifierFixture(),
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					// User chunk with 2 transactions
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&userTx1, &userTx2}},
						Events:     []flow.Event{},
					},
					// System chunk with 1 scheduled transaction and transfer event
					{
						Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&scheduledTx}},
						Events:     []flow.Event{scheduledTxTransferEvent},
					},
				},
			},
		}

		entries, err := indexer.buildAccountTxIndexEntries(testHeight, execData)
		require.NoError(t, err)

		// Expected: userAccount1 + userAccount2 + scheduledTxPayer + scheduledTxRecipient = 4 entries
		require.Len(t, entries, 4)

		// Group by transaction
		entriesByTx := make(map[flow.Identifier][]access.AccountTransaction)
		for _, e := range entries {
			entriesByTx[e.TransactionID] = append(entriesByTx[e.TransactionID], e)
		}

		// Verify user transactions
		userTx1Addrs := toAddressMap(entriesByTx[userTx1.ID()])
		require.Len(t, userTx1Addrs, 1)
		assertAccountEntry(t, userTx1Addrs, userAccount1, true)

		userTx2Addrs := toAddressMap(entriesByTx[userTx2.ID()])
		require.Len(t, userTx2Addrs, 1)
		assertAccountEntry(t, userTx2Addrs, userAccount2, true)

		// Verify scheduled transaction includes both payer AND transfer recipient
		scheduledTxAddrs := toAddressMap(entriesByTx[scheduledTx.ID()])
		require.Len(t, scheduledTxAddrs, 2, "scheduled tx should have payer and transfer recipient")
		assertAccountEntry(t, scheduledTxAddrs, scheduledTxPayer, true)
		assertAccountEntry(t, scheduledTxAddrs, scheduledTxRecipient, false)

		// Verify transaction indices are correct
		for _, e := range entriesByTx[userTx1.ID()] {
			assert.Equal(t, uint32(0), e.TransactionIndex)
		}
		for _, e := range entriesByTx[userTx2.ID()] {
			assert.Equal(t, uint32(1), e.TransactionIndex)
		}
		for _, e := range entriesByTx[scheduledTx.ID()] {
			assert.Equal(t, uint32(2), e.TransactionIndex)
		}
	})
}

func toAddressMap(entries []access.AccountTransaction) map[flow.Address]bool {
	addrMap := make(map[flow.Address]bool)
	for _, e := range entries {
		addrMap[e.Address] = e.IsAuthorizer
	}
	return addrMap
}

func assertAccountEntry(t *testing.T, addrMap map[flow.Address]bool, addr flow.Address, expectedIsAuthorizer bool) {
	isAuthorizer, ok := addrMap[addr]
	require.True(t, ok)
	assert.Equal(t, expectedIsAuthorizer, isAuthorizer)
}
