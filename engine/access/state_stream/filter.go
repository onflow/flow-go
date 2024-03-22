package state_stream

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// DefaultMaxEventTypes is the default maximum number of event types that can be specified in a filter
	DefaultMaxEventTypes = 1000

	// DefaultMaxAddresses is the default maximum number of addresses that can be specified in a filter
	DefaultMaxAddresses = 1000

	// DefaultMaxContracts is the default maximum number of contracts that can be specified in a filter
	DefaultMaxContracts = 1000
)

// EventFilterConfig is used to configure the limits for EventFilters
type EventFilterConfig struct {
	MaxEventTypes int
	MaxAddresses  int
	MaxContracts  int
}

// DefaultEventFilterConfig is the default configuration for EventFilters
var DefaultEventFilterConfig = EventFilterConfig{
	MaxEventTypes: DefaultMaxEventTypes,
	MaxAddresses:  DefaultMaxAddresses,
	MaxContracts:  DefaultMaxContracts,
}

type FieldFilter map[string]map[string]struct{}

// EventFilter represents a filter applied to events for a given subscription
type EventFilter struct {
	hasFilters        bool
	EventTypes        map[flow.EventType]struct{}
	Addresses         map[string]struct{}
	Contracts         map[string]struct{}
	EventFieldFilters map[flow.EventType]FieldFilter
}

func NewEventFilter(
	config EventFilterConfig,
	chain flow.Chain,
	eventTypes []string,
	addresses []string,
	contracts []string,
) (EventFilter, error) {
	// put some reasonable limits on the number of filters. Lookups use a map so they are fast,
	// this just puts a cap on the memory consumed per filter.
	if len(eventTypes) > config.MaxEventTypes {
		return EventFilter{}, fmt.Errorf("too many event types in filter (%d). use %d or fewer", len(eventTypes), config.MaxEventTypes)
	}

	if len(addresses) > config.MaxAddresses {
		return EventFilter{}, fmt.Errorf("too many addresses in filter (%d). use %d or fewer", len(addresses), config.MaxAddresses)
	}

	if len(contracts) > config.MaxContracts {
		return EventFilter{}, fmt.Errorf("too many contracts in filter (%d). use %d or fewer", len(contracts), config.MaxContracts)
	}

	f := EventFilter{
		EventTypes:        make(map[flow.EventType]struct{}, len(eventTypes)),
		Addresses:         make(map[string]struct{}, len(addresses)),
		Contracts:         make(map[string]struct{}, len(contracts)),
		EventFieldFilters: make(map[flow.EventType]FieldFilter),
	}

	// Check all of the filters to ensure they are correctly formatted. This helps avoid searching
	// with criteria that will never match.
	for _, event := range eventTypes {
		eventType := flow.EventType(event)
		if err := validateEventType(eventType, chain); err != nil {
			return EventFilter{}, err
		}
		f.EventTypes[eventType] = struct{}{}
	}

	for _, address := range addresses {
		addr := flow.HexToAddress(address)
		if err := validateAddress(addr, chain); err != nil {
			return EventFilter{}, err
		}
		// use the parsed address to make sure it will match the event address string exactly
		f.Addresses[addr.String()] = struct{}{}
	}

	for _, contract := range contracts {
		if err := validateContract(contract); err != nil {
			return EventFilter{}, err
		}
		f.Contracts[contract] = struct{}{}
	}

	f.hasFilters = len(f.EventTypes) > 0 || len(f.Addresses) > 0 || len(f.Contracts) > 0
	return f, nil
}

// addCoreEventFieldFilter adds a field filter for each core event type
func (f *EventFilter) addCoreEventFieldFilter(eventType flow.EventType, address string, chain flow.Chain) error {
	if err := validateEventType(eventType, chain); err != nil {
		return fmt.Errorf("impossible to add event field filter: %w", err)
	}

	if f.EventFieldFilters[eventType] == nil {
		f.EventFieldFilters[eventType] = make(FieldFilter)
	}

	switch eventType {
	case CoreEventAccountCreated,
		CoreEventAccountKeyAdded,
		CoreEventAccountKeyRemoved,
		CoreEventAccountContractAdded,
		CoreEventAccountContractUpdated,
		CoreEventAccountContractRemoved:
		if f.EventFieldFilters[eventType]["address"] == nil {
			f.EventFieldFilters[eventType]["address"] = make(map[string]struct{})
		}
		f.EventFieldFilters[eventType]["address"][address] = struct{}{}
	case CoreEventInboxValuePublished,
		CoreEventInboxValueClaimed:
		if f.EventFieldFilters[eventType]["provider"] == nil {
			f.EventFieldFilters[eventType]["provider"] = make(map[string]struct{})
		}
		f.EventFieldFilters[eventType]["provider"][address] = struct{}{}

		if f.EventFieldFilters[eventType]["recipient"] == nil {
			f.EventFieldFilters[eventType]["recipient"] = make(map[string]struct{})
		}
		f.EventFieldFilters[eventType]["recipient"][address] = struct{}{}
	case CoreEventInboxValueUnpublished:
		if f.EventFieldFilters[eventType]["provider"] == nil {
			f.EventFieldFilters[eventType]["provider"] = make(map[string]struct{})
		}
		f.EventFieldFilters[eventType]["provider"][address] = struct{}{}
	default:
		return fmt.Errorf("unsupported event type: %s", eventType)
	}

	return nil
}

// Filter applies the all filters on the provided list of events, and returns a list of events that match
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

	if fieldFilter, ok := f.EventFieldFilters[event.Type]; ok {
		return f.matchFieldFilter(&event, fieldFilter)
	}

	if _, ok := f.EventTypes[event.Type]; ok {
		return true
	}

	parsed, err := events.ParseEvent(event.Type)
	if err != nil {
		// TODO: log this error
		return false
	}

	if _, ok := f.Contracts[parsed.Contract]; ok {
		return true
	}

	if parsed.Type == events.AccountEventType {
		_, ok := f.Addresses[parsed.Address]
		return ok
	}

	return false
}

// matchFieldFilter checks if the given event matches the specified field filters.
// It returns true if the event matches any of the provided field filters, otherwise false.
func (f *EventFilter) matchFieldFilter(event *flow.Event, fieldFilters FieldFilter) bool {
	if len(fieldFilters) == 0 {
		return true // empty list always matches
	}

	fields, fieldValues, err := getEventFields(event)
	if err != nil {
		return false
	}

	for i, field := range fields {
		filters, ok := fieldFilters[field.Identifier]
		if !ok {
			continue // no filter for this field
		}

		fieldValue := fieldValues[i].String()
		if _, ok := filters[fieldValue]; ok {
			return true
		}
	}

	return false
}

// getEventFields extracts field values and field names from the payload of a flow event.
// It decodes the event payload into a Cadence event, retrieves the field values and fields, and returns them.
// Parameters:
// - event: The Flow event to extract field values and field names from.
// Returns:
// - []cadence.Field: A slice containing names for each field extracted from the event payload.
// - []cadence.Value: A slice containing the values of the fields extracted from the event payload.
// - error: An error, if any, encountered during event decoding or if the fields are empty.
func getEventFields(event *flow.Event) ([]cadence.Field, []cadence.Value, error) {
	data, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, nil, err
	}

	cdcEvent, ok := data.(cadence.Event)
	if !ok {
		return nil, nil, err
	}

	fieldValues := cdcEvent.GetFieldValues()
	fields := cdcEvent.GetFields()
	if fieldValues == nil || fields == nil {
		return nil, nil, fmt.Errorf("fields are empty")
	}
	return fields, fieldValues, nil
}

// validateEventType ensures that the event type matches the expected format
func validateEventType(eventType flow.EventType, chain flow.Chain) error {
	_, err := events.ValidateEvent(eventType, chain)
	if err != nil {
		return fmt.Errorf("invalid event type %s: %w", eventType, err)
	}
	return nil
}

// validateAddress ensures that the address is valid for the given chain
func validateAddress(address flow.Address, chain flow.Chain) error {
	if !chain.IsValid(address) {
		return fmt.Errorf("invalid address for chain: %s", address)
	}
	return nil
}

// validateContract ensures that the contract is in the correct format
func validateContract(contract string) error {
	if contract == "flow" {
		return nil
	}

	parts := strings.Split(contract, ".")
	if len(parts) != 3 || parts[0] != "A" {
		return fmt.Errorf("invalid contract: %s", contract)
	}
	return nil
}
