package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/dapperlabs/flow-go/model/flow"
)

type ComplianceCollector struct {
	finalizedHeight  prometheus.Gauge
	sealedHeight     prometheus.Gauge
	finalizedBlocks  prometheus.Counter
	sealedBlocks     prometheus.Counter
	finalizedPayload *prometheus.CounterVec
	sealedPayload    *prometheus.CounterVec
}

func NewComplianceCollector(registerer *Registerer) *ComplianceCollector {

	cc := &ComplianceCollector{

		finalizedHeight: registerer.RegisterNewGauge(prometheus.GaugeOpts{
			Name:      "finalized_height",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the last finalized height",
		}),

		sealedHeight: registerer.RegisterNewGauge(prometheus.GaugeOpts{
			Name:      "sealed_height",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the last sealed height",
		}),

		finalizedBlocks: registerer.RegisterNewCounter(prometheus.CounterOpts{
			Name:      "finalized_blocks_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of finalized blocks",
		}),

		sealedBlocks: registerer.RegisterNewCounter(prometheus.CounterOpts{
			Name:      "sealed_blocks_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of sealed blocks",
		}),

		finalizedPayload: registerer.RegisterNewCounterVec(prometheus.CounterOpts{
			Name:      "finalized_payload_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in finalized blocks",
		}, []string{LabelResource}),

		sealedPayload: registerer.RegisterNewCounterVec(prometheus.CounterOpts{
			Name:      "sealed_payload_total",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCompliance,
			Help:      "the number of resources in sealed blocks",
		}, []string{LabelResource}),
	}

	return cc
}

// FinalizedHeight sets the finalized height.
func (cc *ComplianceCollector) FinalizedHeight(height uint64) {
	cc.finalizedHeight.Set(float64(height))
}

// BlockFinalized reports metrics about finalized blocks.
func (cc *ComplianceCollector) BlockFinalized(block *flow.Block) {
	cc.finalizedBlocks.Inc()
	cc.finalizedPayload.With(prometheus.Labels{LabelResource: ResourceIdentity}).Add(float64(len(block.Payload.Identities)))
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
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceIdentity}).Add(float64(len(block.Payload.Identities)))
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceGuarantee}).Add(float64(len(block.Payload.Guarantees)))
	cc.sealedPayload.With(prometheus.Labels{LabelResource: ResourceSeal}).Add(float64(len(block.Payload.Seals)))
}
