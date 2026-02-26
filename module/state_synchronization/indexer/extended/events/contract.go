package events

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// AccountContractAddedEvent represents a decoded flow.AccountContractAdded event,
// emitted when a new contract is deployed to an account.
type AccountContractAddedEvent struct {
	// Address is the account address to which the contract was deployed.
	Address flow.Address
	// CodeHash is the 32-byte hash of the deployed contract code.
	CodeHash []byte
	// ContractName is the name of the deployed contract (e.g. "EVM").
	ContractName string
}

// AccountContractUpdatedEvent represents a decoded flow.AccountContractUpdated event,
// emitted when an existing contract on an account is updated.
type AccountContractUpdatedEvent struct {
	// Address is the account address whose contract was updated.
	Address flow.Address
	// CodeHash is the 32-byte hash of the updated contract code.
	CodeHash []byte
	// ContractName is the name of the updated contract.
	ContractName string
}

// AccountContractRemovedEvent represents a decoded flow.AccountContractRemoved event,
// emitted when a contract is removed from an account.
type AccountContractRemovedEvent struct {
	// Address is the account address from which the contract was removed.
	Address flow.Address
	// CodeHash is the 32-byte hash of the removed contract code.
	CodeHash []byte
	// ContractName is the name of the removed contract.
	ContractName string
}

// DecodeAccountContractAdded extracts fields from a flow.AccountContractAdded event.
//
// Any error indicates that the event is malformed.
//
// No error returns are expected during normal operation.
func DecodeAccountContractAdded(event cadence.Event) (*AccountContractAddedEvent, error) {
	type raw struct {
		Address      cadence.Address `cadence:"address"`
		CodeHash     cadence.Value   `cadence:"codeHash"`
		ContractName string          `cadence:"contract"`
	}
	var r raw
	if err := cadence.DecodeFields(event, &r); err != nil {
		return nil, fmt.Errorf("failed to decode AccountContractAdded event: %w", err)
	}
	hash, err := decodeCodeHashValue(r.CodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AccountContractAdded 'codeHash' field: %w", err)
	}
	return &AccountContractAddedEvent{
		Address:      flow.Address(r.Address),
		CodeHash:     hash,
		ContractName: r.ContractName,
	}, nil
}

// DecodeAccountContractUpdated extracts fields from a flow.AccountContractUpdated event.
//
// Any error indicates that the event is malformed.
//
// No error returns are expected during normal operation.
func DecodeAccountContractUpdated(event cadence.Event) (*AccountContractUpdatedEvent, error) {
	type raw struct {
		Address      cadence.Address `cadence:"address"`
		CodeHash     cadence.Value   `cadence:"codeHash"`
		ContractName string          `cadence:"contract"`
	}
	var r raw
	if err := cadence.DecodeFields(event, &r); err != nil {
		return nil, fmt.Errorf("failed to decode AccountContractUpdated event: %w", err)
	}
	hash, err := decodeCodeHashValue(r.CodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AccountContractUpdated 'codeHash' field: %w", err)
	}
	return &AccountContractUpdatedEvent{
		Address:      flow.Address(r.Address),
		CodeHash:     hash,
		ContractName: r.ContractName,
	}, nil
}

// DecodeAccountContractRemoved extracts fields from a flow.AccountContractRemoved event.
//
// Any error indicates that the event is malformed.
//
// No error returns are expected during normal operation.
func DecodeAccountContractRemoved(event cadence.Event) (*AccountContractRemovedEvent, error) {
	type raw struct {
		Address      cadence.Address `cadence:"address"`
		CodeHash     cadence.Value   `cadence:"codeHash"`
		ContractName string          `cadence:"contract"`
	}
	var r raw
	if err := cadence.DecodeFields(event, &r); err != nil {
		return nil, fmt.Errorf("failed to decode AccountContractRemoved event: %w", err)
	}
	hash, err := decodeCodeHashValue(r.CodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AccountContractRemoved 'codeHash' field: %w", err)
	}
	return &AccountContractRemovedEvent{
		Address:      flow.Address(r.Address),
		CodeHash:     hash,
		ContractName: r.ContractName,
	}, nil
}

// ContractIDFromAddress constructs the canonical contract ID string from an address and contract name.
// Format: "A.{address_hex}.{name}"
func ContractIDFromAddress(address flow.Address, contractName string) string {
	return fmt.Sprintf("A.%s.%s", address.Hex(), contractName)
}

// decodeCodeHashValue extracts a []byte from a Cadence constant-sized array of UInt8 values
// ([UInt8; 32]). Returns an error if the value is not a cadence.Array or contains non-UInt8 elements.
//
// No error returns are expected during normal operation.
func decodeCodeHashValue(v cadence.Value) ([]byte, error) {
	arr, ok := v.(cadence.Array)
	if !ok {
		return nil, fmt.Errorf("expected cadence.Array, got %T", v)
	}
	result := make([]byte, len(arr.Values))
	for i, elem := range arr.Values {
		b, ok := elem.(cadence.UInt8)
		if !ok {
			return nil, fmt.Errorf("expected cadence.UInt8 at index %d, got %T", i, elem)
		}
		result[i] = byte(b)
	}
	return result, nil
}
