package order

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
)

func IdentifierCanonical(id1 flow.Identifier, id2 flow.Identifier) bool {
	return bytes.Compare(id1[:], id2[:]) < 0
}
