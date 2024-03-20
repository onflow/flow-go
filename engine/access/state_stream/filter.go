package state_stream

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"

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

	// Core event types based on documentation https://cadence-lang.org/docs/language/core-events

	// CoreEventAccountCreated is emitted when a new account gets created
	CoreEventAccountCreated = "flow.AccountCreated"

	// CoreEventAccountKeyAdded is emitted when a key gets added to an account
	CoreEventAccountKeyAdded = "flow.AccountKeyAdded"

	// CoreEventAccountKeyRemoved is emitted when a key gets removed from an account
	CoreEventAccountKeyRemoved = "flow.AccountKeyRemoved"

	// CoreEventAccountContractAdded is emitted when a contract gets deployed to an account
	CoreEventAccountContractAdded = "flow.AccountContractAdded"

	// CoreEventAccountContractUpdated is emitted when a contract gets updated on an account
	CoreEventAccountContractUpdated = "flow.AccountContractUpdated"

	// CoreEventAccountContractRemoved is emitted when a contract gets removed from an account
	CoreEventAccountContractRemoved = "flow.AccountContractRemoved"

	// CoreEventInboxValuePublished is emitted when a Capability is published from an account
	CoreEventInboxValuePublished = "flow.InboxValuePublished"

	// CoreEventInboxValueUnpublished is emitted when a Capability is unpublished from an account
	CoreEventInboxValueUnpublished = "flow.InboxValueUnpublished"

	// CoreEventInboxValueClaimed is emitted when a Capability is claimed by an account
	CoreEventInboxValueClaimed = "flow.InboxValueClaimed"
)

// defaultCoreEvents is a slice containing all default core event types.
var defaultCoreEvents = []string{
	CoreEventAccountCreated,
	CoreEventAccountKeyAdded,
	CoreEventAccountKeyRemoved,
	CoreEventAccountContractAdded,
	CoreEventAccountContractUpdated,
	CoreEventAccountContractRemoved,
	CoreEventInboxValuePublished,
	CoreEventInboxValueUnpublished,
	CoreEventInboxValueClaimed,
}

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

// EventFilter represents a filter applied to events for a given subscription
type EventFilter struct {
	hasFilters        bool
	EventTypes        map[flow.EventType]struct{}
	Addresses         map[string]struct{}
	Contracts         map[string]struct{}
	EventFieldFilters map[flow.EventType]map[string]map[string]struct{}
}

type FieldFilter struct {
	FieldName   string
	TargetValue string
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
		EventFieldFilters: make(map[flow.EventType]map[string]map[string]struct{}),
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

func (f *EventFilter) addCoreEventFieldFilter(eventType flow.EventType, address string) error {
	f.EventFieldFilters[eventType] = make(map[string]map[string]struct{})
	switch eventType {
	case "flow.AccountCreated",
		"flow.AccountKeyAdded",
		"flow.AccountKeyRemoved",
		"flow.AccountContractAdded",
		"flow.AccountContractUpdated",
		"flow.AccountContractRemoved":
		f.EventFieldFilters[eventType]["address"] = make(map[string]struct{})
		f.EventFieldFilters[eventType]["address"][address] = struct{}{}
		return nil
	case "flow.InboxValuePublished",
		"flow.InboxValueClaimed":
		f.EventFieldFilters[eventType]["provider"] = make(map[string]struct{})
		f.EventFieldFilters[eventType]["provider"][address] = struct{}{}
		f.EventFieldFilters[eventType]["recipient"] = make(map[string]struct{})
		f.EventFieldFilters[eventType]["recipient"][address] = struct{}{}
	case "flow.InboxValueUnpublished":
		f.EventFieldFilters[eventType]["provider"] = make(map[string]struct{})
		f.EventFieldFilters[eventType]["provider"][address] = struct{}{}
		return nil
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

	if _, ok := f.EventTypes[event.Type]; ok {
		return f.matchFieldFilter(&event)
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

// matchFieldFilter checks if the provided event matches the field filters defined in the event filter.
//
// This method is called internally by the EventFilter's Match method to determine if an event matches the field filters criteria.
// It compares the field values of the event with the target values specified in the field filters.
// If the event matches all field filters, it returns true; otherwise, it returns false.
// If there is an error decoding the event payload or if the event does not contain the expected fields, it returns false.
func (f *EventFilter) matchFieldFilter(event *flow.Event) bool {
	fieldFilters, ok := f.EventFieldFilters[event.Type]
	if !ok {
		return false // no filter for this event
	}

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

type AccountStatusFilter struct {
	*EventFilter
}

func NewAccountStatusFilter(
	config EventFilterConfig,
	chain flow.Chain,
	eventTypes []string,
	accountAddresses []string,
) (AccountStatusFilter, error) {
	filterEventTypes, err := GetCoreEventTypes(eventTypes)
	if err != nil {
		return AccountStatusFilter{}, err
	}

	filter, err := NewEventFilter(
		config,
		chain,
		filterEventTypes,
		[]string{},
		[]string{},
	)
	if err != nil {
		return AccountStatusFilter{}, err
	}

	if len(accountAddresses) > 0 {
		for eventType := range filter.EventTypes {
			for _, address := range accountAddresses {
				err = filter.addCoreEventFieldFilter(eventType, address)
				if err != nil {
					return AccountStatusFilter{}, err
				}
			}
		}
	}

	return AccountStatusFilter{
		EventFilter: &filter,
	}, nil
}

// CreateAccountRelatedCoreEvents extracts account-related core events from the provided list of events.
// It filters events based on the account field specified by the event type and organizes them by account address.
// Parameters:
// - events: The list of events to extract account-related core events from.
// - log: The logger to log errors encountered during event decoding and processing.
// Returns:
//   - A map[string]flow.EventsList: A map where the key is the account address and the value is a list of
//     account-related core events associated with that address.
func (f *AccountStatusFilter) CreateAccountRelatedCoreEvents(events flow.EventsList, log zerolog.Logger) map[string]flow.EventsList {
	allAccountProtocolEvents := map[string]flow.EventsList{}

	for _, event := range events {
		fields, fieldValues, err := getEventFields(&event)
		if err != nil {
			log.Info().Err(err).Msg("could not get event fields")
			continue
		}

		accountField := f.EventFieldFilters[event.Type]
		for i, field := range fields {
			_, ok := accountField[field.Identifier]
			if ok {
				address := fieldValues[i].String()
				allAccountProtocolEvents[address] = append(allAccountProtocolEvents[address], event)
			}
		}
	}

	return allAccountProtocolEvents
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

// GetCoreEventTypes validates the provided core event types and returns them if they are all core events.
// If no event types are provided, it returns all default core events.
func GetCoreEventTypes(providedEventTypes []string) ([]string, error) {
	if len(providedEventTypes) > 0 {
		for _, eventType := range providedEventTypes {
			isMatch := false
			for _, coreEventType := range defaultCoreEvents {
				if coreEventType == eventType {
					isMatch = true
					break
				}
			}

			if !isMatch {
				return nil, fmt.Errorf("invalid provided event types for filter")
			}
		}

		return providedEventTypes, nil
	}

	return defaultCoreEvents, nil
}

// validateEventType ensures that the event type matches the expected format
func validateEventType(eventType flow.EventType, chain flow.Chain) error {
	_, err := events.ValidateEvent(flow.EventType(eventType), chain)
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
