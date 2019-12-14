package blockProducer

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// BlockProducer needs to know when a block needs to be produced (OnForkChoiceGenerated),
// the current view (OnEnteringView), and the payload from the mempool
type BlockProducer struct {
	// Mempool interface here
}

func (bp *BlockProducer) OnForkChoiceGenerated(qc *def.QuorumCertificate) {
	panic("implement me!")
}

func (v *BlockProducer) OnEnteringView(view uint64) {
	panic("Implement me")
}
