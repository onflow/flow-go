package state_stream

import (
	"strings"

	"github.com/onflow/flow-go/model/flow"
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
