package main

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/example"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// main runs a local tracer server on the machine and starts monitoring some metrics for sake of execution, which
// increases result approvals counter and checked chunks counter 100 times each
func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		tracer, err := trace.NewTracer(logger, "collection", "test", trace.SensitivityCaptureAll)
		if err != nil {
			panic(err)
		}
		collector := struct {
			*metrics.HotstuffCollector
			*metrics.ExecutionCollector
			*metrics.NetworkCollector
		}{
			HotstuffCollector:  metrics.NewHotstuffCollector("some_chain_id"),
			ExecutionCollector: metrics.NewExecutionCollector(tracer),
			NetworkCollector:   metrics.NewNetworkCollector(unittest.Logger()),
		}
		diskTotal := rand.Int63n(1024 * 1024 * 1024)
		for i := 0; i < 1000; i++ {
			blockID := unittest.BlockFixture().ID()
			collector.StartBlockReceivedToExecuted(blockID)

			duration := time.Duration(rand.Int31n(2000)) * time.Millisecond
			// adds a random delay for execution duration, between 0 and 2 seconds
			time.Sleep(duration)

			collector.ExecutionBlockExecuted(
				duration,
				module.BlockExecutionResultStats{
					CollectionExecutionResultStats: module.CollectionExecutionResultStats{
						ExecutionResultStats: module.ExecutionResultStats{
							EventSize:       100,
							EventCounts:     2,
							MemoryUsed:      uint64(rand.Int63n(1e6)),
							ComputationUsed: uint64(rand.Int63n(1e6)),
						},
						NumberOfTransactions: 1,
					},
					NumberOfCollections: 1,
				})

			diskIncrease := rand.Int63n(1024 * 1024)
			diskTotal += diskIncrease
			collector.ExecutionStateStorageDiskTotal(diskTotal)
			collector.ExecutionStorageStateCommitment(diskIncrease)

			collector.FinishBlockReceivedToExecuted(blockID)
		}
	})
}
