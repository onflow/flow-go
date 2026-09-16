package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (
	// sigTypeStaking is the label value for a bare staking-only proposer signature.
	sigTypeStaking = "staking"
	// sigTypeBeacon is the label value for a combined staking + beacon share proposer signature.
	sigTypeBeacon = "beacon"
)

// BeaconObservabilityCollector implements the Prometheus metrics exported by the access node
// beacon observability component. It tracks proposer signature types observed in finalized blocks,
// per-proposer freshness, and the random beacon threshold for the current epoch.
//
// All methods are safe for concurrent use.
type BeaconObservabilityCollector struct {
	proposerSigType    *prometheus.CounterVec
	lastProposalHeight *prometheus.GaugeVec
	beaconThreshold    prometheus.Gauge
}

var _ module.BeaconObservabilityMetrics = (*BeaconObservabilityCollector)(nil)

// NewBeaconObservabilityCollector creates a new BeaconObservabilityCollector and registers the
// metrics with the provided registerer.
//
// No errors are expected during normal operation.
func NewBeaconObservabilityCollector(registerer prometheus.Registerer) *BeaconObservabilityCollector {
	proposerSigType := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "proposer_sig_type_total",
		Namespace: namespaceNetwork,
		Subsystem: subsystemFinalized,
		Help:      "counter for proposer signature types observed in finalized blocks",
	}, []string{LabelNodeID, "type"})
	lastProposalHeight := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "last_proposal_height",
		Namespace: namespaceNetwork,
		Subsystem: subsystemFinalized,
		Help:      "gauge for the latest height at which each node proposed a finalized block",
	}, []string{LabelNodeID})
	beaconThreshold := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "dkg_beacon_threshold",
		Namespace: namespaceNetwork,
		Subsystem: subsystemFinalized,
		Help:      "gauge for the random beacon threshold (t+1) in the current epoch",
	})

	registerer.MustRegister(
		proposerSigType,
		lastProposalHeight,
		beaconThreshold,
	)

	return &BeaconObservabilityCollector{
		proposerSigType:    proposerSigType,
		lastProposalHeight: lastProposalHeight,
		beaconThreshold:    beaconThreshold,
	}
}

// NetworkFinalizedProposerSigType records a finalized block whose proposer signature was of the
// given type.
func (c *BeaconObservabilityCollector) NetworkFinalizedProposerSigType(nodeID flow.Identifier, sigType string) {
	c.proposerSigType.WithLabelValues(nodeID.String(), sigType).Inc()
}

// NetworkFinalizedLastProposalHeight updates the latest finalized block height at which the given
// node was observed as the proposer.
func (c *BeaconObservabilityCollector) NetworkFinalizedLastProposalHeight(nodeID flow.Identifier, height uint64) {
	c.lastProposalHeight.WithLabelValues(nodeID.String()).Set(float64(height))
}

// NetworkDKGBeaconThreshold updates the random beacon threshold (t+1) for the current epoch.
func (c *BeaconObservabilityCollector) NetworkDKGBeaconThreshold(threshold uint64) {
	c.beaconThreshold.Set(float64(threshold))
}

// NetworkFinalizedDeleteProposerMetrics deletes the per-proposer metric series for the given node.
// It is used at epoch boundaries to remove series for nodes that left the consensus committee.
func (c *BeaconObservabilityCollector) NetworkFinalizedDeleteProposerMetrics(nodeID flow.Identifier) {
	nodeIDString := nodeID.String()
	_ = c.proposerSigType.DeleteLabelValues(nodeIDString, sigTypeStaking)
	_ = c.proposerSigType.DeleteLabelValues(nodeIDString, sigTypeBeacon)
	_ = c.lastProposalHeight.DeleteLabelValues(nodeIDString)
}

// NetworkFinalizedInitProposerMetrics pre-initializes the per-proposer metric series for the given
// node so that silent committee members are still visible in Prometheus.
func (c *BeaconObservabilityCollector) NetworkFinalizedInitProposerMetrics(nodeID flow.Identifier) {
	nodeIDString := nodeID.String()
	c.proposerSigType.WithLabelValues(nodeIDString, sigTypeStaking).Add(0)
	c.proposerSigType.WithLabelValues(nodeIDString, sigTypeBeacon).Add(0)
	c.lastProposalHeight.WithLabelValues(nodeIDString).Set(0)
}
