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
