package indexer

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	mempoolmock "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

const testTimeout = 2 * time.Second

type indexerTest struct {
	blocks        []*flow.Block
	progress      *mockProgress
	registers     *storagemock.RegisterIndex
	indexTest     *indexCoreTest
	worker        *Indexer
	executionData *mempoolmock.Mempool[flow.Identifier, *execution_data.BlockExecutionDataEntity]
	t             *testing.T
}

// newIndexerTest set up a jobqueue integration test with the worker.
// It will create blocks fixtures with the length provided as availableBlocks, and it will set heights already
// indexed to lastIndexedIndex value. Using run it should index all the remaining blocks up to all available blocks.
func newIndexerTest(t *testing.T, g *fixtures.GeneratorSuite, blocks []*flow.Block, lastIndexedIndex int) *indexerTest {
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	lastIndexedHeight := blocks[lastIndexedIndex].Height
	progress := newMockProgress()
	err := progress.SetProcessedIndex(lastIndexedHeight)
	require.NoError(t, err)

	registers := storagemock.NewRegisterIndex(t)

	indexerCoreTest := newIndexCoreTest(t, g, blocks, nil).
		setLastHeight(func(t *testing.T) uint64 {
			i, err := progress.ProcessedIndex()
			require.NoError(t, err)

			return i
		}).
		useDefaultBlockByHeight().
		useDefaultEvents().
		useDefaultTransactionResults().
		initIndexer()

	executionData := mempoolmock.NewMempool[flow.Identifier, *execution_data.BlockExecutionDataEntity](t)
	exeCache := cache.NewExecutionDataCache(
		mock.NewExecutionDataStore(t),
		indexerCoreTest.indexer.headers,
		nil,
		nil,
		executionData,
	)

	test := &indexerTest{
		t:             t,
		blocks:        blocks,
		progress:      progress,
		indexTest:     indexerCoreTest,
		executionData: executionData,
	}

	test.worker, err = NewIndexer(
		unittest.Logger(),
		test.first().Height,
		registers,
		indexerCoreTest.indexer,
		exeCache,
		test.latestHeight,
		progress,
	)
	require.NoError(t, err)

	return test
}

func (w *indexerTest) latestHeight() (uint64, error) {
	return w.last().Height, nil
}

func (w *indexerTest) last() *flow.Block {
	return w.blocks[len(w.blocks)-1]
}

func (w *indexerTest) first() *flow.Block {
	return w.blocks[0]
}

func (w *indexerTest) run(ctx irrecoverable.SignalerContext, reachHeight uint64, cancel context.CancelFunc) {
	w.worker.Start(ctx)

	unittest.RequireComponentsReadyBefore(w.t, testTimeout, w.worker)

	w.worker.OnExecutionData(nil)

	select {
	case <-ctx.Done():
		return
	case <-w.progress.WaitForIndex(reachHeight):
		cancel()
	}

	unittest.RequireCloseBefore(w.t, w.worker.Done(), testTimeout, "timeout waiting for the consumer to be done")
}

var _ storage.ConsumerProgress = (*mockProgress)(nil)

type mockProgress struct {
	index     *atomic.Uint64
	doneIndex *atomic.Uint64
	// signal to mark the progress reached an index set with WaitForIndex
	doneChan chan struct{}
}

func newMockProgress() *mockProgress {
	return &mockProgress{
		index:     atomic.NewUint64(0),
		doneIndex: atomic.NewUint64(0),
		doneChan:  make(chan struct{}),
	}
}

func (w *mockProgress) ProcessedIndex() (uint64, error) {
	return w.index.Load(), nil
}

func (w *mockProgress) SetProcessedIndex(index uint64) error {
	w.index.Store(index)

	if index > 0 && index == w.doneIndex.Load() {
		close(w.doneChan)
	}

	return nil
}

func (w *mockProgress) BatchSetProcessedIndex(_ uint64, _ storage.ReaderBatchWriter) error {
	return fmt.Errorf("batch not supported")
}

func (w *mockProgress) InitProcessedIndex(index uint64) error {
	w.index.Store(index)
	return nil
}

// WaitForIndex will trigger a signal to the consumer, so they know the test reached a certain point
func (w *mockProgress) WaitForIndex(n uint64) <-chan struct{} {
	w.doneIndex.Store(n)
	return w.doneChan
}

func TestIndexer_Success(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	blocks := g.Blocks().List(10)

	systemCollections, err := systemcollection.NewVersioned(g.ChainID().Chain(), systemcollection.Default(g.ChainID()))
	require.NoError(t, err)

	systemCollection, err := systemCollections.
		ByHeight(math.MaxUint64). // use the latest version
		SystemCollection(g.ChainID().Chain(), nil)
	require.NoError(t, err)

	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	lastIndexedIndex := 5
	test := newIndexerTest(t, g, blocks, lastIndexedIndex)

	for _, block := range blocks[lastIndexedIndex+1:] {
		blockID := block.ID()
		collection := g.Collections().Fixture()
		ed := execution_data.NewBlockExecutionDataEntity(g.Identifiers().Fixture(),
			&execution_data.BlockExecutionData{
				BlockID: blockID,
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection:         collection,
						TransactionResults: g.LightTransactionResults().ForTransactions(collection.Transactions),
					},
					{
						Collection:         systemCollection,
						TransactionResults: g.LightTransactionResults().ForTransactions(systemCollection.Transactions),
					},
				},
			})

		test.executionData.On("Get", blockID).Return(ed, true).Once()
		test.indexTest.collectionIndexer.On("IndexCollections", ed.StandardCollections()).Return(nil).Once()
		test.indexTest.registers.On("Store", flow.RegisterEntries{}, block.Height).Return(nil).Once()
	}

	signalerCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	lastHeight := test.blocks[len(test.blocks)-1].Height
	test.run(signalerCtx, lastHeight, cancel)
}

func TestIndexer_Failure(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	blocks := g.Blocks().List(10)

	systemCollections, err := systemcollection.NewVersioned(g.ChainID().Chain(), systemcollection.Default(g.ChainID()))
	require.NoError(t, err)

	systemCollection, err := systemCollections.
		ByHeight(math.MaxUint64). // use the latest version
		SystemCollection(g.ChainID().Chain(), nil)
	require.NoError(t, err)

	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	lastIndexedIndex := 5
	test := newIndexerTest(t, g, blocks, lastIndexedIndex)

	// make sure the error returned is as expected
	expectedErr := fmt.Errorf("error persisting data")

	lastHeight := blocks[len(blocks)-1].Height
	for _, block := range blocks[lastIndexedIndex+1:] {
		blockID := block.ID()
		collection := g.Collections().Fixture()
		ed := execution_data.NewBlockExecutionDataEntity(g.Identifiers().Fixture(),
			&execution_data.BlockExecutionData{
				BlockID: blockID,
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{
						Collection:         collection,
						TransactionResults: g.LightTransactionResults().ForTransactions(collection.Transactions),
					},
					{
						Collection:         systemCollection,
						TransactionResults: g.LightTransactionResults().ForTransactions(systemCollection.Transactions),
					},
				},
			})

		test.executionData.On("Get", blockID).Return(ed, true).Once()
		test.indexTest.collectionIndexer.On("IndexCollections", ed.StandardCollections()).Return(nil).Once()

		// return an error on the last block to trigger the error path
		if block.Height == lastHeight {
			test.indexTest.registers.On("Store", flow.RegisterEntries{}, block.Height).Return(expectedErr).Once()
		} else {
			test.indexTest.registers.On("Store", flow.RegisterEntries{}, block.Height).Return(nil).Once()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContextWithCallback(t, ctx, func(err error) {
		// the indexer will never reach the last height, so cancel the context to avoid hanging
		cancel()
		require.ErrorIs(t, err, expectedErr)
	})
	test.run(signalerCtx, lastHeight, cancel)
}
