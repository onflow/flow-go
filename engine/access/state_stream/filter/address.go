package filter

import (
	"fmt"

	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
)

type AddressFilter struct {
	Addresses map[string]struct{}
}

var _ Matcher = (*AddressFilter)(nil)

func NewAddressFilter(addresses []string, chain flow.Chain) (*AddressFilter, error) {
	addressesMap := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		addr := flow.HexToAddress(address)
		if err := validateAddress(addr, chain); err != nil {
			return nil, err
		}
		// use the parsed address to make sure it will match the event address string exactly
		addressesMap[addr.String()] = struct{}{}
	}
	return &AddressFilter{Addresses: addressesMap}, nil
}

func (f *AddressFilter) Match(event *flow.Event) (bool, error) {
	parsed, err := events.ParseEvent(event.Type)
	if err != nil {
		return false, fmt.Errorf("error parsing event type: %w", err)
	}

	if parsed.Type != events.AccountEventType {
		return false, nil
	}

	_, ok := f.Addresses[parsed.Address]
	return ok, nil
}

// validateAddress ensures that the address is valid for the given chain
func validateAddress(address flow.Address, chain flow.Chain) error {
	if !chain.IsValid(address) {
		return fmt.Errorf("invalid address for chain: %s", address)
	}
	return nil
}
