package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestGetRolesByTopic evaluates correctness of GetRoleByTopic function against
// inclusion and exclusion of roles. Essentially, the test evaluates that RolesByChannelID
// operates on top of channelIdMap.
func TestGetRolesByChannelID(t *testing.T) {
	// asserts existing topic with its role
	// the roles list should contain collection and consensus roles
	roles, ok := RolesByChannelID(PushGuarantees)
	assert.True(t, ok)
	assert.Len(t, roles, 2)
	assert.Contains(t, roles, flow.RoleConsensus)
	assert.Contains(t, roles, flow.RoleCollection)
	assert.NotContains(t, roles, flow.RoleExecution)
	assert.NotContains(t, roles, flow.RoleVerification)
	assert.NotContains(t, roles, flow.RoleAccess)

	// asserts a non-existing topic
	roles, ok = RolesByChannelID("non-existing-topic")
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
	topics := ChannelIDsByRole(flow.RoleVerification)
	assert.Len(t, topics, 6)
	assert.Contains(t, topics, PushBlocks)
	assert.Contains(t, topics, PushReceipts)
	assert.Contains(t, topics, PushApprovals)
	assert.Contains(t, topics, RequestChunks)
	assert.Contains(t, topics, TestMetrics)
	assert.Contains(t, topics, TestNetwork)
}

// TestIsClusterChannelID verifies the correctness of IsClusterChannelID method
// against cluster and non-cluster channel ids.
func TestIsClusterChannelID(t *testing.T) {
	// creates a consensus cluster channel and verifies it
	conClusterChannel := ChannelConsensusCluster("some-consensus-cluster-id")
	clusterChannelID, ok := IsClusterChannelID(conClusterChannel)
	require.True(t, ok)
	require.Equal(t, clusterChannelID, consensusClusterPrefix)

	// creates a sync cluster channel and verifies it
	syncClusterID := ChannelSyncCluster("some-sync-cluster-id")
	clusterChannelID, ok = IsClusterChannelID(syncClusterID)
	require.True(t, ok)
	require.Equal(t, clusterChannelID, syncClusterPrefix)

	// non-cluster channel should not be verified
	clusterChannelID, ok = IsClusterChannelID("non-cluster-channel-id")
	require.False(t, ok)
	require.Empty(t, clusterChannelID)

}
