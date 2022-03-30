package assignment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/assignment"
)

// Check that FromIdentifierLists will sort the identifierList in canonical order
func TestSort(t *testing.T) {
	node1, err := flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	node2, err := flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
	require.NoError(t, err)
	node3, err := flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000003")
	require.NoError(t, err)

	unsorted := []flow.IdentifierList{flow.IdentifierList{node2, node1, node3}}

	assignments := assignment.FromIdentifierLists(unsorted)
	require.Len(t, assignments, 1)

	require.Equal(t, node1, assignments[0][0])
	require.Equal(t, node2, assignments[0][1])
	require.Equal(t, node3, assignments[0][2])
}
