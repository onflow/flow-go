package common

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

func ParseOwners(hexAddresses []string) (map[string]struct{}, error) {
	if len(hexAddresses) == 0 {
		return nil, fmt.Errorf("at least one address must be provided")
	}

	addresses := make(map[string]struct{}, len(hexAddresses))
	for _, hexAddr := range hexAddresses {
		hexAddr = strings.TrimSpace(hexAddr)

		if len(hexAddr) > 0 {
			addr, err := ParseAddress(hexAddr)
			if err != nil {
				return nil, err
			}

			addresses[string(addr[:])] = struct{}{}
		} else {
			// global registers has empty address
			addresses[""] = struct{}{}
		}
	}

	return addresses, nil
}

func ParseAddress(hexAddr string) (flow.Address, error) {
	b, err := hex.DecodeString(hexAddr)
	if err != nil {
		return flow.Address{}, fmt.Errorf(
			"address is not hex encoded %s: %w",
			strings.TrimSpace(hexAddr),
			err,
		)
	}

	return flow.BytesToAddress(b), nil
}
