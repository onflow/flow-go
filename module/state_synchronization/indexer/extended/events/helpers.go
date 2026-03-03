package events

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/model/flow"
)

// DecodePayload decodes the CCF payload of a [flow.Event] into a [cadence.Event].
//
// Any error indicates that the event payload is malformed.
func DecodePayload(event flow.Event) (cadence.Event, error) {
	value, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return cadence.Event{}, fmt.Errorf("failed to decode CCF payload for %s: %w", event.Type, err)
	}

	cadenceEvent, ok := value.(cadence.Event)
	if !ok {
		return cadence.Event{}, fmt.Errorf("decoded value is not an event for %s: %T", event.Type, value)
	}

	return cadenceEvent, nil
}

// AddressFromOptional extracts an address from a [cadence.Optional] value.
// Returns a zero address if the optional is empty (nil value).
//
// Any error indicates that the provided optional value is not a valid address.
func AddressFromOptional(opt cadence.Optional) (flow.Address, error) {
	if opt.Value == nil {
		return flow.Address{}, nil
	}

	addr, ok := opt.Value.(cadence.Address)
	if !ok {
		return flow.Address{}, fmt.Errorf("unexpected type in optional address field: %T", opt.Value)
	}

	return flow.BytesToAddress(addr.Bytes()), nil
}

// HexToEVMAddress decodes a hex string to an EVM address.
// This is the same logic as `common.HexToAddress`, except it returns an error if the hex string is
// not valid hex or an incorrect length.
//
// Any error indicates that the hex string is malformed.
func HexToEVMAddress(hexStr string) (common.Address, error) {
	addrBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return common.Address{}, fmt.Errorf("invalid hex string: %w", err)
	}
	if len(addrBytes) != common.AddressLength {
		return common.Address{}, fmt.Errorf("invalid EVM address length: %d", len(addrBytes))
	}
	return common.BytesToAddress(addrBytes), nil
}
