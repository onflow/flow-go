package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type BlockProposalProducer interface {
	MakeBlockProposalWithQC(view uint64, qc *types.QuorumCertificate) *types.BlockProposal
}
