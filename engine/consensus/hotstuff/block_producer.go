package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type BlockProposalProducer interface {
	// MakeBlockProposal will build a proposal for the given view with the given QC
	MakeBlockProposal(view uint64, qc *types.QuorumCertificate) *types.BlockProposal
}
