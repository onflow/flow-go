package order

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierCanonical is a function for sorting IdentifierList into canonical order.
// It returns true if id1 < id2, false otherwise.
// Args:
//
//	id1: flow.Identifier
//	id2: flow.Identifier
//
// Returns:
//
//	bool: true if id1 < id2, false otherwise.
//
// Example:
//
//	IdentifierCanonical(flow.Identifier{0x01}, flow.Identifier{0x02}) // true
//	IdentifierCanonical(flow.Identifier{0x02}, flow.Identifier{0x01}) // false
//	IdentifierCanonical(flow.Identifier{0x01}, flow.Identifier{0x01}) // false
func IdentifierCanonical(id1 flow.Identifier, id2 flow.Identifier) bool {
	return IdentifierCompare(id1, id2) < 0
}

// IdentifierCompare is a function for sorting IdentifierList into
// canonical order.
// It returns -1 if id1 < id2, 0 if id1 == id2, and 1 if id1 > id2.
// Args:
//
//	id1: flow.Identifier
//	id2: flow.Identifier
//
// Returns:
//
//	int: -1 if id1 < id2, 0 if id1 == id2, and 1 if id1 > id2.
//
// Example:
//
//	IdentifierCompare(flow.Identifier{0x01}, flow.Identifier{0x02}) // -1
//	IdentifierCompare(flow.Identifier{0x02}, flow.Identifier{0x01}) // 1
//	IdentifierCompare(flow.Identifier{0x01}, flow.Identifier{0x01}) // 0
func IdentifierCompare(id1 flow.Identifier, id2 flow.Identifier) int {
	return bytes.Compare(id1[:], id2[:])
}
