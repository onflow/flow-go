package flow

import (
	"bytes"
	"math/rand"

	"github.com/rs/zerolog/log"
)

// IdentifierList defines a sortable list of identifiers
type IdentifierList []Identifier

// Len returns length of the IdentiferList in the number of stored identifiers.
// It satisfies the sort.Interface making the IdentifierList sortable.
func (il IdentifierList) Len() int {
	return len(il)
}

// Lookup converts the Identifiers to a lookup table.
func (il IdentifierList) Lookup() map[Identifier]struct{} {
	lookup := make(map[Identifier]struct{}, len(il))
	for _, id := range il {
		lookup[id] = struct{}{}
	}
	return lookup
}

// Less returns true if element i in the IdentifierList is less than j based on its identifier.
// Otherwise it returns true.
// It satisfies the sort.Interface making the IdentifierList sortable.
func (il IdentifierList) Less(i, j int) bool {
	// bytes package already implements Comparable for []byte.
	switch bytes.Compare(il[i][:], il[j][:]) {
	case -1:
		return true
	case 0, 1:
		return false
	default:
		log.Error().Msg("not fail-able with `bytes.Comparable` bounded [-1, 1].")
		return false
	}
}

// Swap swaps the element i and j in the IdentifierList.
// It satisfies the sort.Interface making the IdentifierList sortable.
func (il IdentifierList) Swap(i, j int) {
	il[j], il[i] = il[i], il[j]
}

func (il IdentifierList) Strings() []string {
	var list []string
	for _, id := range il {
		list = append(list, id.String())
	}

	return list
}

func (il IdentifierList) Copy() IdentifierList {
	cpy := make(IdentifierList, 0, il.Len())
	return append(cpy, il...)
}

// Contains returns whether this identifier list contains the target identifier.
func (il IdentifierList) Contains(target Identifier) bool {
	for _, id := range il {
		if target == id {
			return true
		}
	}
	return false
}

// Union returns a new identifier list containing the union of `il` and `other`.
// There are no duplicates in the output.
func (il IdentifierList) Union(other IdentifierList) IdentifierList {
	// stores the output, the union of the two lists
	union := make(IdentifierList, 0, len(il)+len(other))
	// efficient lookup to avoid duplicates
	lookup := make(map[Identifier]struct{})

	// add all identifiers, omitted duplicates
	for _, identifier := range append(il.Copy(), other...) {
		if _, exists := lookup[identifier]; exists {
			continue
		}
		union = append(union, identifier)
		lookup[identifier] = struct{}{}
	}

	return union
}

// DeterministicSample returns deterministic random sample from the `IdentifierList` using the given seed
func (il IdentifierList) DeterministicSample(size uint, seed int64) IdentifierList {
	rand.Seed(seed)
	return il.Sample(size)
}

// Sample returns random sample of length 'size' of the ids
// [Fisher-Yates shuffle](https://en.wikipedia.org/wiki/Fisher–Yates_shuffle).
func (il IdentifierList) Sample(size uint) IdentifierList {
	return Sample(size, il...)
}

// Filter will apply a filter to the identifier list.
func (il IdentifierList) Filter(filter IdentifierFilter) IdentifierList {
	var dup IdentifierList
IDLoop:
	for _, identifier := range il {
		if !filter(identifier) {
			continue IDLoop
		}
		dup = append(dup, identifier)
	}
	return dup
}
