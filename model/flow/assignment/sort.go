package assignment

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
)

// FromIdentifierLists creates assignmentlist with canonical ordering from a list of IdentifierList.
func FromIdentifierLists(identifierLists []flow.IdentifierList) flow.AssignmentList {
	assignments := make(flow.AssignmentList, 0, len(identifierLists))
	// in place sort to order the assignment in canonical order
	for _, identities := range identifierLists {
		assignment := identities[:]
		flow.IdentifierList(assignment).Sort(order.IdentifierCanonical)
		assignments = append(assignments, assignment)
	}
	return assignments
}
