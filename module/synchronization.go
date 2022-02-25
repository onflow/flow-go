// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// BlockRequester enables components to request particular blocks by ID from
// synchronization system.
type BlockRequester interface {
	// RequestBlock indicates that the given block should be queued for retrieval.
	RequestBlock(blockID flow.Identifier, height uint64)
}

// SyncCore represents state management for chain state synchronization.
type SyncCore interface {
	// GetRequestableItems returns the set of requestable items.
	GetRequestableItems() (flow.Range, flow.Batch)

	// RangeReceived updates sync state after a Range Request response is received.
	RangeReceived(startHeight uint64, blockIDs []flow.Identifier, originID flow.Identifier)

	// BatchReceived updates sync state after a Batch Request response is received.
	BatchReceived(blockIDs []flow.Identifier, originID flow.Identifier)

	// LatestBlockReceived updates sync state after a Latest Finalized Block Request response is received.
	LatestFinalizedBlockReceived(blockID flow.Identifier, height uint64, originID flow.Identifier)
}

type ActiveRangeTracker interface {
	// ProcessRange processes a range of blocks received from a Range Request
	// response and updates the requestable height range.
	ProcessRange(startHeight uint64, blockIDs []flow.Identifier, originID flow.Identifier)

	// UpdateLowerBound is called to set the lower bound of the tracker.
	UpdateLowerBound(height uint64)

	// UpdateUpperBound is called to set the upper bound of the tracker.
	UpdateUpperBound(height uint64)

	// Get returns the range of requestable block heights.
	GetActiveRange() flow.Range
}

type TargetHeightTracker interface {
	// Update processes a height received from a Latest Finalized Block Response
	// and updates the finalized height estimate.
	ProcessHeight(height uint64, originID flow.Identifier)

	// Get returns the estimated finalized height of the overall chain.
	GetTargetHeight() uint64
}
