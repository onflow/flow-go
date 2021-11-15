package models

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"regexp"
)

type ID struct {
	flow.Identifier
}

func IDFromRequest(id string) (flow.Identifier, error) {
	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, id)
	if !valid {
		return flow.Identifier{}, fmt.Errorf("invalid ID")
	}

	return flow.HexStringToIdentifier(id)
}

func (i *ID) ToFlow() flow.Identifier {
	return i.Identifier
}
