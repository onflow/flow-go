package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

type accountStatusesProvider struct {
	providerCore
	log    zerolog.Logger
	filter state_stream.AccountStatusFilter
}

func newAccountStatusProvider(
	log zerolog.Logger,
	state protocol.State,
	snapshotBuilder *executionStateSnapshotBuilder,
	nextCriteria optimistic_sync.Criteria,
	startHeight uint64,
	filter state_stream.AccountStatusFilter,
) *accountStatusesProvider {
	return &accountStatusesProvider{
		providerCore: providerCore{
			state:           state,
			snapshotBuilder: snapshotBuilder,
			criteria:        nextCriteria,
			blockHeight:     startHeight,
		},
		log:    log,
		filter: filter,
	}
}

var _ subscription.DataProvider = (*accountStatusesProvider)(nil)

func (p *accountStatusesProvider) NextData(_ context.Context) (any, error) {
	metadata, err := p.getSnapshotMetadata()
	if err != nil {
		return nil, err
	}

	// handle spork root special case
	sporkRootBlock := p.state.Params().SporkRootBlock()
	if p.blockHeight == sporkRootBlock.Height {
		response := &AccountStatusesResponse{
			BlockID:        sporkRootBlock.ID(),
			Height:         sporkRootBlock.Height,
			BlockTimestamp: time.UnixMilli(int64(sporkRootBlock.Timestamp)).UTC(),
		}

		p.advanceToNextBlock(metadata.ExecutionResultInfo.ExecutionResultID)
		return response, nil
	}

	// extract events from snapshot
	blockID := metadata.BlockHeader.ID()
	events, err := metadata.Snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not find events for block %s: %w", blockID, err)
	}

	// apply filter and group by account address
	filteredEvents := p.filter.Filter(events)
	accountEvents := p.filter.GroupCoreEventsByAccountAddress(filteredEvents, p.log)

	// build response
	response := &AccountStatusesResponse{
		BlockID:       blockID,
		Height:        metadata.BlockHeader.Height,
		AccountEvents: accountEvents,
	}

	p.advanceToNextBlock(metadata.ExecutionResultInfo.ExecutionResultID)
	return response, nil
}
