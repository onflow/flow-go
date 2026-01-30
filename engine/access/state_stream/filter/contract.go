package filter

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
)

type ContractFilter struct {
	Contracts map[string]struct{}
}

var _ Matcher = (*ContractFilter)(nil)

func NewContractFilter(contracts []string) (*ContractFilter, error) {
	contractsMap := make(map[string]struct{}, len(contracts))
	for _, contract := range contracts {
		if err := validateContract(contract); err != nil {
			return nil, err
		}
		contractsMap[contract] = struct{}{}
	}
	return &ContractFilter{Contracts: contractsMap}, nil
}

func (f *ContractFilter) Match(event *flow.Event) (bool, error) {
	parsed, err := events.ParseEvent(event.Type)
	if err != nil {
		return false, fmt.Errorf("error parsing event type: %w", err)
	}

	if _, ok := f.Contracts[parsed.Contract]; ok {
		return true, nil
	}

	return false, nil
}

// validateContract ensures that the contract is in the correct format
func validateContract(contract string) error {
	if contract == "flow" {
		return nil
	}

	parts := strings.Split(contract, ".")
	if len(parts) != 3 || parts[0] != "A" {
		return fmt.Errorf("invalid contract: %s", contract)
	}
	return nil
}
