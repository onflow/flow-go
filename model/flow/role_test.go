package flow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
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
