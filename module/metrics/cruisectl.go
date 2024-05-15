package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// CruiseCtlMetrics captures metrics about the Block Rate Controller, which adjusts
// the proposal duration to attain a target epoch switchover time.
type CruiseCtlMetrics struct {
	proportionalErr          prometheus.Gauge
	integralErr              prometheus.Gauge
	derivativeErr            prometheus.Gauge
	targetProposalDur        prometheus.Gauge
	controllerOutput         prometheus.Gauge
	proposalPublicationDelay prometheus.Gauge
}

var _ module.CruiseCtlMetrics = (*CruiseCtlMetrics)(nil)

func NewCruiseCtlMetrics() *CruiseCtlMetrics {
	return &CruiseCtlMetrics{
		proportionalErr: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "proportional_err_s",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCruiseCtl,
			Help:      "The current proportional error measured by the controller",
		}),
		integralErr: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "integral_err_s",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCruiseCtl,
			Help:      "The current integral error measured by the controller",
		}),
		derivativeErr: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "derivative_err_per_s",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCruiseCtl,
			Help:      "The current derivative error measured by the controller",
		}),
		targetProposalDur: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "target_proposal_dur_s",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCruiseCtl,
			Help:      "The current target duration from parent to child proposal [seconds]",
		}),
		controllerOutput: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "controller_output_s",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCruiseCtl,
			Help:      "The most recent output of the controller [seconds]; the adjustment to subtract from the baseline proposal duration",
		}),
		proposalPublicationDelay: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "proposal_publication_delay_s",
			Namespace: namespaceConsensus,
			Subsystem: subsystemCruiseCtl,
			Help:      "Effective delay the controller imposes on publishing the node's own proposals [seconds]; with all limits of authority",
		}),
	}
}

func (c *CruiseCtlMetrics) PIDError(p, i, d float64) {
	c.proportionalErr.Set(p)
	c.integralErr.Set(i)
	c.derivativeErr.Set(d)
}

func (c *CruiseCtlMetrics) TargetProposalDuration(duration time.Duration) {
	c.targetProposalDur.Set(duration.Seconds())
}

func (c *CruiseCtlMetrics) ControllerOutput(duration time.Duration) {
	c.controllerOutput.Set(duration.Seconds())
}

func (c *CruiseCtlMetrics) ProposalPublicationDelay(duration time.Duration) {
	c.proposalPublicationDelay.Set(duration.Seconds())
}
