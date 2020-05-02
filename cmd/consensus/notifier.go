package main

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/module"
	consensusmetrics "github.com/dapperlabs/flow-go/module/metrics/consensus"
	"github.com/dapperlabs/flow-go/storage"
)

func createNotifier(log zerolog.Logger, metrics module.Metrics, guarantees storage.Guarantees, seals storage.Seals) hotstuff.Consumer {
	logConsumer := notifications.NewLogConsumer(log)
	metricsConsumer := consensusmetrics.NewMetricsConsumer(metrics, guarantees, seals)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(logConsumer)
	dis.AddConsumer(metricsConsumer)
	return dis
}
