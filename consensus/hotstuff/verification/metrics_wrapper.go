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

var _ hotstuff.Signer = (*SignerMetricsWrapper)(nil)

func NewMetricsWrapper(signer hotstuff.Signer, metrics module.HotstuffMetrics) *SignerMetricsWrapper {
	return &SignerMetricsWrapper{
		signer:  signer,
		metrics: metrics,
	}
}

func (w SignerMetricsWrapper) CreateVote(block *model.Block) (*model.Vote, error) {
	processStart := time.Now()
	vote, err := w.signer.CreateVote(block)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return vote, err
}

func (w SignerMetricsWrapper) CreateTimeout(curView uint64,
	newestQC *flow.QuorumCertificate,
	lastViewTC *flow.TimeoutCertificate) (*model.TimeoutObject, error) {
	processStart := time.Now()
	timeout, err := w.signer.CreateTimeout(curView, newestQC, lastViewTC)
	w.metrics.SignerProcessingDuration(time.Since(processStart))
	return timeout, err
}
