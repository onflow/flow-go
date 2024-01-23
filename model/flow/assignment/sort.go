package assignment

import (
	"github.com/onflow/flow-go/model/flow"
)

// FromIdentifierLists creates a `flow.AssignmentList` with canonical ordering from
// the given `identifierLists`.
func FromIdentifierLists(identifierLists []flow.IdentifierList) flow.AssignmentList {
	assignments := make(flow.AssignmentList, 0, len(identifierLists))
	// in place sort to order the assignment in canonical order
	for _, identities := range identifierLists {
		assignment := flow.IdentifierList(identities).Sort(flow.IdentifierCanonical)
		assignments = append(assignments, assignment)
	}
	return assignments
}
