package validator

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// ValidatorMetricsWrapper implements the hotstuff.Validator interface.
// It wraps a hotstuff.Validator instance and measures the time which the HotStuff's core logic
// spends in the hotstuff.Validator component, i.e. the with verifying higher-level consensus
// messages. The measured time durations are reported as values for the
// ValidatorProcessingDuration metric.
type ValidatorMetricsWrapper struct {
	validator hotstuff.Validator
	metrics   module.HotstuffMetrics
}

func NewMetricsWrapper(validator hotstuff.Validator, metrics module.HotstuffMetrics) *ValidatorMetricsWrapper {
	return &ValidatorMetricsWrapper{
		validator: validator,
		metrics:   metrics,
	}
}

func (w ValidatorMetricsWrapper) ValidateQC(qc *flow.QuorumCertificate) error {
	processStart := time.Now()
	err := w.validator.ValidateQC(qc)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return err
}

func (w ValidatorMetricsWrapper) ValidateTC(tc *flow.TimeoutCertificate) error {
	processStart := time.Now()
	err := w.validator.ValidateTC(tc)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return err
}

func (w ValidatorMetricsWrapper) ValidateProposal(proposal *model.Proposal) error {
	processStart := time.Now()
	err := w.validator.ValidateProposal(proposal)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return err
}

func (w ValidatorMetricsWrapper) ValidateVote(vote *model.Vote) (*flow.IdentitySkeleton, error) {
	processStart := time.Now()
	identity, err := w.validator.ValidateVote(vote)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return identity, err
}
