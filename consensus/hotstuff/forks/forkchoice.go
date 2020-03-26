package forks

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// ForkChoice determines the fork-choice.
// ForkChoice directly interfaces with the Finalizer to query required information
// (such as finalized View, stored block, etc).
type ForkChoice interface {

	// AddQC adds a Quorum Certificate to Forks;
	// Errors in case the block referenced by the qc is unknown.
	AddQC(qc *model.QuorumCertificate) error

	// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
	// current view `curView`. The fork choice is a qc that should be used for
	// building the primaries block.
	//
	// Error return indicates incorrect usage. Processing a QC with view v
	// should result in the PaceMaker being in view v+1 or larger. Hence, given
	// that the current View is curView, all QCs should have view < curView
	MakeForkChoice(curView uint64) (*model.Block, *model.QuorumCertificate, error)
}
