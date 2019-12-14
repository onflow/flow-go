package forkchoice

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// ForkChoice determines the fork-choice.
// It is the highest level of the consensus reactor and feeds the underlying layer
// reactor.core with data
type ForkChoice interface {
	ProcessBlock(*def.Block)
	ProcessQC(*def.QuorumCertificate)

	// IsKnownBlock returns true if the consensus reactor knows the specified block
	IsKnownBlock([]byte, uint64) bool

	// IsProcessingNeeded returns true if consensus reactor should process the specified block
	IsProcessingNeeded([]byte, uint64) bool

	// GenerateForkChoice returns the QC that should be included in the next block
	GenerateForkChoice() *def.QuorumCertificate
}
