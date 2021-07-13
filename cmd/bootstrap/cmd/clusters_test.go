package cmd

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterAssignment_Deterministic(t *testing.T) {

	partners := unittest.NodeInfosFixture(3, unittest.WithRole(flow.RoleCollection))
	internals := unittest.NodeInfosFixture(10, unittest.WithRole(flow.RoleCollection))
	seed := rand.Int63()

	assignments1, _ := constructClusterAssignment(partners, internals, seed)
	assignments2, _ := constructClusterAssignment(partners, internals, seed)
	assert.Equal(t, assignments1, assignments2)
}
