package main

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/example"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/queue"
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
			*metrics.CollectionCollector
			*metrics.NetworkCollector
		}{
			HotstuffCollector:   metrics.NewHotstuffCollector("some_chain_id"),
			CollectionCollector: metrics.NewCollectionCollector(tracer),
			NetworkCollector:    metrics.NewNetworkCollector(unittest.Logger()),
		}

		topic1 := channels.TestNetworkChannel.String()
		topic2 := channels.TestMetricsChannel.String()
		protocol1 := message.ProtocolTypeUnicast.String()
		protocol2 := message.ProtocolTypePubSub.String()
		message1 := "CollectionRequest"
		message2 := "ClusterBlockProposal"

		for i := 0; i < 100; i++ {
			collector.TransactionIngested(unittest.IdentifierFixture())
			collector.HotStuffBusyDuration(10, metrics.HotstuffEventTypeLocalTimeout)
			collector.HotStuffWaitDuration(10, metrics.HotstuffEventTypeLocalTimeout)
			collector.HotStuffIdleDuration(10)
			collector.SetCurView(uint64(i))
			collector.SetQCView(uint64(i))

			collector.OutboundMessageSent(rand.Intn(1000), topic1, protocol1, message1)
			collector.OutboundMessageSent(rand.Intn(1000), topic2, protocol2, message2)

			collector.InboundMessageReceived(rand.Intn(1000), topic1, protocol1, message1)
			collector.InboundMessageReceived(rand.Intn(1000), topic2, protocol2, message2)

			priority1 := rand.Intn(int(queue.HighPriority-queue.LowPriority+1)) + int(queue.LowPriority)
			collector.MessageRemoved(priority1)
			collector.QueueDuration(time.Millisecond*time.Duration(rand.Intn(1000)), priority1)

			priority2 := rand.Intn(int(queue.HighPriority-queue.LowPriority+1)) + int(queue.LowPriority)
			collector.MessageAdded(priority2)
			time.Sleep(1 * time.Second)
		}
	})
}
