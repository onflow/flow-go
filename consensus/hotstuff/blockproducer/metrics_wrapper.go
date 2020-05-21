package blockproducer

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/module"
)

// BlockProducerMetricsWrapper measures the time which the HotStuff's core logic
// spends in the hotstuff.BlockProducer component, i.e. the with generating block payloads
type BlockProducerMetricsWrapper struct {
	blockProducer hotstuff.BlockProducer
	metrics       module.HotstuffMetrics
}

func (w BlockProducerMetricsWrapper) MakeBlockProposal(qc *model.QuorumCertificate, view uint64) (*model.Proposal, error) {
	processStart := time.Now()
	proposal, err := w.blockProducer.MakeBlockProposal(qc, view)
	w.metrics.PayloadProductionDuration(time.Since(processStart))
	return proposal, err
}
