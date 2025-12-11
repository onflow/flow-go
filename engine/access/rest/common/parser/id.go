package parser

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/onflow/flow-go/model/flow"
)

const MaxIDsLength = 50

type ID flow.Identifier

func NewID(raw string) (ID, error) {
	if raw == "" {
		return ID(flow.ZeroID), nil
	}

	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, raw)
	if !valid {
		return ID(flow.Identifier{}), errors.New("invalid ID format")
	}

	flowID, err := flow.HexStringToIdentifier(raw)
	if err != nil {
		return ID(flow.Identifier{}), fmt.Errorf("invalid ID: %w", err)
	}

	return ID(flowID), nil
}

func (i ID) Flow() flow.Identifier {
	return flow.Identifier(i)
}

type IDs []ID

func NewIDs(raw []string) (IDs, error) {
	if len(raw) > MaxIDsLength {
		return nil, fmt.Errorf("at most %d IDs can be requested at a time", MaxIDsLength)
	}

	// make a map to have only unique values as keys
	ids := make(IDs, 0)
	uniqueIDs := make(map[string]bool)
	for _, r := range raw {
		id, err := NewID(r)
		if err != nil {
			return nil, err
		}

		if !uniqueIDs[id.Flow().String()] {
			uniqueIDs[id.Flow().String()] = true
			ids = append(ids, id)
		}
	}

	return ids, nil
}

func (i IDs) Flow() flow.IdentifierList {
	ids := make([]flow.Identifier, len(i))
	for j, id := range i {
		ids[j] = id.Flow()
	}
	return ids
}
