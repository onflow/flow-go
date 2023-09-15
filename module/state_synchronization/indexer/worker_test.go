package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
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
	blocks   []*flow.Block
	progress *mockProgress
}

func (w *workerTest) LatestHeight() (uint64, error) {
	return w.last().Header.Height, nil
}

func (w *workerTest) last() *flow.Block {
	return w.blocks[len(w.blocks)-1]
}

func (w *workerTest) first() *flow.Block {
	return w.blocks[0]
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
	blocks := blocksFixture(10)
	// we use 5th index as the latest indexed height, so we leave 5 more blocks to be indexed by the indexer in this test
	lastIndexedIndex := 5
	lastIndexedHeight := blocks[lastIndexedIndex].Header.Height
	progress := &mockProgress{index: lastIndexedHeight}
	test := workerTest{
		blocks:   blocks,
		progress: progress,
	}

	indexerTest := newIndexTest(t, blocks, nil).
		useDefaultFirstHeight().
		setLastHeight(func(t *testing.T) (uint64, error) {
			return lastIndexedHeight, nil
		}).
		useDefaultBlockByHeight().
		initIndexer()

	blockDataCache := mempool.NewExecutionData(t)
	exeDatastore := mock.NewExecutionDataStore(t)

	blockDataCache.
		On("ByID", mocks.AnythingOfType("flow.Identifier")).
		Return(func(ID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool) {
			trie := trieUpdateFixture()
			ed := &execution_data.BlockExecutionData{
				BlockID: ID,
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{TrieUpdate: trie},
				},
			}

			// create this to capture the closure of the creation of block execution data, so we can for each returned
			// block execution data make sure the store of registers will match what the execution data returned and
			// also that the height was correct
			indexerTest.setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				var blockHeight uint64
				for _, b := range blocks {
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

	exeCache := cache.NewExecutionDataCache(exeDatastore, indexerTest.indexer.headers, nil, nil, blockDataCache)

	worker := NewExecutionStateWorker(
		zerolog.Nop(),
		test.first().Header.Height,
		testTimeout,
		indexerTest.indexer,
		exeCache,
		test.LatestHeight,
		progress,
	)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	worker.Start(signalerCtx)

	unittest.RequireComponentsReadyBefore(t, testTimeout, worker.ComponentConsumer)

	worker.OnExecutionData(nil)

	// give it a bit of time to process all the blocks
	time.Sleep(testTimeout - 50)
	cancel()
	unittest.RequireCloseBefore(t, worker.Done(), testTimeout, "timeout waiting for the consumer to be done")

	// make sure store was called correct number of times
	indexerTest.registers.AssertNumberOfCalls(t, "Store", len(blocks)-lastIndexedIndex-1)
}
