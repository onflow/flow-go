package flow

import (
	"github.com/onflow/flow-go/model/flow/order"
	"golang.org/x/exp/slices"
)

// IdentifierList defines a sortable list of identifiers
type IdentifierList []Identifier

// Len returns length of the IdentifierList in the number of stored identifiers.
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
	return order.IdentifierCanonical(il[i], il[j])
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

// ID returns a cryptographic commitment to the list of identifiers.
// Since an IdentifierList has no mutable fields, it is equal to the checksum.
func (il IdentifierList) ID() Identifier {
	return il.Checksum()
}

// Checksum returns a cryptographic commitment to the list of identifiers.
func (il IdentifierList) Checksum() Identifier {
	return MakeID(il)
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

// Sample returns random sample of length 'size' of the ids
func (il IdentifierList) Sample(size uint) (IdentifierList, error) {
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

// Sort returns a sorted _copy_ of the IdentifierList, leaving the original invariant.
func (il IdentifierList) Sort(less IdentifierOrder) IdentifierList {
	dup := il.Copy()
	slices.SortFunc(dup, less)
	return dup
}
