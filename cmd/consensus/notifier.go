package main

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/module"
	metricsconsumer "github.com/onflow/flow-go/module/metrics/hotstuff"
	"github.com/onflow/flow-go/storage"
)

func createNotifier(log zerolog.Logger, metrics module.HotstuffMetrics, tracer module.Tracer, index storage.Index, chain flow.ChainID,
) hotstuff.Consumer {
	telemetryConsumer := notifications.NewTelemetryConsumer(log, chain)
	tracingConsumer := notifications.NewConsensusTracingConsumer(log, tracer, index)
	metricsConsumer := metricsconsumer.NewMetricsConsumer(metrics)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(telemetryConsumer)
	dis.AddConsumer(tracingConsumer)
	dis.AddConsumer(metricsConsumer)
	return dis
}
