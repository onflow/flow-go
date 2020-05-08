package collectors

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
	metrics module.Metrics
}

func NewMetricsConsumer(metrics module.Metrics) *MetricsConsumer {
	return &MetricsConsumer{
		metrics: metrics,
	}
}

func (c *MetricsConsumer) OnFinalizedBlock(block *model.Block) {
	c.metrics.FinalizedBlocks(1)
}

func (c *MetricsConsumer) OnEnteringView(view uint64) {
	c.metrics.StartNewView(view)
}

func (c *MetricsConsumer) OnForkChoiceGenerated(uint64, *model.QuorumCertificate) {
	c.metrics.MadeBlockProposal()
}

func (c *MetricsConsumer) OnQcIncorporated(qc *model.QuorumCertificate) {
	c.metrics.NewestKnownQC(qc.View)
}
