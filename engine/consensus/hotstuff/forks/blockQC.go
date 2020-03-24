package forks

import (
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// BlockQC is a Block with a QC that pointing to it, meaning a Quorum Certified Block.
// This implies Block.View == QC.View && Block.BlockID == QC.BlockID
type BlockQC struct {
	Block *hotstuff.Block
	QC    *hotstuff.QuorumCertificate
}
