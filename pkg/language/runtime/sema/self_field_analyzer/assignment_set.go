package self_field_analyzer

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/raviqqe/hamt"
)

// AssignmentSet is an immutable set of field assignments.
type AssignmentSet struct {
	set hamt.Set
}

// NewAssignmentSet returns an empty assignment set.
func NewAssignmentSet() AssignmentSet {
	return AssignmentSet{hamt.NewSet()}
}

// Insert inserts an identifier into the set.
func (a AssignmentSet) Insert(identifier ast.Identifier) AssignmentSet {
	return AssignmentSet{a.set.Insert(common.StringKey(identifier.Identifier))}
}

// Contains returns true if the given identifier exists in the set.
func (a AssignmentSet) Contains(identifier ast.Identifier) bool {
	return a.set.Include(common.StringKey(identifier.Identifier))
}

// Intersection returns a new set containing all fields that exist in both sets.
func (a AssignmentSet) Intersection(b AssignmentSet) AssignmentSet {
	c := hamt.NewSet()

	set := a.set

	for set.Size() != 0 {
		var e hamt.Entry
		e, set = set.FirstRest()

		if b.set.Include(e) {
			c = c.Insert(e)
		}
	}

	return AssignmentSet{c}
}
