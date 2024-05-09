package request

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

func ParseAddress(raw string, chain flow.Chain) (flow.Address, error) {
	raw = strings.ReplaceAll(raw, "0x", "") // remove 0x prefix

	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{16}$`, raw)
	if !valid {
		return flow.EmptyAddress, fmt.Errorf("invalid address")
	}

	address, err := convert.HexToAddress(raw, chain)
	if err != nil {
		return flow.EmptyAddress, err
	}

	return address, nil
}
