package models

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"regexp"
)

type ID flow.Identifier

func (i *ID) Parse(raw string) error {
	if raw == "" { // allow empty
		return nil
	}

	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, raw)
	if !valid {
		return errors.New("invalid ID format")
	}

	flowID, err := flow.HexStringToIdentifier(raw)
	if err != nil {
		return fmt.Errorf("invalid ID: %w", err)
	}

	*i = ID(flowID)
	return nil
}
