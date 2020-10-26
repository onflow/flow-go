package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// TestGetRolesByTopic evaluates correctness of GetRoleByTopic function against
// inclusion and exclusion of roles. Essentially, the test evaluates that RolesByChannelID
// operates on top of channelIdMap.
func TestGetRolesByChannelID(t *testing.T) {
	// asserts existing topic with its role
	// the roles list should contain collection and consensus roles
	roles, ok := engine.RolesByChannelID(engine.PushGuarantees)
	assert.True(t, ok)
	assert.Len(t, roles, 2)
	assert.Contains(t, roles, flow.RoleConsensus)
	assert.Contains(t, roles, flow.RoleCollection)
	assert.NotContains(t, roles, flow.RoleExecution)
	assert.NotContains(t, roles, flow.RoleVerification)
	assert.NotContains(t, roles, flow.RoleAccess)

	// asserts a non-existing topic
	roles, ok = engine.RolesByChannelID("non-existing-topic")
	assert.False(t, ok)
	assert.Nil(t, roles)
}

// TestGetChannelIDByRole evaluates retrieving channel IDs associated with a role from the
// channel IDs map using ChannelIDsByRole. Essentially it evaluates that ChannelIDsByRole
// operates on top of channelIDMap.
func TestGetChannelIDByRole(t *testing.T) {
	// asserts topics by the role for verification node
	// it should have the topics of
	// - PushBlocks
	// - PushReceipts
	// - PushApprovals
	// - ProvideChunks
	// - TestNetwork
	// - TestMetric
	// the roles list should contain collection and consensus roles
	topics := engine.ChannelIDsByRole(flow.RoleVerification)
	assert.Len(t, topics, 6)
	assert.Contains(t, topics, engine.PushBlocks)
	assert.Contains(t, topics, engine.PushReceipts)
	assert.Contains(t, topics, engine.PushApprovals)
	assert.Contains(t, topics, engine.RequestChunks)
	assert.Contains(t, topics, engine.TestMetrics)
	assert.Contains(t, topics, engine.TestNetwork)
}
