package main

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/module"
	metricsconsumer "github.com/onflow/flow-go/module/metrics/hotstuff"
)

func createNotifier(log zerolog.Logger, metrics module.HotstuffMetrics) *pubsub.Distributor {
	telemetryConsumer := notifications.NewTelemetryConsumer(log)
	metricsConsumer := metricsconsumer.NewMetricsConsumer(metrics)
	logsConsumer := notifications.NewLogConsumer(log)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(telemetryConsumer)
	dis.AddConsumer(metricsConsumer)
	dis.AddConsumer(logsConsumer)
	return dis
}
