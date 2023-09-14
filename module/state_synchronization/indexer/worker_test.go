package indexer

import (
	"context"
	"fmt"
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
	fmt.Println("index", index)
	w.index = index
	return nil
}

func (w *mockProgress) InitProcessedIndex(index uint64) error {
	w.index = index
	return nil
}

func TestWorker(t *testing.T) {

	blocks := blocksFixture(10)
	fmt.Println("blocks range", blocks[0].Header.Height, blocks[len(blocks)-1].Header.Height)

	lastIndexedHeight := blocks[5].Header.Height
	fmt.Println("already indexed range", blocks[0].Header.Height, lastIndexedHeight)
	progress := &mockProgress{index: lastIndexedHeight}
	test := workerTest{
		blocks:   blocks,
		progress: progress,
	}

	indexerTest := newIndexTest(t, blocks, nil)
	indexerTest.useDefaultFirstHeight()
	indexerTest.setLastHeight(func(t *testing.T) (uint64, error) {
		return lastIndexedHeight, nil
	})
	indexerTest.initIndexer()

	blockDataCache := mempool.NewExecutionData(t)
	exeDatastore := mock.NewExecutionDataStore(t)

	indexerTest.headers.
		On("BlockIDByHeight", mocks.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			for _, b := range blocks {
				if b.Header.Height == height {
					return b.ID(), nil
				}
			}
			return flow.Identifier{}, fmt.Errorf("not found")
		})

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

			indexerTest.setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				var blockHeight uint64
				for _, b := range blocks {
					if b.ID() == ID {
						blockHeight = b.Header.Height
					}
				}
				assert.Equal(t, blockHeight, height)
				for j, e := range entries {
					assert.True(t, trie.Payloads[j].Value().Equals(e.Value))
				}
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

	ctx, _ := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	worker.Start(signalerCtx)

	unittest.RequireComponentsReadyBefore(t, testTimeout*1000, worker.ComponentConsumer)

	worker.OnExecutionData(nil)

	//cancel()
	unittest.RequireCloseBefore(t, worker.Done(), testTimeout*1000, "timeout waiting for the consumer to be done")

}
