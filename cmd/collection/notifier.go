package main

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/metrics/collectors"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/module"
)

func createNotifier(log zerolog.Logger, metrics module.Metrics) hotstuff.Consumer {
	logConsumer := notifications.NewLogConsumer(log)
	metricsConsumer := collectors.NewMetricsConsumer(metrics)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(logConsumer)
	dis.AddConsumer(metricsConsumer)
	return dis
}
