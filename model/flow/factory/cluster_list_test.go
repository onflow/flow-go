package factory_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewClusterList ensures that implementation enforces the following protocol rules in case they are violated:
//
//	(a) input `collectors` only contains collector nodes with positive weight
//	(b) collectors have unique node IDs
//	(c) each collector is assigned exactly to one cluster and is only listed once within that cluster
//	(d) cluster contains at least one collector (i.e. is not empty)
//	(e) cluster is composed of known nodes, i.e. for each nodeID in `assignments` an IdentitySkeleton is given in `collectors`
//	(f) cluster assignment lists the nodes in canonical ordering
func TestNewClusterList(t *testing.T) {
	identities := unittest.IdentityListFixture(100, unittest.WithRole(flow.RoleCollection))

	t.Run("valid inputs", func(t *testing.T) {
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.NoError(t, err)
	})
	t.Run("(a) input `collectors` only contains collector nodes with positive weight", func(t *testing.T) {
		identities := identities.Copy()
		identities[0].InitialWeight = 0
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
	t.Run("(b) collectors have unique node IDs", func(t *testing.T) {
		identities := identities.Copy()
		identities[0].NodeID = identities[1].NodeID
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
	t.Run("(c) each collector is assigned exactly to one cluster", func(t *testing.T) {
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		assignments[1][0] = assignments[0][0]
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
	t.Run("(c) each collector is only listed once within that cluster", func(t *testing.T) {
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		assignments[0][0] = assignments[0][1]
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
	t.Run("(d) cluster contains at least one collector (i.e. is not empty)", func(t *testing.T) {
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		assignments[0] = flow.IdentifierList{}
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
	t.Run("(e) cluster is composed of known nodes, i.e. for each nodeID in `assignments` an IdentitySkeleton is given in `collectors` ", func(t *testing.T) {
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		assignments[0][0] = unittest.IdentifierFixture()
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
	t.Run("(f) cluster assignment lists the nodes in canonical ordering", func(t *testing.T) {
		assignments := unittest.ClusterAssignment(10, identities.ToSkeleton())
		// sort in non-canonical order
		assignments[0] = assignments[0].Sort(func(lhs flow.Identifier, rhs flow.Identifier) int {
			return -flow.IdentifierCanonical(lhs, rhs)
		})
		_, err := factory.NewClusterList(assignments, identities.ToSkeleton())
		require.Error(t, err)
	})
}
