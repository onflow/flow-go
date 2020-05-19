package consensus

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/module"
)

// MetricsConsumer is a consumer that subscribes to hotstuff events and
// collects metrics data when certain events trigger.
// It depends on Metrics module to report metrics data.
type MetricsConsumer struct {
	// inherit from noop consumer in order to satisfy the full interface
	notifications.NoopConsumer
	metrics module.HotstuffMetrics
}

func NewMetricsConsumer(metrics module.HotstuffMetrics) *MetricsConsumer {
	return &MetricsConsumer{
		metrics: metrics,
	}
}

func (c *MetricsConsumer) OnEnteringView(view uint64) {
	c.metrics.SetCurView(view)
}

func (c *MetricsConsumer) OnQcIncorporated(qc *model.QuorumCertificate) {
	c.metrics.SetQCView(qc.View)
}

func (c *MetricsConsumer) OnSkippedAhead(view uint64) {
	c.metrics.CountSkipped()
}

func (c *MetricsConsumer) OnReachedTimeout(info *model.TimerInfo) {
	c.metrics.CountTimeout()
}

func (c *MetricsConsumer) OnStartingTimeout(info *model.TimerInfo) {
	c.metrics.SetTimeout(info.Duration)
}
