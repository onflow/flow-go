package pacemaker

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Pacemaker interface {
	OnBlockIncorporated(proposal *types.BlockProposal)
	OnQcFromVotesIncorporated(*types.QuorumCertificate)

	Run()
}
