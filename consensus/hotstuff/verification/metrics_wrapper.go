package verification

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// SignerMetricsWrapper implements the hotstuff.SignerVerifier interface.
// It wraps a hotstuff.SignerVerifier instance and measures the time which the HotStuff's core logic
// spends in the hotstuff.SignerVerifier component, i.e. the with crypto-related operations.
// The measured time durations are reported as values for the
// SignerProcessingDuration metric.
type SignerMetricsWrapper struct {
	signer  hotstuff.SignerVerifier
	metrics module.HotstuffMetrics
}

func NewMetricsWrapper(signer hotstuff.SignerVerifier, metrics module.HotstuffMetrics) *SignerMetricsWrapper {
	return &SignerMetricsWrapper{
		signer:  signer,
		metrics: metrics,
	}
}

func (w SignerMetricsWrapper) VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	processStart := time.Now()
	valid, err := w.signer.VerifyVote(voterID, sigData, block)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return valid, err
}

func (w SignerMetricsWrapper) VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	processStart := time.Now()
	valid, err := w.signer.VerifyQC(voterIDs, sigData, block)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return valid, err
}

func (w SignerMetricsWrapper) CreateProposal(block *model.Block) (*model.Proposal, error) {
	processStart := time.Now()
	proposal, err := w.signer.CreateProposal(block)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return proposal, err
}

func (w SignerMetricsWrapper) CreateVote(block *model.Block) (*model.Vote, error) {
	processStart := time.Now()
	vote, err := w.signer.CreateVote(block)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return vote, err
}

func (w SignerMetricsWrapper) CreateQC(votes []*model.Vote) (*flow.QuorumCertificate, error) {
	processStart := time.Now()
	qc, err := w.signer.CreateQC(votes)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return qc, err
}
