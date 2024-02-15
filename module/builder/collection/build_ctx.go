package collection

import (
	"github.com/onflow/flow-go/model/flow"
)

// blockBuildContext encapsulates required information about the cluster chain and
// main chain state needed to build a new cluster block proposal.
type blockBuildContext struct {
	parentID                   flow.Identifier  // ID of the parent we are extending
	parent                     *flow.Header     // parent of the block we are building
	clusterChainFinalizedBlock *flow.Header     // finalized block on the cluster chain
	refChainFinalizedHeight    uint64           // finalized height on reference chain
	refChainFinalizedID        flow.Identifier  // finalized block ID on reference chain
	refEpochFirstHeight        uint64           // first height of this cluster's operating epoch
	refEpochFinalHeight        *uint64          // last height of this cluster's operating epoch (nil if epoch not ended)
	refEpochFinalID            *flow.Identifier // ID of last block in this cluster's operating epoch (nil if epoch not ended)
	config                     Config
	limiter                    *rateLimiter
	lookup                     *transactionLookup
}

// highestPossibleReferenceBlockHeight returns the height of the highest possible valid reference block.
// It is the highest finalized block which is in this cluster's operating epoch.
func (ctx *blockBuildContext) highestPossibleReferenceBlockHeight() uint64 {
	if ctx.refEpochFinalHeight != nil {
		return *ctx.refEpochFinalHeight
	}
	return ctx.refChainFinalizedHeight
}

// highestPossibleReferenceBlockID returns the ID of the highest possible valid reference block.
// It is the highest finalized block which is in this cluster's operating epoch.
func (ctx *blockBuildContext) highestPossibleReferenceBlockID() flow.Identifier {
	if ctx.refEpochFinalID != nil {
		return *ctx.refEpochFinalID
	}
	return ctx.refChainFinalizedID
}

// lowestPossibleReferenceBlockHeight returns the height of the lowest possible valid reference block.
// This is the higher of:
//   - the first block in this cluster's operating epoch
//   - the lowest block which could be used as a reference block without being
//     immediately expired (accounting for the configured expiry buffer)
func (ctx *blockBuildContext) lowestPossibleReferenceBlockHeight() uint64 {
	// By default, the lowest possible reference block for a non-expired collection has a height
	// δ below the latest finalized block, for `δ := flow.DefaultTransactionExpiry - ctx.config.ExpiryBuffer`
	// However, our current Epoch might not have δ finalized blocks yet, in which case the lowest
	// possible reference block is the first block in the Epoch.
	delta := uint64(flow.DefaultTransactionExpiry - ctx.config.ExpiryBuffer)
	if ctx.refChainFinalizedHeight <= ctx.refEpochFirstHeight+delta {
		return ctx.refEpochFirstHeight
	}
	return ctx.refChainFinalizedHeight - delta
}
