package collection

import (
	"github.com/onflow/flow-go/model/flow"
)

// blockBuildContext encapsulates required information about the cluster chain and
// main chain state needed to build a new cluster block proposal.
type blockBuildContext struct {
	parent                     *flow.Header    // parent of the block we are building
	clusterChainFinalizedBlock *flow.Header    // finalized block on the cluster chain
	refChainFinalizedHeight    uint64          // finalized height on reference chain
	refChainFinalizedID        flow.Identifier // finalized block ID on reference chain
	refEpochFirstHeight        uint64          // first height of this cluster's operating epoch
	refEpochFinalHeight        uint64          // last height of this cluster's operating epoch (may not be known)
	refEpochFinalID            flow.Identifier // ID of last block in this cluster's operating epoch (may not be known)
	refEpochHasEnded           bool            // whether this cluster's operating epoch has ended (and whether above 2 fields are known)
	config                     Config
}

// highestPossibleReferenceBlockHeight returns the height of the highest possible valid reference block.
// It is the highest finalized block which is in this cluster's operating epoch.
func (ctx *blockBuildContext) highestPossibleReferenceBlockHeight() uint64 {
	if ctx.refEpochHasEnded {
		return ctx.refEpochFinalHeight
	}
	return ctx.refChainFinalizedHeight
}

// highestPossibleReferenceBlockID returns the ID of the highest possible valid reference block.
// It is the highest finalized block which is in this cluster's operating epoch.
func (ctx *blockBuildContext) highestPossibleReferenceBlockID() flow.Identifier {
	if ctx.refEpochHasEnded {
		return ctx.refEpochFinalID
	}
	return ctx.refChainFinalizedID
}

// lowestPossibleReferenceBlockHeight returns the height of the lowest possible valid reference block.
// This is the higher of:
//   - the first block in this cluster's operating epoch
//   - the lowest block which could be used as a reference block without being
//     immediately expired (accounting for the configured expiry buffer)
func (ctx *blockBuildContext) lowestPossibleReferenceBlockHeight() uint64 {
	minPossibleRefHeight := ctx.refChainFinalizedHeight - uint64(flow.DefaultTransactionExpiry-ctx.config.ExpiryBuffer)
	if minPossibleRefHeight > ctx.refChainFinalizedHeight {
		minPossibleRefHeight = 0 // overflow check
	}
	if minPossibleRefHeight < ctx.refEpochFirstHeight {
		minPossibleRefHeight = ctx.refEpochFirstHeight
	}
	return minPossibleRefHeight
}
