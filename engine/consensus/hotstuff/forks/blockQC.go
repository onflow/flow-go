package forks

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// BlockQC is a Block with a QC that pointing to it, meaning a Quorum Certified Block.
// This implies Block.View == QC.View && Block.BlockID == QC.BlockID
type BlockQC struct {
	Block *types.Block
	QC    *types.QuorumCertificate
}
