package main

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/utils/unittest"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
)

func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		collector, err := metrics.NewClusterCollector(logger, "someChainID")
		if err != nil {
			panic(err)
		}

		for i := 0; i < 100; i++ {
			collector.TransactionReceived(unittest.IdentifierFixture())
			collector.PendingClusterBlocks(uint(rand.Intn(20) + 10))
			collector.HotStuffBusyDuration(10, metrics.HotstuffEventTypeTimeout)
			collector.HotStuffWaitDuration(10, metrics.HotstuffEventTypeTimeout)
			collector.HotStuffIdleDuration(10)
			collector.StartNewView(uint64(i))
			collector.NewestKnownQC(uint64(i))

			collector.NetworkMessageSent(rand.Intn(1000), engine.ChannelName(engine.CollectionProvider))
			collector.NetworkMessageSent(rand.Intn(1000), engine.ChannelName(engine.CollectionIngest))

			collector.NetworkMessageReceived(rand.Intn(1000), engine.ChannelName(engine.CollectionProvider))
			collector.NetworkMessageReceived(rand.Intn(1000), engine.ChannelName(engine.CollectionIngest))

			time.Sleep(1 * time.Second)
		}
	})
}
