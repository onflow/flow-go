package main

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/module"
	metricsconsumer "github.com/onflow/flow-go/module/metrics/hotstuff"
)

// createLogger creates logger which reports chain ID on every log message.
func createLogger(log zerolog.Logger, chainID flow.ChainID) zerolog.Logger {
	return log.With().Str("chain", chainID.String()).Logger()
}

// createNotifier creates a pubsub distributor and connects it to consensus consumers.
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
