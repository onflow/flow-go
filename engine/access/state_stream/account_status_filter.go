package state_stream

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

// Core event types based on documentation https://cadence-lang.org/docs/language/core-events
const (
	// DefaultAccountAddressesLimit specifies limitation for possible number of accounts that could be used in filter
	DefaultAccountAddressesLimit = 50

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

// defaultCoreEvents is an array containing all default core event types.
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

// AccountStatusFilter defines a specific filter for account statuses.
// It embeds the EventFilter type to inherit its functionality.
type AccountStatusFilter struct {
	*EventFilter
}

// NewAccountStatusFilter creates a new AccountStatusFilter based on the provided configuration.
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
	} else if len(accountAddresses) > DefaultAccountAddressesLimit {
		// If `accountAddresses` exceeds the `DefaultAccountAddressesLimit`, it returns an error.
		return AccountStatusFilter{}, fmt.Errorf("account limit exceeds, the limit is %d", DefaultAccountAddressesLimit)
	}

	// If `eventTypes` is empty, the filter returns all core events for any accounts.
	if len(eventTypes) == 0 {
		eventTypes = defaultCoreEvents
	}

	// Creates an `EventFilter` with the provided `eventTypes`.
	filter, err := NewEventFilter(config, chain, eventTypes, []string{}, []string{})
	if err != nil {
		return AccountStatusFilter{}, err
	}

	for _, address := range accountAddresses {
		//Validate account address
		addr := flow.HexToAddress(address)
		if err := validateAddress(addr, chain); err != nil {
			return AccountStatusFilter{}, err
		}

		// If there are non-core event types at this stage, it returns an error from `addCoreEventFieldFilter`.
		for eventType := range filter.EventTypes {
			err = filter.addCoreEventFieldFilter(eventType, address, chain)
			if err != nil {
				return AccountStatusFilter{}, err
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

// validateCoreEventTypes validates the provided event types against the default core event types.
// It returns an error if any of the provided event types are not in the default core event types list. Note, an empty
// event types array is also valid.
func validateCoreEventTypes(eventTypes []string) error {
	for _, eventType := range eventTypes {
		isMatch := false
		for _, coreEventType := range defaultCoreEvents {
			if coreEventType == eventType {
				isMatch = true
				break
			}
		}

		// If the provided event type does not match any of the default core event types, return an error
		if !isMatch {
			return fmt.Errorf("invalid provided event types for filter")
		}
	}

	return nil // All provided event types are valid core event types or event types are empty
}
