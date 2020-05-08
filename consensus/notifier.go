package consensus

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/module"
	consensusmetrics "github.com/dapperlabs/flow-go/module/metrics/consensus"
	"github.com/dapperlabs/flow-go/storage"
)

func CreateNotifier(log zerolog.Logger, metrics module.Metrics, payloads storage.Payloads) hotstuff.Consumer {
	logConsumer := notifications.NewLogConsumer(log)
	metricsConsumer := consensusmetrics.NewMetricsConsumer(metrics, payloads)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(logConsumer)
	dis.AddConsumer(metricsConsumer)
	return dis
}
