package flow

import (
	"bytes"
	"fmt"
	"math"

	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/utils/rand"
)

// Notes on runtime EFFICIENCY of GENERIC TYPES:
// DO NOT pass an interface to a generic function (100x runtime cost as of go 1.20).
// For example, consider the function
//
//    func f[T GenericIdentity]()
//
// The call `f(identity)` is completely ok and doesn't introduce overhead when `identity` is a struct type,
// such as `var identity *flow.Identity`.
// In contrast `f(identity)` where identity is declared as an interface `var identity GenericIdentity` is drastically slower,
// since golang involves a global hash table lookup for every method call to dispatch the underlying type behind the interface.

// GenericIdentity defines a constraint for generic identities.
// Golang doesn't support constraint with fields(for time being) so we have to define this interface
// with getter methods.
// Details here: https://github.com/golang/go/issues/51259.
type GenericIdentity interface {
	Identity | IdentitySkeleton
	GetNodeID() Identifier
	GetRole() Role
	GetStakingPubKey() crypto.PublicKey
	GetNetworkPubKey() crypto.PublicKey
	GetInitialWeight() uint64
	GetSkeleton() IdentitySkeleton
}

// IdentityFilter is a filter on identities. Mathematically, an IdentityFilter F
// can be described as a function F: 𝓘 → 𝐼, where 𝓘 denotes the set of all identities
// and 𝐼 ⊆ 𝓘. For an input identity i, F(i) returns true if and only if i passed the
// filter, i.e. i ∈ 𝐼. Returning false means that some necessary criterion was violated
// and identity i should be dropped, i.e. i ∉ 𝐼.
type IdentityFilter[T GenericIdentity] func(*T) bool

// IdentityOrder is an order function for identities.
//
// It defines a strict weak ordering between identities.
// It returns a negative number if the first identity is "strictly less" than the second,
// a positive number if the second identity is "strictly less" than the first,
// and zero if the two identities are equal.
//
// `IdentityOrder` can be used to sort identities with
// https://pkg.go.dev/golang.org/x/exp/slices#SortFunc.
type IdentityOrder[T GenericIdentity] func(*T, *T) int

// IdentityMapFunc is a modifier function for map operations for identities.
// Identities are COPIED from the source slice.
type IdentityMapFunc[T GenericIdentity] func(T) T

// IdentitySkeletonList is a list of nodes skeletons. We use a type alias instead of defining a new type
// since go generics doesn't support implicit conversion between types.
type IdentitySkeletonList = GenericIdentityList[IdentitySkeleton]

// IdentityList is a list of nodes. We use a type alias instead of defining a new type
// since go generics doesn't support implicit conversion between types.
type IdentityList = GenericIdentityList[Identity]

type GenericIdentityList[T GenericIdentity] []*T

// Filter will apply a filter to the identity list.
// The resulting list will only contain entries that match the filtering criteria.
func (il GenericIdentityList[T]) Filter(filter IdentityFilter[T]) GenericIdentityList[T] {
	var dup GenericIdentityList[T]
	for _, identity := range il {
		if filter(identity) {
			dup = append(dup, identity)
		}
	}
	return dup
}

// Map returns a new identity list with the map function f applied to a copy of
// each identity.
//
// CAUTION: this relies on structure copy semantics. Map functions that modify
// an object referenced by the input Identity structure will modify identities
// in the source slice as well.
func (il GenericIdentityList[T]) Map(f IdentityMapFunc[T]) GenericIdentityList[T] {
	dup := make(GenericIdentityList[T], 0, len(il))
	for _, identity := range il {
		next := f(*identity)
		dup = append(dup, &next)
	}
	return dup
}

// Copy returns a copy of IdentityList. The resulting slice uses a different
// backing array, meaning appends and insert operations on either slice are
// guaranteed to only affect that slice.
//
// Copy should be used when modifying an existing identity list by either
// appending new elements, re-ordering, or inserting new elements in an
// existing index.
//
// CAUTION:
// All Identity fields are deep-copied, _except_ for their keys, which
// are copied by reference as they are treated as immutable by convention.
func (il GenericIdentityList[T]) Copy() GenericIdentityList[T] {
	dup := make(GenericIdentityList[T], 0, len(il))
	lenList := len(il)
	for i := 0; i < lenList; i++ { // performance tests show this is faster than 'range'
		next := *(il[i]) // copy the object
		dup = append(dup, &next)
	}
	return dup
}

// Selector returns an identity filter function that selects only identities
// within this identity list.
func (il GenericIdentityList[T]) Selector() IdentityFilter[T] {
	lookup := il.Lookup()
	return func(identity *T) bool {
		_, exists := lookup[(*identity).GetNodeID()]
		return exists
	}
}

// Lookup converts the identity slice to a map using the NodeIDs as keys. This
// is useful when _repeatedly_ querying identities by their NodeIDs. The
// conversation from slice to map incurs cost O(n), for `n` the slice length.
// For a _single_ lookup, use method `ByNodeID(Identifier)` (avoiding conversion).
func (il GenericIdentityList[T]) Lookup() map[Identifier]*T {
	lookup := make(map[Identifier]*T, len(il))
	for _, identity := range il {
		lookup[(*identity).GetNodeID()] = identity
	}
	return lookup
}

// Sort will sort the list using the given ordering.  This is
// not recommended for performance.  Expand the 'less' function
// in place for best performance, and don't use this function.
func (il GenericIdentityList[T]) Sort(less IdentityOrder[T]) GenericIdentityList[T] {
	dup := il.Copy()
	slices.SortFunc(dup, less)
	return dup
}

// Sorted returns whether the list is sorted by the input ordering.
func (il GenericIdentityList[T]) Sorted(less IdentityOrder[T]) bool {
	return slices.IsSortedFunc(il, less)
}

// NodeIDs returns the NodeIDs of the nodes in the list (order preserving).
func (il GenericIdentityList[T]) NodeIDs() IdentifierList {
	nodeIDs := make([]Identifier, 0, len(il))
	for _, id := range il {
		nodeIDs = append(nodeIDs, (*id).GetNodeID())
	}
	return nodeIDs
}

// PublicStakingKeys returns a list with the public staking keys (order preserving).
func (il GenericIdentityList[T]) PublicStakingKeys() []crypto.PublicKey {
	pks := make([]crypto.PublicKey, 0, len(il))
	for _, id := range il {
		pks = append(pks, (*id).GetStakingPubKey())
	}
	return pks
}

// ID uniquely identifies a list of identities, by node ID. This can be used
// to perpetually identify a group of nodes, even if mutable fields of some nodes
// are changed, as node IDs are immutable.
// CAUTION:
//   - An IdentityList's ID is a cryptographic commitment to only node IDs. A node operator
//     can freely choose the ID for their node. There is no relationship whatsoever between
//     a node's ID and keys.
//   - To generate a cryptographic commitment for the full IdentityList, use method `Checksum()`.
//   - The outputs of `IdentityList.ID()` and `IdentityList.Checksum()` are both order-sensitive.
//     Therefore, the `IdentityList` must be in canonical order, unless explicitly specified
//     otherwise by the protocol.
func (il GenericIdentityList[T]) ID() Identifier {
	return il.NodeIDs().ID()
}

// Checksum generates a cryptographic commitment to the full IdentityList, including mutable fields.
// The checksum for the same group of identities (by NodeID) may change from block to block.
func (il GenericIdentityList[T]) Checksum() Identifier {
	return MakeID(il)
}

// TotalWeight returns the total weight of all given identities.
func (il GenericIdentityList[T]) TotalWeight() uint64 {
	var total uint64
	for _, identity := range il {
		total += (*identity).GetInitialWeight()
	}
	return total
}

// Count returns the count of identities.
func (il GenericIdentityList[T]) Count() uint {
	return uint(len(il))
}

// ByIndex returns the node at the given index.
func (il GenericIdentityList[T]) ByIndex(index uint) (*T, bool) {
	if index >= uint(len(il)) {
		return nil, false
	}
	return il[int(index)], true
}

// ByNodeID gets a node from the list by node ID.
func (il GenericIdentityList[T]) ByNodeID(nodeID Identifier) (*T, bool) {
	for _, identity := range il {
		if (*identity).GetNodeID() == nodeID {
			return identity, true
		}
	}
	return nil, false
}

// ByNetworkingKey gets a node from the list by network public key.
func (il GenericIdentityList[T]) ByNetworkingKey(key crypto.PublicKey) (*T, bool) {
	for _, identity := range il {
		if (*identity).GetNetworkPubKey().Equals(key) {
			return identity, true
		}
	}
	return nil, false
}

// Sample returns non-deterministic random sample from the `IdentityList`
func (il GenericIdentityList[T]) Sample(size uint) (GenericIdentityList[T], error) {
	n := uint(len(il))
	dup := make(GenericIdentityList[T], 0, n)
	dup = append(dup, il...)
	if n < size {
		size = n
	}
	swap := func(i, j uint) {
		dup[i], dup[j] = dup[j], dup[i]
	}
	err := rand.Samples(n, size, swap)
	if err != nil {
		return nil, fmt.Errorf("failed to sample identity list: %w", err)
	}
	return dup[:size], nil
}

// Shuffle randomly shuffles the identity list (non-deterministic),
// and returns the shuffled list without modifying the receiver.
func (il GenericIdentityList[T]) Shuffle() (GenericIdentityList[T], error) {
	return il.Sample(uint(len(il)))
}

// SamplePct returns a random sample from the receiver identity list. The
// sample contains `pct` percentage of the list. The sample is rounded up
// if `pct>0`, so this will always select at least one identity.
//
// NOTE: The input must be between in the interval [0, 1.0]
func (il GenericIdentityList[T]) SamplePct(pct float64) (GenericIdentityList[T], error) {
	if pct <= 0 {
		return GenericIdentityList[T]{}, nil
	}

	count := float64(il.Count()) * pct
	size := uint(math.Round(count))
	// ensure we always select at least 1, for non-zero input
	if size == 0 {
		size = 1
	}

	return il.Sample(size)
}

// Union returns a new identity list containing every identity that occurs in
// either `il`, or `other`, or both. There are no duplicates in the output,
// where duplicates are identities with the same node ID. In case an entry
// with the same NodeID exists in the receiver `il` as well as in `other`,
// the identity from `il` is included in the output.
// Receiver `il` and/or method input `other` can be nil or empty.
// The returned IdentityList is sorted in canonical order.
func (il GenericIdentityList[T]) Union(other GenericIdentityList[T]) GenericIdentityList[T] {
	maxLen := len(il) + len(other)

	union := make(GenericIdentityList[T], 0, maxLen)
	set := make(map[Identifier]struct{}, maxLen)

	for _, list := range []GenericIdentityList[T]{il, other} {
		for _, id := range list {
			if _, isDuplicate := set[(*id).GetNodeID()]; !isDuplicate {
				set[(*id).GetNodeID()] = struct{}{}
				union = append(union, id)
			}
		}
	}

	slices.SortFunc(union, Canonical[T])
	return union
}

// IdentityListEqualTo checks if the other list if the same, that it contains the same elements
// in the same order.
// NOTE: currently a generic comparison is not possible, so we have to use a specific function.
func IdentityListEqualTo(lhs, rhs IdentityList) bool {
	return slices.EqualFunc(lhs, rhs, func(a, b *Identity) bool {
		return a.EqualTo(b)
	})
}

// IdentitySkeletonListEqualTo checks if the other list if the same, that it contains the same elements
// in the same order.
// NOTE: currently a generic comparison is not possible, so we have to use a specific function.
func IdentitySkeletonListEqualTo(lhs, rhs IdentitySkeletonList) bool {
	return slices.EqualFunc(lhs, rhs, func(a, b *IdentitySkeleton) bool {
		return a.EqualTo(b)
	})
}

// Exists takes a previously sorted Identity list and searches it for the target
// identity by its NodeID.
// CAUTION:
//   - Other identity fields are not compared.
//   - The identity list MUST be sorted prior to calling this method.
func (il GenericIdentityList[T]) Exists(target *T) bool {
	return il.IdentifierExists((*target).GetNodeID())
}

// IdentifierExists takes a previously sorted Identity list and searches it for the target value
// target:  value to search for
// CAUTION:  The identity list MUST be sorted prior to calling this method
func (il GenericIdentityList[T]) IdentifierExists(target Identifier) bool {
	_, ok := slices.BinarySearchFunc(il, target, func(a *T, b Identifier) int {
		lhs := (*a).GetNodeID()
		return bytes.Compare(lhs[:], b[:])
	})
	return ok
}

// GetIndex returns the index of the identifier in the IdentityList and true
// if the identifier is found.
func (il GenericIdentityList[T]) GetIndex(target Identifier) (uint, bool) {
	i := slices.IndexFunc(il, func(a *T) bool {
		return (*a).GetNodeID() == target
	})
	if i == -1 {
		return 0, false
	}
	return uint(i), true
}

// ToSkeleton converts the identity list to a list of identity skeletons.
func (il GenericIdentityList[T]) ToSkeleton() IdentitySkeletonList {
	skeletons := make(IdentitySkeletonList, len(il))
	for i, id := range il {
		v := (*id).GetSkeleton()
		skeletons[i] = &v
	}
	return skeletons
}
