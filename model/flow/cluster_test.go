package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test that converting between assignments and clusters does not change contents or order.
func TestClusterAssignments(t *testing.T) {

	identities := unittest.IdentityListFixture(100, unittest.WithRole(flow.RoleCollection))
	assignments := unittest.ClusterAssignment(10, identities)
	assert.Len(t, assignments, 10)

	clusters, err := flow.NewClusterList(assignments, identities)
	require.NoError(t, err)
	assert.Equal(t, assignments, clusters.Assignments())
}

func TestAssignmentList_EqualTo(t *testing.T) {

	t.Run("empty are equal", func(t *testing.T) {
		a := flow.AssignmentList{}
		b := flow.AssignmentList{}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("different length arent equal", func(t *testing.T) {
		a := flow.AssignmentList{}
		b := flow.AssignmentList{[]flow.Identifier{[32]byte{}}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("different nested length arent equal", func(t *testing.T) {
		a := flow.AssignmentList{[]flow.Identifier{[32]byte{}}}
		b := flow.AssignmentList{[]flow.Identifier{[32]byte{}, [32]byte{}}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("equal length with wrong data means not equal", func(t *testing.T) {
		a := flow.AssignmentList{[]flow.Identifier{[32]byte{1, 2, 3}, [32]byte{2, 2, 2}}}
		b := flow.AssignmentList{[]flow.Identifier{[32]byte{1, 2, 3}, [32]byte{}}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("equal length with same data means equal", func(t *testing.T) {
		a := flow.AssignmentList{[]flow.Identifier{[32]byte{1, 2, 3}, [32]byte{2, 2, 2}}}
		b := flow.AssignmentList{[]flow.Identifier{[32]byte{1, 2, 3}, [32]byte{2, 2, 2}}}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}
