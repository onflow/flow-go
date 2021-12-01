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
// spends in the hotstuff.Signer component, i.e. the with crypto-related operations.
// The measured time durations are reported as values for the
// SignerProcessingDuration metric.
// TODO: to be moved to consensus/hotstuff/signature
type SignerMetricsWrapper struct {
	signer  hotstuff.Signer
	metrics module.HotstuffMetrics
}

func NewMetricsWrapper(signer hotstuff.Signer, metrics module.HotstuffMetrics) *SignerMetricsWrapper {
	return &SignerMetricsWrapper{
		signer:  signer,
		metrics: metrics,
	}
}

// TODO: to be moved to VerifierMetricsWrapper
// func (w SignerMetricsWrapper) VerifyVote(voter *flow.Identity, sigData []byte, block *model.Block) (bool, error) {
// 	processStart := time.Now()
// 	valid, err := w.signer.VerifyVote(voter, sigData, block)
// 	w.metrics.SignerProcessingDuration(time.Since(processStart))
// 	return valid, err
// }
//
// func (w SignerMetricsWrapper) VerifyQC(signers flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {
// 	processStart := time.Now()
// 	valid, err := w.signer.VerifyQC(signers, sigData, block)
// 	w.metrics.SignerProcessingDuration(time.Since(processStart))
// 	return valid, err
// }

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
