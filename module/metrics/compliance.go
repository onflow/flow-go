package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
)

type ComplianceCollector struct {
	finalizedHeight          prometheus.Gauge
	sealedHeight             prometheus.Gauge
	finalizedBlocks          prometheus.Counter
	sealedBlocks             prometheus.Counter
	blockProposalDuration    prometheus.Counter
	finalizedPayload         *prometheus.CounterVec
	sealedPayload            *prometheus.CounterVec
	lastBlockFinalizedAt     time.Time
	finalizedBlocksPerSecond prometheus.Summary
}

func NewComplianceCollector() *ComplianceCollector {

	cc := &ComplianceCollector{

		finalizedHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "finalized_height",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the last finalized height",
		}),

		sealedHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "sealed_height",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the last sealed height",
		}),

		finalizedBlocks: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "finalized_blocks_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of finalized blocks",
		}),

		sealedBlocks: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "sealed_blocks_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of sealed blocks",
		}),

		blockProposalDuration: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "consensus_committee_block_proposal_duration_seconds_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "time spent processing block proposals in seconds",
		}),

		finalizedPayload: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "finalized_payload_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in finalized blocks",
		}, []string{LabelResource}),

		sealedPayload: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "sealed_payload_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in sealed blocks",
		}, []string{LabelResource}),

		finalizedBlocksPerSecond: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "finalized_blocks_per_second",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of finalized blocks per second/the finalized block rate",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.5:  0.05,
				0.9:  0.01,
				0.99: 0.001,
			},
			MaxAge:     10 * time.Minute,
			AgeBuckets: 5,
			BufCap:     500,
		}),
	}

	return cc
}

// FinalizedHeight sets the finalized height.
func (cc *ComplianceCollector) FinalizedHeight(height uint64) {
	cc.finalizedHeight.Set(float64(height))
}

// BlockFinalized reports metrics about finalized blocks.
func (cc *ComplianceCollector) BlockFinalized(block *flow.Block) {
	now := time.Now()
	if !cc.lastBlockFinalizedAt.IsZero() {
		cc.finalizedBlocksPerSecond.Observe(1 / now.Sub(cc.lastBlockFinalizedAt).Seconds())
	}
	cc.lastBlockFinalizedAt = now

	cc.finalizedBlocks.Inc()
	cc.finalizedPayload.With(prometheus.Labels{LabelResource: ResourceGuarantee}).Add(float64(len(block.Payload.Guarantees)))
	cc.finalizedPayload.With(prometheus.Labels{LabelResource: ResourceSeal}).Add(float64(len(block.Payload.Seals)))
}

// SealedHeight sets the finalized height.
func (cc *ComplianceCollector) SealedHeight(height uint64) {
	cc.sealedHeight.Set(float64(height))
}

// BlockSealed reports metrics about sealed blocks.
func (cc *ComplianceCollector) BlockSealed(block *flow.Block) {
	cc.sealedBlocks.Inc()
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceGuarantee}).Add(float64(len(block.Payload.Guarantees)))
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceSeal}).Add(float64(len(block.Payload.Seals)))
}

func (cc *ComplianceCollector) BlockProposalDuration(duration time.Duration) {
	cc.blockProposalDuration.Add(duration.Seconds())
}
