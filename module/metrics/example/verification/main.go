package main

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// main runs a local tracer server on the machine and starts monitoring some metrics for sake of verification, which
// increases result approvals counter and checked chunks counter 100 times each
func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		tracer, err := trace.NewTracer(logger, "collection")
		if err != nil {
			panic(err)
		}
		collector := struct {
			*metrics.HotstuffCollector
			*metrics.VerificationCollector
			*metrics.NetworkCollector
		}{
			HotstuffCollector:     metrics.NewHotstuffCollector("some_chain_id"),
			VerificationCollector: metrics.NewVerificationCollector(tracer),
			NetworkCollector:      metrics.NewNetworkCollector(),
		}
		for i := 0; i < 100; i++ {
			chunkID := unittest.ChunkFixture().ID()
			collector.OnResultApproval()
			collector.OnChunkVerificationStarted(chunkID)

			// adds a synthetic 1 s delay for verification duration
			time.Sleep(1 * time.Second)
			collector.OnChunkVerificationFinished(chunkID)
			collector.OnResultApproval()

			// storage tests
			collector.OnChunkDataAdded(chunkID, 10)
			// adds a synthetic 10 ms delay between adding an removing storage
			time.Sleep(10 * time.Millisecond)
			collector.OnChunkDataRemoved(chunkID, 10)
		}
	})
}
