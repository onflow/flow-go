package forkchoice

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/juju/loggo"
)

var ForkChoiceLogger loggo.Logger

// ForkChoice determines the fork-choice.
// It is the highest level of the consensus reactor and feeds the underlying layer
// reactor.core with data
type ForkChoice interface {
	ProcessBlock(proposal *types.BlockProposal)
	ProcessQcFromVotes(*types.QuorumCertificate)

	// IsKnownBlock returns true if the consensus reactor knows the specified block
	IsKnownBlock([]byte, uint64) bool

	// IsProcessingNeeded returns true if consensus reactor should process the specified block
	IsProcessingNeeded([]byte, uint64) bool

	// OnForkChoiceTrigger prompts the ForkChoice to generate a fork choice
	// and publish it via emitting an OnForkChoiceGenerated event
	OnForkChoiceTrigger(viewNumber uint64)
}
