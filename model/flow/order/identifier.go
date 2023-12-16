package order

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierCanonical is a function that defines a weak strict ordering "<" for identifiers.
// It returns:
//   - a strict negative number if id1 < id2
//   - a strict positive number if id2 < id1
//   - zero if id1 and id2 are equal
//
// The current function is based on the identifiers bytes comparison.
// Example:
//
//	IdentifierCanonical(flow.Identifier{1}, flow.Identifier{2}) // -1
//	IdentifierCanonical(flow.Identifier{2}, flow.Identifier{1}) // 1
//	IdentifierCanonical(flow.Identifier{1}, flow.Identifier{1}) // 0
//	IdentifierCanonical(flow.Identifier{0, 1}, flow.Identifier{0, 2}) // -1
func IdentifierCanonical(id1 flow.Identifier, id2 flow.Identifier) int {
	return bytes.Compare(id1[:], id2[:])
}
