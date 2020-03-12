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

// Add adds the given identifier to the list, if it doesn't already exist, and
// returns the list.
func (il *IdentifierList) Add(id Identifier) IdentifierList {
	if il == nil {
		*il = IdentifierList{}
	}

	for _, checkID := range *il {
		if checkID == id {
			return *il
		}
	}

	*il = append(*il, id)
	return *il
}

// Rem removes all instances of the identifier from the list, without
// changing ordering, and returns the list.
func (il *IdentifierList) Rem(id Identifier) IdentifierList {
	if il == nil {
		*il = IdentifierList{}
	}

	// keep track of the indices to remove
	var indices []int
	for i, checkID := range *il {
		if checkID == id {
			indices = append(indices, i)
		}
	}

	// go through the list of indices and remove each one from `il`
	for _, index := range indices {
		(*il)[index] = (*il)[len(*il)-1]
		*il = (*il)[:len(*il)-1]
	}

	return *il
}

// RandSubsetN returns a list containing n identifiers randomly selected from the
// receiver list L. If n<0 or n>len(L), returns L.
func (il IdentifierList) RandSubsetN(n int) IdentifierList {

	if n == 0 {
		return IdentifierList{}
	}
	if n < 0 || n > len(il) {
		return il
	}

	// create a copy of the list to pick random elements from
	src := make(IdentifierList, len(il))
	copy(src, il)

	out := make(IdentifierList, 0, n)
	for i := 0; i < n; i++ {
		// pick a random index
		index := rand.Intn(len(src))
		// add the identity at the index to the output
		out = append(out, src[index])
		// remove the selected identity to avoid duplicates
		src[index] = src[len(src)-1]
		src = src[:len(src)-1]
	}

	return out
}

// JoinIdentifierLists appends and returns two IdentifierLists
func JoinIdentifierLists(this, other IdentifierList) IdentifierList {
	joined := make([]Identifier, 0, len(this)+len(other))
	for _, id := range this {
		joined = append(joined, id)
	}

	for _, id := range other {
		joined = append(joined, id)
	}

	return joined
}
