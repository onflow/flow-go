package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

var addressRegex = regexp.MustCompile(`^[0-9a-fA-F]{16}$`)

func ParseAddress(raw string, chain flow.Chain) (flow.Address, error) {
	raw = strings.TrimPrefix(raw, "0x") // remove 0x prefix

	if !addressRegex.MatchString(raw) {
		return flow.EmptyAddress, fmt.Errorf("invalid address")
	}

	address, err := convert.HexToAddress(raw, chain)
	if err != nil {
		return flow.EmptyAddress, err
	}

	return address, nil
}
