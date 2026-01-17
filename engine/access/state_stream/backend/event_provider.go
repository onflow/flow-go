package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

// eventProvider is responsible for managing event-related data and interactions within the protocol state.
// It interacts with multiple components, such as protocol state, execution results, and event filters.
//
// NOT CONCURRENCY SAFE! eventProvider is designed to be used by a single streamer goroutine.
type eventProvider struct {
	providerCore
	eventFilter state_stream.EventFilter
}

var _ subscription.DataProvider = (*eventProvider)(nil)

func newEventProvider(
	state protocol.State,
	snapshotBuilder *executionStateSnapshotBuilder,
	nextCriteria optimistic_sync.Criteria,
	startHeight uint64,
	eventFilter state_stream.EventFilter,
) *eventProvider {
	return &eventProvider{
		providerCore: providerCore{
			state:           state,
			snapshotBuilder: snapshotBuilder,
			criteria:        nextCriteria,
			blockHeight:     startHeight,
		},
		eventFilter: eventFilter,
	}
}

func (p *eventProvider) NextData(_ context.Context) (any, error) {
	metadata, err := p.getSnapshotMetadata()
	if err != nil {
		return nil, err
	}

	// handle spork root special case
	sporkRootBlock := p.state.Params().SporkRootBlock()
	if p.blockHeight == sporkRootBlock.Height {
		response := &EventsResponse{
			BlockID:        sporkRootBlock.ID(),
			Height:         sporkRootBlock.Height,
			BlockTimestamp: time.UnixMilli(int64(sporkRootBlock.Timestamp)).UTC(),
		}

		p.incrementHeight(metadata.ExecutionResultInfo.ExecutionResultID)
		return response, nil
	}

	// extract events from snapshot
	blockID := metadata.BlockHeader.ID()
	events, err := metadata.Snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find events for block %s: %w", blockID, err)
	}

	// apply filter
	filteredEvents := p.eventFilter.Filter(events)

	// build response
	response := &EventsResponse{
		BlockID:        blockID,
		Height:         metadata.BlockHeader.Height,
		Events:         filteredEvents,
		BlockTimestamp: time.UnixMilli(int64(metadata.BlockHeader.Timestamp)).UTC(),
	}

	p.incrementHeight(metadata.ExecutionResultInfo.ExecutionResultID)
	return response, nil
}
