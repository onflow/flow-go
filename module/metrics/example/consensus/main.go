package main

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/example"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/unittest"
)

func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		tracer, err := trace.NewTracer(logger, "collection", "test", trace.SensitivityCaptureAll)
		if err != nil {
			panic(err)
		}
		collector := struct {
			*metrics.HotstuffCollector
			*metrics.ConsensusCollector
			*metrics.NetworkCollector
			*metrics.ComplianceCollector
			*metrics.MempoolCollector
		}{
			HotstuffCollector:   metrics.NewHotstuffCollector("some_chain_id"),
			ConsensusCollector:  metrics.NewConsensusCollector(tracer, prometheus.DefaultRegisterer),
			NetworkCollector:    metrics.NewNetworkCollector(unittest.Logger()),
			ComplianceCollector: metrics.NewComplianceCollector(),
			MempoolCollector:    metrics.NewMempoolCollector(5 * time.Second),
		}

		for i := 0; i < 100; i++ {
			block := unittest.BlockFixture()
			collector.MempoolEntries(metrics.ResourceGuarantee, 22)
			collector.BlockFinalized(&block)
			collector.HotStuffBusyDuration(10, metrics.HotstuffEventTypeLocalTimeout)
			collector.HotStuffWaitDuration(10, metrics.HotstuffEventTypeLocalTimeout)
			collector.HotStuffIdleDuration(10)
			collector.SetCurView(uint64(i))
			collector.SetQCView(uint64(i))

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

			collProvider := channels.TestNetworkChannel.String()
			collIngest := channels.TestMetricsChannel.String()
			protocol1 := message.ProtocolTypeUnicast.String()
			protocol2 := message.ProtocolTypePubSub.String()
			message1 := "CollectionRequest"
			message2 := "ClusterBlockProposal"

			collector.OutboundMessageSent(rand.Intn(1000), collProvider, protocol1, message1)
			collector.OutboundMessageSent(rand.Intn(1000), collIngest, protocol2, message2)

			collector.InboundMessageReceived(rand.Intn(1000), collProvider, protocol1, message1)
			collector.InboundMessageReceived(rand.Intn(1000), collIngest, protocol2, message2)

			time.Sleep(1 * time.Second)
		}
	})
}
