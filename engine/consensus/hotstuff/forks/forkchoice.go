package forks

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// ForkChoice determines the fork-choice.
// It is the highest level of the consensus reactor and feeds the underlying layer
// reactor.core with data
type ForkChoice interface {

	// AddQC adds a quorum certificate to Forks.
	// Might error in case the block referenced by the qc is unknown.
	AddQC(qc *types.QuorumCertificate) error

	// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
	// current view `curView`. The fork choice is a qc that should be used for
	// building the primaries block.
	//
	// Error return indicates incorrect usage. Processing a QC with view v
	// should result in the PaceMaker being in view v+1 or larger. Hence, given
	// that the current View is curView, all QCs should have view < curView
	MakeForkChoice(curView uint64) (*types.QuorumCertificate, error)
}
