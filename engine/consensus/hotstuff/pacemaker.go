package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type PaceMaker struct {
	curView   uint64
	highestQC *types.QuorumCertificate
	timeout   *time.Timer
	Timeouts  chan<- *types.Timeout
}

func (p *PaceMaker) CurView() uint64 {
	return p.curView
}

func (p *PaceMaker) UpdateValidQC(qc *types.QuorumCertificate) *types.NewViewEvent {
	panic("TODO")
}

func (p *PaceMaker) UpdateBlock(block *types.BlockProposal) *types.NewViewEvent {
	panic("TODO")
}

func (p *PaceMaker) OnLocalTimeout(timeout *types.Timeout) *types.NewViewEvent {
	panic("TODO")
}
