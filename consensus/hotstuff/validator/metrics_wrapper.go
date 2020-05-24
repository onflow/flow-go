package validator

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
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

func (w ValidatorMetricsWrapper) ValidateQC(qc *model.QuorumCertificate, block *model.Block) error {
	processStart := time.Now()
	err := w.validator.ValidateQC(qc, block)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return err
}

func (w ValidatorMetricsWrapper) ValidateProposal(proposal *model.Proposal) error {
	processStart := time.Now()
	err := w.validator.ValidateProposal(proposal)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return err
}

func (w ValidatorMetricsWrapper) ValidateVote(vote *model.Vote, block *model.Block) (*flow.Identity, error) {
	processStart := time.Now()
	identity, err := w.validator.ValidateVote(vote, block)
	w.metrics.ValidatorProcessingDuration(time.Since(processStart))
	return identity, err
}
