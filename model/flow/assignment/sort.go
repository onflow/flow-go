package assignment

import (
	"github.com/onflow/flow-go/model/flow"
)

// FromIdentifierLists creates a `flow.AssignmentList` with canonical ordering from
// the given `identifierLists`.
func FromIdentifierLists(identifierLists []flow.IdentifierList) flow.AssignmentList {
	assignments := make(flow.AssignmentList, 0, len(identifierLists))
	for _, identities := range identifierLists {
		assignment := identities.Sort(flow.IdentifierCanonical) // sort each cluster in canonical order (already creates copy)
		assignments = append(assignments, assignment)
	}
	return assignments
}
