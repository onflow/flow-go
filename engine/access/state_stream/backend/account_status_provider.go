package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

type accountStatusProvider struct {
	log                   zerolog.Logger
	executionDataProvider *executionDataProvider
	filter                state_stream.AccountStatusFilter
}

func newAccountStatusProvider(
	log zerolog.Logger,
	provider *executionDataProvider,
	filter state_stream.AccountStatusFilter,
) *accountStatusProvider {
	return &accountStatusProvider{
		log:                   log,
		executionDataProvider: provider,
		filter:                filter,
	}
}

var _ subscription.DataProvider = (*accountStatusProvider)(nil)

func (a *accountStatusProvider) NextData(ctx context.Context) (any, error) {
	executionDataRaw, err := a.executionDataProvider.NextData(ctx)
	if err != nil {
		return nil, err
	}

	executionData, ok := executionDataRaw.(*ExecutionDataResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected execution data type: %T", executionDataRaw)
	}

	// extract events from execution data
	var events flow.EventsList
	for _, chunkExecutionData := range executionData.ExecutionData.ChunkExecutionDatas {
		events = append(events, chunkExecutionData.Events...)
	}

	// apply filter and group by account address
	filteredProtocolEvents := a.filter.Filter(events)
	allAccountProtocolEvents := a.filter.GroupCoreEventsByAccountAddress(filteredProtocolEvents, a.log)

	return &AccountStatusesResponse{
		BlockID:       executionData.ExecutionData.BlockID,
		Height:        executionData.Height,
		AccountEvents: allAccountProtocolEvents,
	}, nil
}
