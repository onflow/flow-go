package blockProducer

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// BlockProducer needs to know when a block needs to be produced (OnForkChoiceGenerated),
// the current view (OnEnteringView), and the payload from the mempool
type BlockProducer struct {
	// Mempool interface here
}

// OnForkChoiceGenerated listens to OnForkChoiceGenerated events and builds
// a block with qc
func (bp *BlockProducer) OnForkChoiceGenerated(qc *def.QuorumCertificate) {
	panic("implement me!")
}

// OnEnteringView listens to OnEnteringView events and updates BlockProducer's
// current view for when it needs to build a block
func (v *BlockProducer) OnEnteringView(view uint64) {
	panic("Implement me")
}
