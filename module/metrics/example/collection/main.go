package main

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		tracer, err := trace.NewTracer(logger, "collection")
		if err != nil {
			panic(err)
		}
		collector := struct {
			*metrics.HotstuffCollector
			*metrics.CollectionCollector
			*metrics.NetworkCollector
		}{
			HotstuffCollector:   metrics.NewHotstuffCollector("some_chain_id"),
			CollectionCollector: metrics.NewCollectionCollector(tracer),
			NetworkCollector:    metrics.NewNetworkCollector(),
		}

		for i := 0; i < 100; i++ {
			collector.TransactionIngested(unittest.IdentifierFixture())
			collector.HotStuffBusyDuration(10, metrics.HotstuffEventTypeTimeout)
			collector.HotStuffWaitDuration(10, metrics.HotstuffEventTypeTimeout)
			collector.HotStuffIdleDuration(10)
			collector.SetCurView(uint64(i))
			collector.SetQCView(uint64(i))

			collector.NetworkMessageSent(rand.Intn(1000), engine.ChannelName(engine.CollectionProvider))
			collector.NetworkMessageSent(rand.Intn(1000), engine.ChannelName(engine.CollectionIngest))

			collector.NetworkMessageReceived(rand.Intn(1000), engine.ChannelName(engine.CollectionProvider))
			collector.NetworkMessageReceived(rand.Intn(1000), engine.ChannelName(engine.CollectionIngest))

			time.Sleep(1 * time.Second)
		}
	})
}
