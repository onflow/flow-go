package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type BlockProducer interface {
	MakeBlockWithQC(qc *types.QuorumCertificate) *types.BlockProposal
}
