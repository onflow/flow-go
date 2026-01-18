package backend

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

// executionStateSnapshotBuilder is responsible for constructing execution state snapshots for given criteria and block heights.
type executionStateSnapshotBuilder struct {
	headers                 storage.Headers
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
}

type executionStateSnapshotMetadata struct {
	BlockHeader         *flow.Header
	ExecutionResultInfo *optimistic_sync.ExecutionResultInfo
	Snapshot            optimistic_sync.Snapshot
}

func newExecutionStateSnapshotBuilder(
	headers storage.Headers,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *executionStateSnapshotBuilder {
	return &executionStateSnapshotBuilder{
		headers:                 headers,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
	}
}

// BuildSnapshot constructs an execution state snapshot for a specific block height based on the given criteria.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound]: If no finalized block is known at the given height
//   - [optimistic_sync.ErrBlockBeforeNodeHistory]: If the request is for data before the node's root block.
//   - [optimistic_sync.ErrExecutionResultNotReady]: If criteria cannot be satisfied at the moment.
//   - [optimistic_sync.CriteriaNotMetError]: Returned when the block is already sealed but no execution result
//     can satisfy the provided criteria.
//   - [optimistic_sync.SnapshotNotFoundError]: Result is not available, not ready for querying,
//     or does not descend from the latest sealed result.
func (p *executionStateSnapshotBuilder) BuildSnapshot(
	height uint64,
	criteria optimistic_sync.Criteria,
) (*executionStateSnapshotMetadata, error) {
	header, err := p.headers.ByHeight(height)
	if err != nil {
		return nil, err
	}

	result, err := p.executionResultProvider.ExecutionResultInfo(header.ID(), criteria)
	if err != nil {
		return nil, err
	}

	snapshot, err := p.executionStateCache.Snapshot(result.ExecutionResultID)
	if err != nil {
		return nil, err
	}

	return &executionStateSnapshotMetadata{
		BlockHeader:         header,
		ExecutionResultInfo: result,
		Snapshot:            snapshot,
	}, err
}
