package request

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"regexp"
)

const maxIDsLength = 50

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

func (i ID) Flow() flow.Identifier {
	return flow.Identifier(i)
}

type IDs []ID

func (i *IDs) Parse(raw []string) error {
	if len(raw) > maxIDsLength {
		return fmt.Errorf("at most %d IDs can be requested at a time", maxIDsLength)
	}

	id := make([]ID, len(raw))
	for i, r := range raw {
		err := id[i].Parse(r)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i IDs) Flow() []flow.Identifier {
	ids := make([]flow.Identifier, len(i))
	for j, id := range i {
		ids[j] = id.Flow()
	}
	return ids
}
