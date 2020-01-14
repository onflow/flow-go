package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type PaceMaker interface {
	CurView() uint64
	TimeoutChannel() <-chan time.Time
	UpdateQC(qc *types.QuorumCertificate) (*types.NewViewEvent, bool)
	UpdateBlock(block *types.BlockProposal) (*types.NewViewEvent, bool)
	OnTimeout() (*types.NewViewEvent, bool)
}
