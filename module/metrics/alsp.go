package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/module"
)

// AlspMetrics is a struct that contains all the metrics related to the ALSP module.
// It implements the AlspMetrics interface.
// AlspMetrics encapsulates the metrics collectors for the Application Layer Spam Prevention (ALSP) module, which
// is part of the networking layer. ALSP is responsible to prevent spam attacks on the application layer messages that
// appear to be valid for the networking layer but carry on a malicious intent on the application layer (i.e., Flow protocols).
type AlspMetrics struct {
	reportedMisbehaviorCount *prometheus.CounterVec
}

var _ module.AlspMetrics = (*AlspMetrics)(nil)

// NewAlspMetrics creates a new AlspMetrics struct. It initializes the metrics collectors for the ALSP module.
// Returns:
// - a pointer to the AlspMetrics struct.
func NewAlspMetrics() *AlspMetrics {
	alsp := &AlspMetrics{}

	alsp.reportedMisbehaviorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemAlsp,
			Name:      "reported_misbehavior_total",
			Help:      "number of reported spamming misbehavior received by alsp",
		}, []string{LabelChannel, LabelMisbehavior},
	)

	return alsp
}

// OnMisbehaviorReported is called when a misbehavior is reported by the application layer to ALSP.
// An engine detecting a spamming-related misbehavior reports it to the ALSP module. It increases
// the counter vector of reported misbehavior.
// Args:
// - channel: the channel on which the misbehavior was reported
// - misbehaviorType: the type of misbehavior reported
func (a *AlspMetrics) OnMisbehaviorReported(channel string, misbehaviorType string) {
	a.reportedMisbehaviorCount.With(prometheus.Labels{
		LabelChannel:     channel,
		LabelMisbehavior: misbehaviorType,
	}).Inc()
}
