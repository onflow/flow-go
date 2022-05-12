package request

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

type Address flow.Address

func (a *Address) Parse(raw string) error {
	raw = strings.ReplaceAll(raw, "0x", "") // remove 0x prefix

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
