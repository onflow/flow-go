package request

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

func ParseAddress(raw string) (flow.Address, error) {
	raw = strings.ReplaceAll(raw, "0x", "") // remove 0x prefix

	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{16}$`, raw)
	if !valid {
		return flow.EmptyAddress, fmt.Errorf("invalid address")
	}

	return flow.HexToAddress(raw), nil
}
