package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

// executionDataProvider connects subscription/streamer with backends.
// It is intended to be used as a data provider for the subscription package.
//
// NOT CONCURRENCY SAFE! executionDataProvider is designed to be used by a single goroutine.
type executionDataProvider struct {
	providerCore
	executionDataTracker tracker.ExecutionDataTracker
}

var _ subscription.DataProvider = (*executionDataProvider)(nil)

func newExecutionDataProvider(
	state protocol.State,
	executionDataTracker tracker.ExecutionDataTracker,
	snapshotBuilder *executionStateSnapshotBuilder,
	nextCriteria optimistic_sync.Criteria,
	startHeight uint64,
) *executionDataProvider {
	return &executionDataProvider{
		providerCore: providerCore{
			state:           state,
			snapshotBuilder: snapshotBuilder,
			criteria:        nextCriteria,
			blockHeight:     startHeight,
		},
		executionDataTracker: executionDataTracker,
	}
}

// NextData returns the execution data for the next block height.
// It is intended to be used by the streamer to fetch data sequentially.
//
// Expected errors during normal operations:
//   - [subscription.ErrBlockNotReady]: If the execution data is not yet available. This includes cases where
//     the block is not finalized yet, or the execution result is pending (e.g. not enough agreeing executors).
//   - [optimistic_sync.ErrBlockBeforeNodeHistory]: If the request is for data before the node's root block.
//   - [optimistic_sync.SnapshotNotFoundError]: Result is not available, not ready for querying, or does not descend from the latest sealed result.
//   - [optimistic_sync.CriteriaNotMetError]: Returned when the block is already sealed but no execution result can satisfy the provided criteria.
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *executionDataProvider) NextData(ctx context.Context) (any, error) {
	// execution data provider specific check: it's possible for the data to exist in the data store before
	// the notification is received. This ensures a consistent view is available to all streams.
	availableFinalizedHeight := p.executionDataTracker.GetHighestAvailableFinalizedHeight()
	if p.blockHeight > availableFinalizedHeight {
		return nil, subscription.ErrBlockNotReady
	}

	metadata, err := p.getSnapshotMetadata()
	if err != nil {
		return nil, err
	}

	// handle spork root special case
	if p.isSporkRoot() {
		sporkRootBlock := p	.state.Params().SporkRootBlock()
		response := &ExecutionDataResponse{
			Height: p.blockHeight,
			ExecutionData: &execution_data.BlockExecutionData{
				BlockID: sporkRootBlock.ID(),
			},
			BlockTimestamp: time.UnixMilli(int64(sporkRootBlock.Timestamp)).UTC(),
		}

		p.incrementHeight(metadata.ExecutionResultInfo.ExecutionResultID)
		return response, nil
	}

	// extract execution data from snapshot
	blockID := metadata.BlockHeader.ID()
	executionData, err := metadata.Snapshot.BlockExecutionData().ByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find execution data for block %s: %w", blockID, err)
	}

	// build response
	response := &ExecutionDataResponse{
		Height:        p.blockHeight,
		ExecutionData: executionData.BlockExecutionData,
		ExecutorMetadata: accessmodel.ExecutorMetadata{
			ExecutionResultID: metadata.ExecutionResultInfo.ExecutionResultID,
			ExecutorIDs:       metadata.ExecutionResultInfo.ExecutionNodes.NodeIDs(),
		},
		BlockTimestamp: time.UnixMilli(int64(metadata.BlockHeader.Timestamp)).UTC(),
	}

	p.incrementHeight(metadata.ExecutionResultInfo.ExecutionResultID)
	return response, nil
}
