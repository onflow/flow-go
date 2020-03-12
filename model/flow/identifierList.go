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

// With returns a new list that includes the given IDs. If the receiver list
// already contains any IDs, they are not not re-added.
func (il IdentifierList) With(ids ...Identifier) IdentifierList {

	lookup := make(map[Identifier]struct{})
	for _, id := range ids {
		lookup[id] = struct{}{}
	}

	dupes := make(map[Identifier]struct{})

	// find all IDs that already exist in the list
	for _, checkID := range il {
		if _, exists := lookup[checkID]; exists {
			dupes[checkID] = struct{}{}
		}
	}

	// add any IDs that don't already exist to the list
	for id := range lookup {
		if _, exists := dupes[id]; exists {
			continue
		}
		il = append(il, id)
	}

	return il
}

// Without returns a new list with all instances of `id` removed, without
// changing ordering.
func (il IdentifierList) Without(ids ...Identifier) IdentifierList {

	lookup := make(map[Identifier]struct{})
	for _, id := range ids {
		lookup[id] = struct{}{}
	}

	// keep track of the indices to remove
	toRemove := make(map[Identifier]struct{})
	for _, id := range il {
		if _, exists := lookup[id]; exists {
			toRemove[id] = struct{}{}
		}
	}

	// create a new output list to avoid modifying the receiver
	without := make(IdentifierList, 0, len(il)-len(toRemove))

	// go through the list of indices and remove each one from the list
	for _, id := range il {
		if _, skip := toRemove[id]; skip {
			continue
		}
		without = append(without, id)
	}

	return without
}

// RandSubsetN returns a list containing n identifiers randomly selected from the
// receiver list L. If n>len(L), returns L.
func (il IdentifierList) RandSubsetN(n int) IdentifierList {

	if n <= 0 {
		return IdentifierList{}
	}
	if n > len(il) {
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

// Join appends the input to the receiver and returns the result.
func (il IdentifierList) Join(other IdentifierList) IdentifierList {
	joined := make([]Identifier, 0, len(il)+len(other))
	for _, id := range il {
		joined = append(joined, id)
	}

	for _, id := range other {
		joined = append(joined, id)
	}

	return joined
}
