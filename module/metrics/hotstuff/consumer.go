package consensus

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// MetricsConsumer is a consumer that subscribes to hotstuff events and
// collects metrics data when certain events trigger.
// It depends on Metrics module to report metrics data.
type MetricsConsumer struct {
	// inherit from noop consumer in order to satisfy the full interface
	notifications.NoopConsumer
	metrics module.HotstuffMetrics
}

var _ hotstuff.Consumer = (*MetricsConsumer)(nil)

func NewMetricsConsumer(metrics module.HotstuffMetrics) *MetricsConsumer {
	return &MetricsConsumer{
		metrics: metrics,
	}
}

func (c *MetricsConsumer) OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64) {
	c.metrics.SetCurView(newView)
	c.metrics.SetQCView(qc.View)
	c.metrics.CountSkipped()
}

func (c *MetricsConsumer) OnTcTriggeredViewChange(tc *flow.TimeoutCertificate, newView uint64) {
	c.metrics.SetCurView(newView)
	c.metrics.SetTCView(tc.View)
	c.metrics.CountTimeout()
}

func (c *MetricsConsumer) OnStartingTimeout(info model.TimerInfo) {
	c.metrics.SetTimeout(info.Duration)
}
