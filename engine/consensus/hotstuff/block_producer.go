package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type BlockProposalProducer interface {
	MakeBlockProposalWithQC(qc *types.QuorumCertificate) *types.BlockProposal
}
