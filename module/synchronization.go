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

	// RequestHeight indicates that the given block height should be queued for retrieval.
	RequestHeight(height uint64)

	// Manually Prune requests
	Prune(final *flow.Header)
}

// SyncCore represents state management for chain state synchronization.
type SyncCore interface {

	// HandleBlock handles receiving a new block. It returns true if the block
	// should be passed along to the rest of the system for processing, or false
	// if it should be discarded.
	HandleBlock(header *flow.Header) bool

	// HandleBlock handles receiving a new finalized block. It returns true if the
	// block should be passed along to the rest of the system for processing, or
	// false if it should be discarded.
	HandleFinalizedBlock(header *flow.Header) bool

	// HandleHeight handles receiving a new highest finalized height from another node.
	HandleHeight(final *flow.Header, height uint64)

	// ScanPending scans all pending block statuses for blocks that should be
	// requested. It apportions requestable items into range and batch requests
	// according to configured maximums, giving precedence to range requests.
	ScanPending(final *flow.Header) ([]flow.Range, []flow.Batch)

	// WithinTolerance returns whether or not the given height is within configured
	// height tolerance, wrt the given local finalized header.
	WithinTolerance(final *flow.Header, height uint64) bool

	// RangeRequested updates sync state after a range is requested.
	RangeRequested(ran flow.Range)

	// BatchRequested updates sync state after a batch is requested.
	BatchRequested(batch flow.Batch)
}
