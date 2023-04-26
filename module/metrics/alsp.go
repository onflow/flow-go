package networking

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type AlspMetrics struct {
	reportedMisbehaviorCount prometheus.Counter
}

var _ module.AlspMetrics = (*AlspMetrics)(nil)

func (a *AlspMetrics) OnMisbehaviorReported(channel channels.Channel, misbehaviorType network.Misbehavior) {
	//TODO implement me
	panic("implement me")
}
