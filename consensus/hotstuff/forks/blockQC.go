package forks

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// BlockQC is a Block with a QC that pointing to it, meaning a Quorum Certified Block.
// This implies Block.View == QC.View && Block.BlockID == QC.BlockID
type BlockQC struct {
	Block *model.Block
	QC    *flow.QuorumCertificate
}
