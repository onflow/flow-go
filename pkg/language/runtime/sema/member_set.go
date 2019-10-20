package sema

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/common/interface_entry"
)

// MemberSet is an immutable set of field assignments.
//
type MemberSet struct {
	set hamt.Set
}

// NewMemberSet returns an empty member set.
//
func NewMemberSet() *MemberSet {
	return &MemberSet{hamt.NewSet()}
}

func (ms *MemberSet) entry(member *Member) hamt.Entry {
	return interface_entry.InterfaceEntry{Interface: member}
}

// Add inserts a member into the set.
//
func (ms *MemberSet) Add(member *Member) {
	entry := ms.entry(member)
	ms.set = ms.set.Insert(entry)
}

// Contains returns true if the given member exists in the set.
//
func (ms *MemberSet) Contains(member *Member) bool {
	entry := ms.entry(member)
	return ms.set.Include(entry)
}

// Intersection returns a new set containing all members that exist in both sets.
//
func (ms *MemberSet) Intersection(b *MemberSet) *MemberSet {
	result := hamt.NewSet()

	set := ms.set

	for set.Size() != 0 {
		var entry hamt.Entry
		entry, set = set.FirstRest()

		if b.set.Include(entry) {
			result = result.Insert(entry)
		}
	}

	return &MemberSet{result}
}

func (ms *MemberSet) Clone() *MemberSet {
	return &MemberSet{set: ms.set}
}
