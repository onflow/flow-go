package main

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/module"
	metricsconsumer "github.com/dapperlabs/flow-go/module/metrics/hotstuff"
)

func createNotifier(log zerolog.Logger, metrics module.HotstuffMetrics, chain flow.ChainID) hotstuff.Consumer {
	telemetryConsumer := notifications.NewTelemetryConsumer(log, chain)
	metricsConsumer := metricsconsumer.NewMetricsConsumer(metrics)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(telemetryConsumer)
	dis.AddConsumer(metricsConsumer)
	return dis
}
