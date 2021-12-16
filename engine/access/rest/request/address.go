package request

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"regexp"
)

type Address flow.Address

func (a *Address) Parse(raw string) error {
	if raw == "" { // allow empty
		return nil
	}

	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{16}$`, raw)
	if !valid {
		return fmt.Errorf("invalid address")
	}

	*a = Address(flow.HexToAddress(raw))
	return nil
}

func (a Address) Flow() flow.Address {
	return flow.Address(a)
}
