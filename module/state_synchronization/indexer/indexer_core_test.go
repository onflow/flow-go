package indexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	collectionsmock "github.com/onflow/flow-go/engine/access/ingestion/collections/mock"
	rpcconvert "github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/testutil"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
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
		i.collectionIndexer,
		collectionExecutedMetric,
		lockManager,
		nil,
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

	// test that extended indexer is invoked when configured
	t.Run("Index account transactions", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).initIndexer()

		startHeight := tf.Block.Height - 1
		done := make(chan struct{})
		var gotHeight uint64
		stub := &testExtendedIndexer{
			name:         "test",
			latestHeight: startHeight,
			indexBlockFn: func(data extended.BlockData) error {
				gotHeight = data.Header.Height
				close(done)
				return nil
			},
		}

		mockState := newMockStateForBlock(t, tf.Block)
		extendedIndexer, err := extended.NewExtendedIndexer(
			test.indexer.log,
			metrics.NewNoopCollector(),
			test.indexer.protocolDB,
			storage.NewTestingLockManager(),
			mockState,
			storagemock.NewIndex(t),
			storagemock.NewHeaders(t),
			storagemock.NewGuarantees(t),
			test.collections,
			test.events,
			test.results,
			[]extended.Indexer{stub},
			test.indexer.chainID,
			extended.DefaultBackfillDelay,
		)
		require.NoError(t, err)

		// Start the ExtendedIndexer component so its ingest loop can process data
		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
		extendedIndexer.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 5*time.Second, extendedIndexer)
		t.Cleanup(func() {
			cancel()
			unittest.RequireCloseBefore(t, extendedIndexer.Done(), 5*time.Second, "timeout waiting for shutdown")
		})

		test.indexer.extendedIndexer = extendedIndexer

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

		err = test.indexer.IndexBlockData(tf.ExecutionDataEntity())
		require.NoError(t, err)

		// Wait for the async extended indexer to process the block
		unittest.RequireCloseBefore(t, done, 5*time.Second, "timeout waiting for extended indexer")
		assert.Equal(t, tf.Block.Height, gotHeight)
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

	// test that errors from the extended indexer are propagated via irrecoverable context
	t.Run("Account transactions error propagation", func(t *testing.T) {
		test := newIndexCoreTest(t, g, blocks, tf.ExecutionDataEntity()).initIndexer()

		startHeight := tf.Block.Height - 1
		expectedErr := errors.New("indexer failure")
		stub := &testExtendedIndexer{
			name:         "test",
			latestHeight: startHeight,
			indexBlockFn: func(data extended.BlockData) error {
				return expectedErr
			},
		}

		mockState := newMockStateForBlock(t, tf.Block)
		extendedIndexer, err := extended.NewExtendedIndexer(
			test.indexer.log,
			metrics.NewNoopCollector(),
			test.indexer.protocolDB,
			storage.NewTestingLockManager(),
			mockState,
			storagemock.NewIndex(t),
			storagemock.NewHeaders(t),
			storagemock.NewGuarantees(t),
			test.collections,
			test.events,
			test.results,
			[]extended.Indexer{stub},
			test.indexer.chainID,
			extended.DefaultBackfillDelay,
		)
		require.NoError(t, err)

		// Start the ExtendedIndexer with an error callback to capture thrown errors
		thrown := make(chan error, 1)
		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx := irrecoverable.NewMockSignalerContextWithCallback(t, ctx, func(err error) {
			thrown <- err
			cancel()
		})
		extendedIndexer.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 5*time.Second, extendedIndexer)
		t.Cleanup(func() {
			cancel()
		})

		test.indexer.extendedIndexer = extendedIndexer

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

		err = test.indexer.IndexBlockData(tf.ExecutionDataEntity())
		// IndexBlockData itself succeeds - the error is thrown asynchronously by the extended indexer
		require.NoError(t, err)

		// Wait for the error to be thrown via irrecoverable context
		select {
		case err := <-thrown:
			assert.ErrorIs(t, err, expectedErr)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for error propagation")
		}
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

// newMockStateForBlock returns a mock protocol.State configured so that
// AtBlockID returns the block's header.
func newMockStateForBlock(t *testing.T, block *flow.Block) *protocolmock.State {
	snapshot := protocolmock.NewSnapshot(t)
	snapshot.On("Head").Return(block.ToHeader(), nil)

	state := protocolmock.NewState(t)
	state.On("AtBlockID", block.ID()).Return(snapshot)
	return state
}

type testExtendedIndexer struct {
	name         string
	latestHeight uint64
	indexBlockFn func(data extended.BlockData) error
}

func (t *testExtendedIndexer) Name() string { return t.name }

func (t *testExtendedIndexer) IndexBlockData(_ lockctx.Proof, data extended.BlockData, _ storage.ReaderBatchWriter) error {
	if t.indexBlockFn != nil {
		return t.indexBlockFn(data)
	}
	return nil
}

func (t *testExtendedIndexer) NextHeight() (uint64, error) {
	return t.latestHeight + 1, nil
}
