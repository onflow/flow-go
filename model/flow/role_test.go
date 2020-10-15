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
