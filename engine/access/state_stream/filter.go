package state_stream

import (
	"github.com/onflow/flow-go/model/flow"
)

// EventFilter represents a filter applied to events for a given subscription
type EventFilter struct {
	hasFilters bool
	EventTypes map[flow.EventType]struct{}
	Addresses  map[string]struct{}
	Contracts  map[string]struct{}
	EventNames map[string]struct{}
}

func NewEventFilter(
	eventTypes []string,
	addresses []string,
	contracts []string,
	eventNames []string,
) EventFilter {
	f := EventFilter{
		EventTypes: make(map[flow.EventType]struct{}, len(eventTypes)),
		Addresses:  make(map[string]struct{}, len(addresses)),
		Contracts:  make(map[string]struct{}, len(contracts)),
		EventNames: make(map[string]struct{}, len(eventNames)),
	}
	for _, eventType := range eventTypes {
		f.EventTypes[flow.EventType(eventType)] = struct{}{}
	}
	for _, address := range addresses {
		// convert to flow.Address to ensure it's in the correct format, but use the string value
		// for matching to avoid an address conversion for every event
		f.Addresses[flow.HexToAddress(address).String()] = struct{}{}
	}
	for _, contract := range contracts {
		f.Contracts[contract] = struct{}{}
	}
	for _, eventName := range eventNames {
		f.EventNames[eventName] = struct{}{}
	}
	f.hasFilters = len(f.EventTypes) > 0 || len(f.Addresses) > 0 || len(f.Contracts) > 0 || len(f.EventNames) > 0
	return f
}

// Filter applies the all filters on the provided list of events, and returns a list of events that
// match
func (f *EventFilter) Filter(events flow.EventsList) flow.EventsList {
	var filteredEvents flow.EventsList
	for _, event := range events {
		if f.Match(event) {
			filteredEvents = append(filteredEvents, event)
		}
	}
	return filteredEvents
}

// Match applies all filters to a specific event, and returns true if the event matches
func (f *EventFilter) Match(event flow.Event) bool {
	// No filters means all events match
	if !f.hasFilters {
		return true
	}

	if _, ok := f.EventTypes[event.Type]; ok {
		return true
	}

	parsed, err := ParseEvent(event.Type)
	if err != nil {
		// TODO: log this error
		return false
	}

	if _, ok := f.EventNames[parsed.Name]; ok {
		return true
	}

	if _, ok := f.Contracts[parsed.Contract]; ok {
		return true
	}

	if parsed.Type == AccountEventType {
		_, ok := f.Addresses[parsed.Address]
		return ok
	}

	return false
}
