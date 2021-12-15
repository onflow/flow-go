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
	RangeReceived(headers []flow.Header, originID flow.Identifier)

	// BatchReceived updates sync state after a Batch Request response is received.
	BatchReceived(headers []flow.Header, originID flow.Identifier)

	// HeightReceived updates sync state after a Sync Height response is received.
	HeightReceived(height uint64, originID flow.Identifier)
}

type ActiveRange interface {
	// Update processes a range of blocks received from a Range Request
	// response and updates the requestable height range.
	Update(headers []flow.Header, originID flow.Identifier)

	// LocalFinalizedHeight is called to notify a change in the local finalized height.
	LocalFinalizedHeight(height uint64)

	// TargetFinalizedHeight is called to notify a change in the target finalized height.
	TargetFinalizedHeight(height uint64)

	// Get returns the range of requestable block heights.
	Get() flow.Range
}

type TargetFinalizedHeight interface {
	// Update processes a height received from a Sync Height Response
	// and updates the finalized height estimate.
	Update(height uint64, originID flow.Identifier)

	// Get returns the estimated finalized height of the overall chain.
	Get() uint64
}
