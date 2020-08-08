/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sema

import (
	"github.com/raviqqe/hamt"

	interfaceentry "example.com/cadence-initial/runtime/common/interface_entry"
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
	return interfaceentry.InterfaceEntry{Interface: member}
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
