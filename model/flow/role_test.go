package flow_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func TestRoleYAML(t *testing.T) {
	r := flow.RoleCollection
	bz, err := yaml.Marshal(r)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%v\n", r), string(bz))
	var actual flow.Role
	err = yaml.Unmarshal(bz, &actual)
	assert.NoError(t, err)
	assert.Equal(t, r, actual)
}
