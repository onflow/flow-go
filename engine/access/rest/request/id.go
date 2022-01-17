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
		*i = ID(flow.ZeroID)
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

	// make a map to have only unique values as keys
	uniqueIDs := make(map[ID]bool)
	for _, r := range raw {
		var id ID
		err := id.Parse(r)
		if err != nil {
			return err
		}

		uniqueIDs[id] = true
	}

	ids := make([]ID, 0)
	for id := range uniqueIDs {
		ids = append(ids, id)
	}

	*i = ids
	return nil
}

func (i IDs) Flow() []flow.Identifier {
	ids := make([]flow.Identifier, len(i))
	for j, id := range i {
		ids[j] = id.Flow()
	}
	return ids
}
