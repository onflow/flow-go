package main

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
)

func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		collector, err := metrics.NewCollector(logger)
		if err != nil {
			panic(err)
		}

		topic1, err := engine.ChannelName(engine.CollectionProvider)
		if err != nil {
			panic(err)
		}
		topic2, err := engine.ChannelName(engine.CollectionIngest)
		if err != nil {
			panic(err)
		}

		for i := 0; i < 100; i++ {
			collector.CollectionsPerBlock(1)
			collector.CollectionsInFinalizedBlock(3)
			collector.SealsInFinalizedBlock(3)
			collector.HotStuffBusyDuration(10, metrics.HotstuffEventTypeTimeout)
			collector.HotStuffWaitDuration(10, metrics.HotstuffEventTypeTimeout)
			collector.HotStuffIdleDuration(10)
			collector.StartNewView(uint64(i))
			collector.NewestKnownQC(uint64(i))

			entityID := make([]byte, 32)
			binary.LittleEndian.PutUint32(entityID, uint32(i/6))

			entity2ID := make([]byte, 32)
			binary.LittleEndian.PutUint32(entity2ID, uint32(i/6+100000))
			if i%6 == 0 {
				collector.StartCollectionToFinalized(flow.HashToID(entityID))
			} else if i%6 == 3 {
				collector.FinishCollectionToFinalized(flow.HashToID(entityID))
			}

			if i%5 == 0 {
				collector.StartBlockToSeal(flow.HashToID(entityID))
			} else if i%6 == 3 {
				collector.FinishBlockToSeal(flow.HashToID(entityID))
			}

			collector.NetworkMessageSent(rand.Intn(1000), topic1)
			collector.NetworkMessageSent(rand.Intn(1000), topic2)

			collector.NetworkMessageReceived(rand.Intn(1000), topic1)
			collector.NetworkMessageReceived(rand.Intn(1000), topic2)

			time.Sleep(1 * time.Second)
		}
	})
}
