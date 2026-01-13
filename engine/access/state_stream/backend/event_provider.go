package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
)

type eventProvider struct {
	executionDataProvider *executionDataProvider
	eventFilter           state_stream.EventFilter
}

func newEventProvider(provider *executionDataProvider, eventFilter state_stream.EventFilter) *eventProvider {
	return &eventProvider{
		executionDataProvider: provider,
		eventFilter:           eventFilter,
	}
}

var _ subscription.DataProvider = (*eventProvider)(nil)

func (e *eventProvider) NextData(ctx context.Context) (any, error) {
	executionDataRaw, err := e.executionDataProvider.NextData(ctx)
	if err != nil {
		return nil, err
	}

	executionData, ok := executionDataRaw.(ExecutionDataResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected execution data type: %T", executionDataRaw)
	}

	// extract events from execution data
	var events flow.EventsList
	for _, chunkExecutionData := range executionData.ExecutionData.ChunkExecutionDatas {
		events = append(events, chunkExecutionData.Events...)
	}

	// apply filter
	events = e.eventFilter.Filter(events)

	return &EventsResponse{
		BlockID: executionData.ExecutionData.BlockID,
		Height:  executionData.Height,
		Events:  events,
	}, nil
}
