package indexer

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

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
func findContractUpdates(events []flow.Event) (map[common.AddressLocation]struct{}, error) {
	invalidatedPrograms := make(map[common.AddressLocation]struct{})
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

	address, ok := cdcEvent.Fields[0].(cadence.Address)
	if !ok {
		return common.AddressLocation{}, fmt.Errorf("invalid cadence type for address: %T", cdcEvent.Fields[0])
	}

	contractName, ok := cdcEvent.Fields[2].(cadence.String)
	if !ok {
		return common.AddressLocation{}, fmt.Errorf("invalid cadence type for contract name: %T", cdcEvent.Fields[2])
	}

	return common.NewAddressLocation(nil, common.Address(address), contractName.String()), nil
}

var _ derived.TransactionInvalidator = (*accessInvalidator)(nil)

// accessInvalidator is a derived.TransactionInvalidator that invalidates programs and meter param overrides.
type accessInvalidator struct {
	programs            *programInvalidator
	meterParamOverrides *meterParamOverridesInvalidator
}

func (inv *accessInvalidator) ProgramInvalidator() derived.ProgramInvalidator {
	return inv.programs
}

func (inv *accessInvalidator) MeterParamOverridesInvalidator() derived.MeterParamOverridesInvalidator {
	return inv.meterParamOverrides
}

var _ derived.ProgramInvalidator = (*programInvalidator)(nil)

// programInvalidator is a derived.ProgramInvalidator that invalidates all programs or a specific set of programs.
// this is used to invalidate all programs who's code was updated in a specific block.
type programInvalidator struct {
	invalidateAll bool
	invalidated   map[common.AddressLocation]struct{}
}

func (inv *programInvalidator) ShouldInvalidateEntries() bool {
	return inv.invalidateAll
}

func (inv *programInvalidator) ShouldInvalidateEntry(location common.AddressLocation, _ *derived.Program, _ *snapshot.ExecutionSnapshot) bool {
	_, ok := inv.invalidated[location]
	return inv.invalidateAll || ok
}

var _ derived.MeterParamOverridesInvalidator = (*meterParamOverridesInvalidator)(nil)

// meterParamOverridesInvalidator is a derived.MeterParamOverridesInvalidator that invalidates meter param overrides.
type meterParamOverridesInvalidator struct {
	invalidateAll bool
}

func (inv *meterParamOverridesInvalidator) ShouldInvalidateEntries() bool {
	return inv.invalidateAll
}

func (inv *meterParamOverridesInvalidator) ShouldInvalidateEntry(_ struct{}, _ derived.MeterParamOverrides, _ *snapshot.ExecutionSnapshot) bool {
	return inv.invalidateAll
}
