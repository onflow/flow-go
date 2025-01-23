package indexer

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

var (
	accountContractUpdated = flow.EventType(stdlib.AccountContractUpdatedEventType.ID())
)

// hasAuthorizedTransaction checks if the provided account was an authorizer in any of the transactions
// within the provided collections.
func hasAuthorizedTransaction(collections []*flow.Collection, address flow.Address) bool {
	for _, collection := range collections {
		for _, tx := range collection.Transactions {
			for _, authorizer := range tx.Authorizers {
				if authorizer == address {
					return true
				}
			}
		}
	}

	return false
}

// findContractUpdates returns a map of common.AddressLocation for all contracts updated within the
// provided events.
// No errors are expected during normal operation and indicate an invalid protocol event was encountered
func findContractUpdates(events []flow.Event) (map[common.Location]struct{}, error) {
	invalidatedPrograms := make(map[common.Location]struct{})
	for _, event := range events {
		if event.Type == accountContractUpdated {
			location, err := parseAccountContractUpdated(&event)
			if err != nil {
				return nil, fmt.Errorf("could not parse account contract updated event: %w", err)
			}
			invalidatedPrograms[location] = struct{}{}
		}
	}
	return invalidatedPrograms, nil
}

// parseAccountContractUpdated parses an account contract updated event and returns the address location.
// No errors are expected during normal operation and indicate an invalid protocol event was encountered
func parseAccountContractUpdated(event *flow.Event) (common.AddressLocation, error) {
	payload, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return common.AddressLocation{}, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	cdcEvent, ok := payload.(cadence.Event)
	if !ok {
		return common.AddressLocation{}, fmt.Errorf("invalid event payload type: %T", payload)
	}

	fields := cadence.FieldsMappedByName(cdcEvent)

	addressField := fields[stdlib.AccountEventAddressParameter.Identifier]
	address, ok := addressField.(cadence.Address)
	if !ok {
		return common.AddressLocation{}, fmt.Errorf("invalid Cadence type for address field: %T", addressField)
	}

	contractNameField := fields[stdlib.AccountEventContractParameter.Identifier]
	contractName, ok := contractNameField.(cadence.String)
	if !ok {
		return common.AddressLocation{}, fmt.Errorf(
			"invalid Cadence type for contract name field: %T",
			contractNameField,
		)
	}

	return common.NewAddressLocation(
		nil,
		common.Address(address),
		string(contractName),
	), nil
}

var _ derived.TransactionInvalidator = (*accessInvalidator)(nil)

// accessInvalidator is a derived.TransactionInvalidator that invalidates programs and meter param overrides.
type accessInvalidator struct {
	programs            *programInvalidator
	executionParameters *executionParametersInvalidator
}

func (inv *accessInvalidator) ProgramInvalidator() derived.ProgramInvalidator {
	return inv.programs
}

func (inv *accessInvalidator) ExecutionParametersInvalidator() derived.ExecutionParametersInvalidator {
	return inv.executionParameters
}

var _ derived.ProgramInvalidator = (*programInvalidator)(nil)

// programInvalidator is a derived.ProgramInvalidator that invalidates all programs or a specific set of programs.
// this is used to invalidate all programs who's code was updated in a specific block.
type programInvalidator struct {
	invalidateAll bool
	invalidated   map[common.Location]struct{}
}

func (inv *programInvalidator) ShouldInvalidateEntries() bool {
	return inv.invalidateAll
}

func (inv *programInvalidator) ShouldInvalidateEntry(location common.AddressLocation, _ *derived.Program, _ *snapshot.ExecutionSnapshot) bool {
	_, ok := inv.invalidated[location]
	return inv.invalidateAll || ok
}

var _ derived.ExecutionParametersInvalidator = (*executionParametersInvalidator)(nil)

// executionParametersInvalidator is a derived.ExecutionParametersInvalidator that invalidates meter param overrides and execution version.
type executionParametersInvalidator struct {
	invalidateAll bool
}

func (inv *executionParametersInvalidator) ShouldInvalidateEntries() bool {
	return inv.invalidateAll
}

func (inv *executionParametersInvalidator) ShouldInvalidateEntry(_ struct{}, _ derived.StateExecutionParameters, _ *snapshot.ExecutionSnapshot) bool {
	return inv.invalidateAll
}
