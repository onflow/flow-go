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
	latestHeight := uint64(100)

	collector := newCollector(log, latestHeight)

	ctx := context.Background()
	go func() {
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)
		collector.metricsCollectorWorker(ictx, func() {})
	}()

	wg := sync.WaitGroup{}

	wg.Add(16 * 16 * 16)
	for h := 0; h < 16; h++ {
		for b := 0; b < 16; b++ {
			for t := 0; t < 16; t++ {
				go func(h, b, t int) {
					defer wg.Done()

					block := flow.Identifier{}
					// 4 different blocks per height
					block[0] = byte(h)
					block[1] = byte(b)

					collector.Collect(block, latestHeight+1+uint64(h), TransactionExecutionMetrics{
						ExecutionTime: time.Duration(b + t),
					})
				}(h, b, t)
			}
			// wait a bit for the collector to process the data
			<-time.After(1 * time.Millisecond)
		}
	}

	wg.Wait()
	// wait a bit for the collector to process the data
	<-time.After(10 * time.Millisecond)

	for h := 0; h < 16; h++ {
		block := flow.Identifier{}
		block[0] = byte(h)

		data := collector.Pop(latestHeight+1+uint64(h), block)

		require.Len(t, data, 16)
	}

	data := collector.Pop(latestHeight, flow.ZeroID)
	require.Nil(t, data)

	data = collector.Pop(latestHeight+17, flow.ZeroID)
	require.Nil(t, data)
}
