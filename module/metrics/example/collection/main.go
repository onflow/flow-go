package main

import (
	"encoding/binary"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
)

func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		metrics, err := metrics.NewCollector(logger)
		if err != nil {
			panic(err)
		}

		for i := 0; i < 100; i++ {
			metrics.CollectionsPerBlock(1)
			metrics.CollectionsInFinalizedBlock(3)
			metrics.SealsInFinalizedBlock(3)
			metrics.HotStuffBusyDuration(10)
			metrics.HotStuffIdleDuration(10)
			metrics.StartNewView(uint64(i))
			metrics.NewestKnownQC(uint64(i))

			entityID := make([]byte, 32)
			binary.LittleEndian.PutUint32(entityID, uint32(i/6))

			entity2ID := make([]byte, 32)
			binary.LittleEndian.PutUint32(entity2ID, uint32(i/6+100000))
			if i%6 == 0 {
				metrics.StartCollectionToFinalized(flow.HashToID(entityID))
			} else if i%6 == 3 {
				metrics.FinishCollectionToFinalized(flow.HashToID(entityID))
			}

			if i%5 == 0 {
				metrics.StartBlockToSeal(flow.HashToID(entityID))
			} else if i%6 == 3 {
				metrics.FinishBlockToSeal(flow.HashToID(entityID))
			}

			time.Sleep(1 * time.Second)
		}
	})
}
