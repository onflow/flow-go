// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"github.com/onflow/flow-go/model/flow"
)

// Canonical is a function that defines a weak strict ordering "<" for identities.
// It returns:
//   - a strict negative number if id1 < id2
//   - a strict positive number if id2 < id1
//   - zero if id1 and id2 are equal
//
// By definition, two identities (id1, id2) are in canonical order if id1's NodeID is lexicographically
// _strictly_ smaller than id2's NodeID. The strictness is important, meaning that identities
// with equal NodeIDs do not satisfy canonical ordering (order is irreflexive).
// Hence, only a returned strictly negative value means the pair is in canonical order.
// Use `IsCanonical` for canonical order checks.
//
// The current function is based on the identifiers bytes lexicographic comparison.
func Canonical(identity1 *flow.Identity, identity2 *flow.Identity) int {
	return IdentifierCanonical(identity1.NodeID, identity2.NodeID)
}

// IsCanonical returns true if and only if the given Identities are in canonical order.
//
// By convention, two Identities (i1, i2) are in canonical order if i1's NodeID bytes
// are lexicographically _strictly_ smaller than i2's NodeID bytes.
//
// The strictness is important, meaning that two identities with the same
// NodeID do not satisfy the canonical order.
// This also implies that the canonical order is irreflexive ((i,i) isn't in canonical order).
func IsCanonical(i1, i2 *flow.Identity) bool {
	return Canonical(i1, i2) < 0
}

// ByReferenceOrder return a function for sorting identities based on the order
// of the given nodeIDs
func ByReferenceOrder(nodeIDs []flow.Identifier) func(*flow.Identity, *flow.Identity) int {
	indices := make(map[flow.Identifier]int)
	for index, nodeID := range nodeIDs {
		_, ok := indices[nodeID]
		if ok {
			panic("should never order by reference order with duplicate node IDs")
		}
		indices[nodeID] = index
	}
	return func(identity1 *flow.Identity, identity2 *flow.Identity) int {
		return indices[identity1.NodeID] - indices[identity2.NodeID]
	}
}

// IdentityListCanonical takes a list of identities and
// checks if it's strictly sorted in canonical order.
// Strict sorting means that equality is not allowed.

// IdentityListCanonical returns true if and only if the given IdentityList is
// strictly sorted in the canonical order.
//
// The strictness is important here, meaning that a list with 2 successive entities
// with the same NodeID isn't considered to be sorted.
func IdentityListCanonical(il flow.IdentityList) bool {
	for i := 0; i < len(il)-1; i++ {
		if !IsCanonical(il[i], il[i+1]) {
			return false
		}
	}
	return true
}
