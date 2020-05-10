package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

type ComplianceCollector struct {
	proposedBlocks   prometheus.Counter
	finalizedBlocks  prometheus.Counter
	sealedBlocks     prometheus.Counter
	proposedPayload  *prometheus.CounterVec
	finalizedPayload *prometheus.CounterVec
	sealedPayload    *prometheus.CounterVec
}

func NewComplianceCollector() *ComplianceCollector {

	cc := &ComplianceCollector{

		proposedBlocks: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "blocks_proposed_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of proposed blocks",
		}),

		finalizedBlocks: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "blocks_finalized_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of finalized blocks",
		}),

		sealedBlocks: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "blocks_sealed_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of sealed blocks",
		}),

		proposedPayload: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "payload_proposed_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in proposed blocks",
		}, []string{LabelResource}),

		finalizedPayload: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "payload_finalized_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in finalized blocks",
		}, []string{LabelResource}),

		sealedPayload: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "payload_sealed_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in sealed blocks",
		}, []string{LabelResource}),
	}

	return cc
}

// BlockProposed reports metrics about proposed blocks.
func (cc *ComplianceCollector) BlockProposed(block *flow.Block) {
	cc.proposedBlocks.Inc()
	cc.proposedPayload.With(prometheus.Labels{LabelResource: ResourceIdentity}).Add(float64(len(block.Payload.Identities)))
	cc.proposedPayload.With(prometheus.Labels{LabelResource: ResourceGuarantee}).Add(float64(len(block.Payload.Guarantees)))
	cc.proposedPayload.With(prometheus.Labels{LabelResource: ResourceSeal}).Add(float64(len(block.Payload.Seals)))
}

// BlockFinalized reports metrics about finalized blocks.
func (cc *ComplianceCollector) BlockFinalized(block *flow.Block) {
	cc.finalizedBlocks.Inc()
	cc.finalizedPayload.With(prometheus.Labels{LabelResource: ResourceIdentity}).Add(float64(len(block.Payload.Identities)))
	cc.finalizedPayload.With(prometheus.Labels{LabelResource: ResourceGuarantee}).Add(float64(len(block.Payload.Guarantees)))
	cc.finalizedPayload.With(prometheus.Labels{LabelResource: ResourceSeal}).Add(float64(len(block.Payload.Seals)))
}

// BlockSealed reports metrics about sealed blocks.
func (cc *ComplianceCollector) BlockSealed(block *flow.Block) {
	cc.sealedBlocks.Inc()
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceIdentity}).Add(float64(len(block.Payload.Identities)))
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceGuarantee}).Add(float64(len(block.Payload.Guarantees)))
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceSeal}).Add(float64(len(block.Payload.Seals)))
}
