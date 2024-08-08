package metrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

func Test_CollectorPopOnEmpty(t *testing.T) {
	t.Parallel()

	log := zerolog.New(zerolog.NewTestWriter(t))
	latestHeight := uint64(100)

	collector := newCollector(log, latestHeight)

	data := collector.Pop(latestHeight, flow.ZeroID)
	require.Nil(t, data)
}

func Test_CollectorCollection(t *testing.T) {
	log := zerolog.New(zerolog.NewTestWriter(t))
	startHeight := uint64(100)

	collector := newCollector(log, startHeight)

	ctx := context.Background()
	go func() {
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)
		collector.metricsCollectorWorker(ictx, func() {})
	}()

	wg := sync.WaitGroup{}

	wg.Add(16 * 16 * 16)
	for height := 0; height < 16; height++ {
		// for each height we add multiple blocks. Only one block will be popped per height
		for block := 0; block < 16; block++ {
			// for each block we add multiple transactions
			for transaction := 0; transaction < 16; transaction++ {
				go func(h, b, t int) {
					defer wg.Done()

					block := flow.Identifier{}
					block[0] = byte(h)
					block[1] = byte(b)

					collector.Collect(
						block,
						startHeight+1+uint64(h),
						TransactionExecutionMetrics{
							ExecutionTime: time.Duration(b + t),
						},
					)
				}(height, block, transaction)
			}
			// wait a bit for the collector to process the data
			<-time.After(1 * time.Millisecond)
		}
	}

	wg.Wait()
	// wait a bit for the collector to process the data
	<-time.After(10 * time.Millisecond)

	// there should be no data at the start height
	data := collector.Pop(startHeight, flow.ZeroID)
	require.Nil(t, data)

	for height := 0; height < 16; height++ {
		block := flow.Identifier{}
		block[0] = byte(height)
		// always pop the first block each height
		block[1] = byte(0)

		data := collector.Pop(startHeight+1+uint64(height), block)

		require.Len(t, data, 16)
	}

	block := flow.Identifier{}
	block[0] = byte(15)
	block[1] = byte(1)
	// height 16 was already popped so there should be no more data for any blocks
	data = collector.Pop(startHeight+16, block)
	require.Nil(t, data)

	// there should be no data past the last collected height
	data = collector.Pop(startHeight+17, flow.ZeroID)
	require.Nil(t, data)
}
