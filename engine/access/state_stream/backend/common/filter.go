package common

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	modelEvents "github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
)

// FieldFilter represents a filter over specific Cadence event fields.
// The first key is the field name, the inner map is a set of accepted stringified values for that field.
// If any (field,value) pair matches, the event matches the filter.
// Example: {"address": {"0x1": {}, "0x2": {}}}
//
// This type is intentionally shared across state stream backends.
// Keep in sync with usages in events and account_statuses packages.
//
// NOTE: Values are compared using cadence.Value.String().
// For addresses ensure to provide hex with prefix to match Cadence address string format.
//
//   - event type specific ownership of field names lives with the caller.
//   - empty FieldFilter set means "match any value" for that field grouping logic may decide differently.
//
// In practice we treat an empty FieldFilter map as "no field filters".
// An empty inner map would never match as there is no accepted value.
//
// This mirrors the previous unexported definition that existed in events package.
// Moving it here allows sharing with account_statuses.
//
// See usages in events.EventFilter and account_statuses.AccountStatusFilter.

type FieldFilter map[string]map[string]struct{}

// GetEventFields extracts field values and field names from the payload of a flow event.
// It decodes the event payload into a Cadence event, retrieves the field values and fields, and returns them.
// Returns an error when payload cannot be decoded or fields are empty.
func GetEventFields(event *flow.Event) (map[string]cadence.Value, error) {
	data, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, err
	}

	cdcEvent, ok := data.(cadence.Event)
	if !ok {
		return nil, fmt.Errorf("decoded payload is not a cadence.Event")
	}

	fields := cadence.FieldsMappedByName(cdcEvent)
	if fields == nil {
		return nil, fmt.Errorf("fields are empty")
	}
	return fields, nil
}

// ValidateEventType ensures that the event type matches the expected format for the provided chain.
func ValidateEventType(eventType flow.EventType, chain flow.Chain) error {
	_, err := modelEvents.ValidateEvent(eventType, chain)
	if err != nil {
		return fmt.Errorf("invalid event type %s: %w", eventType, err)
	}
	return nil
}

// ValidateAddress ensures that the address is valid for the given chain.
func ValidateAddress(address flow.Address, chain flow.Chain) error {
	if !chain.IsValid(address) {
		return fmt.Errorf("invalid address for chain: %s", address)
	}
	return nil
}

// ValidateContract ensures that the contract is in the correct format.
// Valid contracts are either the literal "flow" or of the form "A.<address>.<name>" (3 parts, starting with 'A').
func ValidateContract(contract string) error {
	if contract == "flow" {
		return nil
	}
	parts := strings.Split(contract, ".")
	if len(parts) != 3 || parts[0] != "A" {
		return fmt.Errorf("invalid contract: %s", contract)
	}
	return nil
}
