package transfers

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/model/flow"
)

// DecodeEvent decodes the CCF payload of a [flow.Event] into a [cadence.Event].
//
// Any error indicates that the event payload is malformed.
func DecodeEvent(event flow.Event) (cadence.Event, error) {
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

// addressFromOptional extracts an address from a [cadence.Optional] value.
// Returns a zero address if the optional is empty (nil value).
//
// Any error indicates that the provided optional value is not a valid address.
func addressFromOptional(opt cadence.Optional) (flow.Address, error) {
	if opt.Value == nil {
		return flow.Address{}, nil
	}

	addr, ok := opt.Value.(cadence.Address)
	if !ok {
		return flow.Address{}, fmt.Errorf("unexpected type in optional address field: %T", opt.Value)
	}

	return flow.BytesToAddress(addr.Bytes()), nil
}
