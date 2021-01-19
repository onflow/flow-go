package flow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func TestRoleJSON(t *testing.T) {
	r := flow.RoleCollection
	bz, err := json.Marshal(r)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("\"%v\"", r), string(bz))
	var actual flow.Role
	err = json.Unmarshal(bz, &actual)
	assert.NoError(t, err)
	assert.Equal(t, r, actual)
}

// TestRoleList_Contains evaluates correctness of Contains method of RoleList.
func TestRoleList_Contains(t *testing.T) {
	roleList := flow.RoleList{flow.RoleConsensus, flow.RoleVerification}

	// asserts Contains returns true for roles in the list
	assert.True(t, roleList.Contains(flow.RoleConsensus))
	assert.True(t, roleList.Contains(flow.RoleVerification))

	// asserts Contains returns false for roles not in the list
	assert.False(t, roleList.Contains(flow.RoleAccess))
	assert.False(t, roleList.Contains(flow.RoleExecution))
	assert.False(t, roleList.Contains(flow.RoleCollection))

}

// TestRoleList_Union evaluates correctness of Union method of RoleList.
func TestRoleList_Union(t *testing.T) {
	this := flow.RoleList{flow.RoleConsensus, flow.RoleVerification}
	other := flow.RoleList{flow.RoleConsensus, flow.RoleExecution}

	union := this.Union(other)

	// asserts length of role lists
	assert.Len(t, union, 3)
	assert.Len(t, this, 2)
	assert.Len(t, other, 2)

	// asserts content of role lists
	// this
	assert.Contains(t, this, flow.RoleConsensus)
	assert.Contains(t, this, flow.RoleVerification)
	// other
	assert.Contains(t, other, flow.RoleConsensus)
	assert.Contains(t, other, flow.RoleExecution)
	// union
	assert.Contains(t, union, flow.RoleConsensus)
	assert.Contains(t, union, flow.RoleVerification)
	assert.Contains(t, union, flow.RoleExecution)
}

// TestRoleList_ID evaluates that corresponding identifier of a role list is unique with its content, and
// does not depend on the order of its element
func TestRoleList_ID(t *testing.T) {
	// identifier of a role list should be the same as long as its element are the same
	// regardless of order of its elements
	this := flow.RoleList{flow.RoleConsensus, flow.RoleVerification}
	shuffled := flow.RoleList{flow.RoleVerification, flow.RoleConsensus}
	thisID := this.ID()
	shuffledID := shuffled.ID()
	assert.Equal(t, thisID, shuffledID)

	// lists with distinct elements should have distinct identifiers
	other := flow.RoleList{flow.RoleExecution, flow.RoleVerification}
	otherID := other.ID()
	assert.NotEqual(t, thisID, otherID)
}
