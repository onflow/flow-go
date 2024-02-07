package flow

import (
	"bytes"
)

// IdentifierCanonical is a function that defines a weak strict ordering "<" for identifiers.
// It returns:
//   - a strict negative number if id1 < id2
//   - a strict positive number if id2 < id1
//   - zero if id1 and id2 are equal
//
// By definition, two Identifiers (id1, id2) are in canonical order if id1 is lexicographically
// _strictly_ smaller than id2. The strictness is important, meaning that duplicates do not
// satisfy canonical ordering (order is irreflexive). Hence, only a returned strictly negative
// value means the pair is in canonical order.
// Use `IsIdentifierCanonical` for canonical order checks.
//
// The current function is based on the identifiers bytes lexicographic comparison.
// Example:
//
//	IdentifierCanonical(Identifier{1}, Identifier{2}) // -1
//	IdentifierCanonical(Identifier{2}, Identifier{1}) // 1
//	IdentifierCanonical(Identifier{1}, Identifier{1}) // 0
//	IdentifierCanonical(Identifier{0, 1}, Identifier{0, 2}) // -1
func IdentifierCanonical(id1 Identifier, id2 Identifier) int {
	return bytes.Compare(id1[:], id2[:])
}

// IsIdentifierCanonical returns true if and only if the given identifiers are in canonical order.
//
// By convention, two identifiers (i1, i2) are in canonical order if i1's bytes
// are lexicographically _strictly_ smaller than i2's bytes.
//
// The strictness is important, meaning that the canonical order
// is irreflexive ((i,i) isn't in canonical order).
func IsIdentifierCanonical(i1, i2 Identifier) bool {
	return IdentifierCanonical(i1, i2) < 0
}

// IsIdentifierListCanonical returns true if and only if the given list is
// _strictly_ sorted with regards to the canonical order.
//
// The strictness is important here, meaning that a list with 2 equal identifiers
// isn't considered well sorted.
func IsIdentifierListCanonical(il IdentifierList) bool {
	for i := 0; i < len(il)-1; i++ {
		if !IsIdentifierCanonical(il[i], il[i+1]) {
			return false
		}
	}
	return true
}
