package state_stream

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type EventFilter struct {
	hasFilters bool
	EventTypes map[flow.EventType]bool
	Addresses  map[string]bool
	Contracts  map[string]bool
}

func NewEventFilter(
	eventTypes []string,
	addresses []string,
	contracts []string,
) EventFilter {
	f := EventFilter{
		EventTypes: make(map[flow.EventType]bool, len(eventTypes)),
		Addresses:  make(map[string]bool, len(addresses)),
		Contracts:  make(map[string]bool, len(contracts)),
	}
	for _, eventType := range eventTypes {
		f.EventTypes[flow.EventType(eventType)] = true
	}
	for _, address := range addresses {
		f.Addresses[flow.HexToAddress(address).String()] = true
	}
	for _, contract := range contracts {
		f.Contracts[contract] = true
	}
	f.hasFilters = len(f.EventTypes) > 0 || len(f.Addresses) > 0 || len(f.Contracts) > 0
	return f
}

func (f *EventFilter) Filter(events flow.EventsList) flow.EventsList {
	var filteredEvents flow.EventsList
	for _, event := range events {
		if f.Match(event) {
			filteredEvents = append(filteredEvents, event)
		}
	}
	return filteredEvents
}

func (f *EventFilter) Match(event flow.Event) bool {
	if !f.hasFilters {
		return true
	}

	if f.EventTypes[event.Type] {
		return true
	}

	parts := strings.Split(string(event.Type), ".")

	if len(parts) < 2 {
		// TODO: log the error
		return false
	}

	// name := parts[len(parts)-1]
	contract := parts[len(parts)-2]
	if f.Contracts[contract] {
		return true
	}

	if len(parts) > 2 && f.Addresses[parts[1]] {
		return true
	}

	return false
}

type EventsResponse struct {
	BlockID flow.Identifier
	Height  uint64
	Events  flow.EventsList
}

type EventsBackend struct {
	log         zerolog.Logger
	headers     storage.Headers
	broadcaster *engine.Broadcaster
	sendTimeout time.Duration

	getExecutionData GetExecutionDataFunc
	getStartHeight   GetStartHeightFunc
}

// TODO: add polling endpoint. To start, this could just get the execution data for a block/range of blocks
// and filter the events.

func (b EventsBackend) SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription {
	sub := &HeightBasedSubscription{
		SubscriptionImpl: NewSubscription(),
		getData:          b.getResponseFactory(filter),
	}

	nextHeight, err := b.getStartHeight(startBlockID, startHeight)
	if err != nil {
		sub.Fail(fmt.Errorf("could not get start height: %w", err))
		return sub
	}

	sub.nextHeight = nextHeight

	go NewStreamer(b.log, b.broadcaster, b.sendTimeout, sub).Stream(ctx)

	return sub
}

func (b EventsBackend) getResponseFactory(filter EventFilter) GetDataByHeightFunc {
	return func(ctx context.Context, height uint64) (interface{}, error) {
		header, err := b.headers.ByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get block header: %w", err)
		}

		executionData, err := b.getExecutionData(ctx, header.ID())
		if err != nil {
			return nil, err
		}

		events := []flow.Event{}
		for _, chunkExecutionData := range executionData.ChunkExecutionDatas {
			events = append(events, filter.Filter(chunkExecutionData.Events)...)
		}

		return &EventsResponse{
			BlockID: header.ID(),
			Height:  header.Height,
			Events:  events,
		}, nil
	}
}
