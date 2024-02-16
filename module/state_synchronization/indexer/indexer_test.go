package indexer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const testTimeout = 300 * time.Millisecond

type indexerTest struct {
	blocks        []*flow.Block
	progress      *mockProgress
	registers     *storagemock.RegisterIndex
	indexTest     *indexCoreTest
	worker        *Indexer
	executionData *mempool.ExecutionData
	t             *testing.T
}

// newIndexerTest set up a jobqueue integration test with the worker.
// It will create blocks fixtures with the length provided as availableBlocks, and it will set heights already
// indexed to lastIndexedIndex value. Using run it should index all the remaining blocks up to all available blocks.
func newIndexerTest(t *testing.T, availableBlocks int, lastIndexedIndex int) *indexerTest {
	blocks := unittest.BlockchainFixture(availableBlocks)
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	lastIndexedHeight := blocks[lastIndexedIndex].Header.Height
	progress := newMockProgress()
	err := progress.SetProcessedIndex(lastIndexedHeight)
	require.NoError(t, err)

	registers := storagemock.NewRegisterIndex(t)

	indexerCoreTest := newIndexCoreTest(t, blocks, nil).
		setLastHeight(func(t *testing.T) uint64 {
			i, err := progress.ProcessedIndex()
			require.NoError(t, err)

			return i
		}).
		useDefaultBlockByHeight().
		useDefaultEvents().
		useDefaultTransactionResults().
		initIndexer()

	executionData := mempool.NewExecutionData(t)
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
		test.first().Header.Height,
		func() (uint64, error) {
			return registers.FirstHeight(), nil
		},
		indexerCoreTest.indexer,
		exeCache,
		test.latestHeight,
		progress,
	)
	require.NoError(t, err)

	return test
}

func (w *indexerTest) setBlockDataByID(f func(ID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool)) {
	w.executionData.
		On("ByID", mocks.AnythingOfType("flow.Identifier")).
		Return(f)
}

func (w *indexerTest) latestHeight() (uint64, error) {
	return w.last().Header.Height, nil
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

	// wait for end to be reached
	<-w.progress.WaitForIndex(reachHeight)
	cancel()

	unittest.RequireCloseBefore(w.t, w.worker.Done(), testTimeout, "timeout waiting for the consumer to be done")
}

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
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	blocks := 10
	lastIndexedIndex := 5
	test := newIndexerTest(t, blocks, lastIndexedIndex)

	test.setBlockDataByID(func(ID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool) {
		trie := trieUpdateFixture(t)
		ed := &execution_data.BlockExecutionData{
			BlockID: ID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: trie},
			},
		}

		// create this to capture the closure of the creation of block execution data, so we can for each returned
		// block execution data make sure the store of registers will match what the execution data returned and
		// also that the height was correct
		test.indexTest.setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
			var blockHeight uint64
			for _, b := range test.blocks {
				if b.ID() == ID {
					blockHeight = b.Header.Height
				}
			}

			assert.Equal(t, blockHeight, height)
			trieRegistersPayloadComparer(t, trie.Payloads, entries)
			return nil
		})

		return execution_data.NewBlockExecutionDataEntity(ID, ed), true
	})

	signalerCtx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	lastHeight := test.blocks[len(test.blocks)-1].Header.Height
	test.run(signalerCtx, lastHeight, cancel)

	// make sure store was called correct number of times
	test.indexTest.registers.AssertNumberOfCalls(t, "Store", blocks-lastIndexedIndex-1)
}

func TestIndexer_Failure(t *testing.T) {
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	blocks := 10
	lastIndexedIndex := 5
	test := newIndexerTest(t, blocks, lastIndexedIndex)

	test.setBlockDataByID(func(ID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool) {
		trie := trieUpdateFixture(t)
		ed := &execution_data.BlockExecutionData{
			BlockID: ID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: trie},
			},
		}

		// fail when trying to persist registers
		test.indexTest.setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
			return fmt.Errorf("error persisting data")
		})

		return execution_data.NewBlockExecutionDataEntity(ID, ed), true
	})

	// make sure the error returned is as expected
	expectedErr := fmt.Errorf(
		"failed to index block data at height %d: %w",
		test.blocks[lastIndexedIndex].Header.Height+1,
		fmt.Errorf(
			"could not index register payloads at height %d: %w", test.blocks[lastIndexedIndex].Header.Height+1, fmt.Errorf("error persisting data")),
	)

	_, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr)
	lastHeight := test.blocks[lastIndexedIndex].Header.Height
	test.run(signalerCtx, lastHeight, cancel)

	// make sure store was called correct number of times
	test.indexTest.registers.AssertNumberOfCalls(t, "Store", 1) // it fails after first run
}
