package state_stream

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// Core event types based on documentation https://cadence-lang.org/docs/language/core-events
const (
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

var defaultCoreEventsMap map[string]struct{}

func init() {
	defaultCoreEventsMap = make(map[string]struct{}, len(DefaultCoreEvents))
	for i := range DefaultCoreEvents {
		defaultCoreEventsMap[DefaultCoreEvents[i]] = struct{}{}
	}
}

// DefaultCoreEvents is an array containing all default core event types.
var DefaultCoreEvents = []string{
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

// AccountStatusFilter defines a specific filter for account statuses.
// It embeds the EventFilter type to inherit its functionality.
type AccountStatusFilter struct {
	*EventFilter
}

// NewAccountStatusFilter creates a new AccountStatusFilter based on the provided configuration.
// Expected errors:
// - error: An error, if any, encountered during core event type validating, check for max account addresses
// or validating account addresses.
func NewAccountStatusFilter(
	config EventFilterConfig,
	chain flow.Chain,
	eventTypes []string,
	accountAddresses []string,
) (AccountStatusFilter, error) {
	if len(accountAddresses) == 0 {
		// If `accountAddresses` is empty, the validation on `addCoreEventFieldFilter` would not happen.
		// Therefore, event types are validated with `validateCoreEventTypes` to fail at the beginning of filter creation.
		err := validateCoreEventTypes(eventTypes)
		if err != nil {
			return AccountStatusFilter{}, err
		}
	} else if len(accountAddresses) > DefaultMaxAccountAddresses {
		// If `accountAddresses` exceeds the `DefaultAccountAddressesLimit`, it returns an error.
		return AccountStatusFilter{}, fmt.Errorf("account limit exceeds, the limit is %d", DefaultMaxAccountAddresses)
	}

	// If `eventTypes` is empty, the filter returns all core events for any accounts.
	if len(eventTypes) == 0 {
		eventTypes = DefaultCoreEvents
	}

	//  It's important to only set eventTypes if there are no addresses passed.
	var filterEventTypes []string
	if len(accountAddresses) == 0 {
		filterEventTypes = eventTypes
	}

	// Creates an `EventFilter` with the provided `eventTypes`.
	filter, err := NewEventFilter(config, chain, filterEventTypes, []string{}, []string{})
	if err != nil {
		return AccountStatusFilter{}, err
	}

	accountStatusFilter := AccountStatusFilter{
		EventFilter: &filter,
	}

	for _, address := range accountAddresses {
		// Validate account address
		addr := flow.HexToAddress(address)
		if err := validateAddress(addr, chain); err != nil {
			return AccountStatusFilter{}, err
		}

		// If there are non-core event types at this stage, it returns an error from `addCoreEventFieldFilter`.
		for _, eventType := range eventTypes {
			// use the hex with prefix address to make sure it will match the cadence address
			err = accountStatusFilter.addCoreEventFieldFilter(flow.EventType(eventType), addr.HexWithPrefix(), chain)
			if err != nil {
				return AccountStatusFilter{}, err
			}
		}
	}

	// We need to set hasFilters here if filterEventTypes was empty
	accountStatusFilter.hasFilters = len(accountStatusFilter.EventFieldFilters) > 0 || len(eventTypes) > 0

	return accountStatusFilter, nil
}

// GroupCoreEventsByAccountAddress extracts account-related core events from the provided list of events.
// It filters events based on the account field specified by the event type and organizes them by account address.
// Parameters:
// - events: The list of events to extract account-related core events from.
// - log: The logger to log errors encountered during event decoding and processing.
// Returns:
//   - A map[string]flow.EventsList: A map where the key is the account address and the value is a list of
//     account-related core events associated with that address.
func (f *AccountStatusFilter) GroupCoreEventsByAccountAddress(events flow.EventsList, log zerolog.Logger) map[string]flow.EventsList {
	allAccountProtocolEvents := make(map[string]flow.EventsList)

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

// addCoreEventFieldFilter adds a field filter for each core event type
func (f *AccountStatusFilter) addCoreEventFieldFilter(eventType flow.EventType, address string, chain flow.Chain) error {
	addFilter := func(field, value string) {
		if _, ok := f.EventFieldFilters[eventType]; !ok {
			f.EventFieldFilters[eventType] = make(FieldFilter)
		}
		if _, ok := f.EventFieldFilters[eventType][field]; !ok {
			f.EventFieldFilters[eventType][field] = make(map[string]struct{})
		}
		f.EventFieldFilters[eventType][field][value] = struct{}{}
	}

	switch eventType {
	case CoreEventAccountCreated,
		CoreEventAccountKeyAdded,
		CoreEventAccountKeyRemoved,
		CoreEventAccountContractAdded,
		CoreEventAccountContractUpdated,
		CoreEventAccountContractRemoved:
		addFilter("address", address)
	case CoreEventInboxValuePublished,
		CoreEventInboxValueClaimed:
		addFilter("provider", address)
		addFilter("recipient", address)
	case CoreEventInboxValueUnpublished:
		addFilter("provider", address)
	default:
		return fmt.Errorf("unsupported event type: %s", eventType)
	}

	return nil
}

// validateCoreEventTypes validates the provided event types against the default core event types.
// It returns an error if any of the provided event types are not in the default core event types list. Note, an empty
// event types array is also valid.
func validateCoreEventTypes(eventTypes []string) error {
	for _, eventType := range eventTypes {
		_, ok := defaultCoreEventsMap[eventType]
		// If the provided event type does not match any of the default core event types, return an error
		if !ok {
			return fmt.Errorf("invalid provided event types for filter")
		}
	}

	return nil // All provided event types are valid core event types or event types are empty
}
