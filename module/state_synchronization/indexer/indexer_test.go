package indexer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const testTimeout = 300 * time.Millisecond

type workerTest struct {
	blocks        []*flow.Block
	progress      *mockProgress
	indexTest     *indexTest
	worker        *Indexer
	executionData *mempool.ExecutionData
	t             *testing.T
}

// newWorkerTest set up a jobqueue integration test with the worker.
// It will create blocks fixtures with the length provided as availableBlocks, and it will set heights already
// indexed to lastIndexedIndex value. Using run it should index all the remaining blocks up to all available blocks.
func newWorkerTest(t *testing.T, availableBlocks int, lastIndexedIndex int) *workerTest {
	blocks := blocksFixture(availableBlocks)
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	lastIndexedHeight := blocks[lastIndexedIndex].Header.Height
	progress := &mockProgress{index: lastIndexedHeight}

	indexerTest := newIndexTest(t, blocks, nil).
		setLastHeight(func(t *testing.T) uint64 {
			return progress.index
		}).
		useDefaultBlockByHeight().
		initIndexer()

	executionData := mempool.NewExecutionData(t)
	exeCache := cache.NewExecutionDataCache(
		mock.NewExecutionDataStore(t),
		indexerTest.indexer.headers,
		nil,
		nil,
		executionData,
	)

	test := &workerTest{
		t:             t,
		blocks:        blocks,
		progress:      progress,
		indexTest:     indexerTest,
		executionData: executionData,
	}

	test.worker = NewExecutionStateWorker(
		unittest.Logger(),
		test.first().Header.Height,
		testTimeout,
		indexerTest.indexer,
		exeCache,
		test.latestHeight,
		progress,
	)

	return test
}

func (w *workerTest) setBlockDataByID(f func(ID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool)) {
	w.executionData.
		On("ByID", mocks.AnythingOfType("flow.Identifier")).
		Return(f)
}

func (w *workerTest) latestHeight() (uint64, error) {
	return w.last().Header.Height, nil
}

func (w *workerTest) last() *flow.Block {
	return w.blocks[len(w.blocks)-1]
}

func (w *workerTest) first() *flow.Block {
	return w.blocks[0]
}

func (w *workerTest) run(ctx irrecoverable.SignalerContext, cancel context.CancelFunc) {
	w.worker.Start(ctx)

	unittest.RequireComponentsReadyBefore(w.t, testTimeout, w.worker.component)

	w.worker.OnExecutionData(nil)

	// give it a bit of time to process all the blocks
	time.Sleep(testTimeout - 50)
	cancel()

	unittest.RequireCloseBefore(w.t, w.worker.component.Done(), testTimeout, "timeout waiting for the consumer to be done")
}

type mockProgress struct {
	index uint64
}

func (w *mockProgress) ProcessedIndex() (uint64, error) {
	return w.index, nil
}

func (w *mockProgress) SetProcessedIndex(index uint64) error {
	w.index = index
	return nil
}

func (w *mockProgress) InitProcessedIndex(index uint64) error {
	w.index = index
	return nil
}

func TestWorker_Success(t *testing.T) {
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	blocks := 10
	lastIndexedIndex := 5
	test := newWorkerTest(t, blocks, lastIndexedIndex)

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
	test.run(signalerCtx, cancel)

	// make sure store was called correct number of times
	test.indexTest.registers.AssertNumberOfCalls(t, "Store", blocks-lastIndexedIndex-1)
}

func TestWorker_Failure(t *testing.T) {
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	blocks := 10
	lastIndexedIndex := 5
	test := newWorkerTest(t, blocks, lastIndexedIndex)

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
		"failed to index block data at height %d: could not index register payloads at height %d: error persisting data",
		test.blocks[lastIndexedIndex].Header.Height+1,
		test.blocks[lastIndexedIndex].Header.Height+1,
	)

	_, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr)
	test.run(signalerCtx, cancel)

	// make sure store was called correct number of times
	test.indexTest.registers.AssertNumberOfCalls(t, "Store", 1) // it fails after first run
}
