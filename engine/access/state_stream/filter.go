package state_stream

import (
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

type EventFilter struct {
	hasFilters bool
	EventTypes map[flow.EventType]struct{}
	Addresses  map[string]struct{}
	Contracts  map[string]struct{}
}

func NewEventFilter(
	eventTypes []string,
	addresses []string,
	contracts []string,
) EventFilter {
	f := EventFilter{
		EventTypes: make(map[flow.EventType]struct{}, len(eventTypes)),
		Addresses:  make(map[string]struct{}, len(addresses)),
		Contracts:  make(map[string]struct{}, len(contracts)),
	}
	for _, eventType := range eventTypes {
		f.EventTypes[flow.EventType(eventType)] = struct{}{}
	}
	for _, address := range addresses {
		f.Addresses[flow.HexToAddress(address).String()] = struct{}{}
	}
	for _, contract := range contracts {
		f.Contracts[contract] = struct{}{}
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

	if _, ok := f.EventTypes[event.Type]; ok {
		return true
	}

	parts := strings.Split(string(event.Type), ".")

	// There are 2 valid EventType formats:
	// * flow.[EventName]
	// * A.[Address].[Contract].[EventName]
	if len(parts) != 2 && len(parts) != 4 {
		return false
	}

	contract := parts[len(parts)-2]
	if _, ok := f.Contracts[contract]; ok {
		return true
	}

	if len(parts) > 2 {
		_, ok := f.Addresses[parts[1]]
		return ok
	}

	return false
}
