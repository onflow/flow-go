package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestList_WithAllRoles tests that the WithAllRoles and WithAllRolesExcept methods work as expected.
func TestList_WithAllRoles(t *testing.T) {
	suite := NewGeneratorSuite()

	g := suite.Identities()

	t.Run("Fixture: WithAllRoles uses first role", func(t *testing.T) {
		identities := g.Fixture(g.WithAllRoles())

		expected := flow.Roles()[0]
		assert.Equal(t, expected, identities.Role)
	})

	t.Run("Fixture: WithAllRolesExcept skips omitted role", func(t *testing.T) {
		identities := g.Fixture(g.WithAllRolesExcept(flow.RoleCollection))

		expected := flow.Roles()[1] // index 0 is collection
		assert.Equal(t, expected, identities.Role)
	})

	t.Run("List: WithAllRoles returns all roles", func(t *testing.T) {
		identities := g.List(5, g.WithAllRoles())
		require.Len(t, identities, len(flow.Roles()))

		for i, role := range flow.Roles() {
			require.Equal(t, role, identities[i].Role)
		}
	})

	t.Run("List: WithAllRolesExcept returns all roles except the ones specified", func(t *testing.T) {
		identities := g.List(4, g.WithAllRolesExcept(flow.RoleCollection))
		require.Len(t, identities, len(flow.Roles())-1)

		i := 0
		for _, role := range flow.Roles() {
			if role != flow.RoleCollection {
				assert.Equal(t, role, identities[i].Role)
				i++
			}
		}
	})

	t.Run("List: WithAllRoles cycles through roles when more than 5 are requested", func(t *testing.T) {
		expected := make([]flow.Role, 0, 20)
		for len(expected) < 18 {
			expected = append(expected, flow.Roles()...)
		}

		identities := g.List(18, g.WithAllRoles())
		require.Len(t, identities, 18)

		for i, role := range expected[:18] {
			assert.Equal(t, role, identities[i].Role)
		}
	})
}
