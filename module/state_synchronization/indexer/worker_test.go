package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestWorker(t *testing.T) {
	blocks := blocksFixture(5)
	indexer := newIndexTest(t, blocks, nil)
	indexer.initIndexer()

	progress := storagemock.NewConsumerProgress(t)
	initHeight := blocks[0].Header.Height

	log := zerolog.Nop()

	progress.On("ProcessedIndex").Return(func() (uint64, error) {
		return blocks[0].Header.Height, nil // todo change
	})

	blockDataCache := herocache.NewBlockExecutionData(128, log, metrics.NewNoopCollector())
	exeDatastore := mock.NewExecutionDataStore(t)
	exeCache := cache.NewExecutionDataCache(exeDatastore, indexer.headers, nil, nil, blockDataCache)

	latestHeight := func() (uint64, error) {
		return blocks[len(blocks)-1].Header.Height, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	worker := NewExecutionStateWorker(log, initHeight, 2*time.Second, indexer.indexer, exeCache, latestHeight, progress)
	worker.ComponentConsumer.Start(signalerCtx)

	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, worker.ComponentConsumer)

	cancel()
	worker.OnExecutionData(nil)

	unittest.RequireCloseBefore(t, worker.Done(), 100*time.Millisecond, "timeout waiting for the consumer to be done")

}
