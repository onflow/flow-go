package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestGetRolesByChannelID_NonClusterChannelID evaluates correctness of GetRoleByChannelID function against
// inclusion and exclusion of roles. Essentially, the test evaluates that RolesByChannel
// operates on top of channelRoleMap.
func TestGetRolesByChannelID_NonClusterChannelID(t *testing.T) {
	// asserts existing topic with its role
	// the roles list should contain collection and consensus roles
	roles, ok := RolesByChannel(PushGuarantees)
	assert.True(t, ok)
	assert.Len(t, roles, 2)
	assert.Contains(t, roles, flow.RoleConsensus)
	assert.Contains(t, roles, flow.RoleCollection)
	assert.NotContains(t, roles, flow.RoleExecution)
	assert.NotContains(t, roles, flow.RoleVerification)
	assert.NotContains(t, roles, flow.RoleAccess)

	// asserts a non-existing topic
	roles, ok = RolesByChannel("non-existing-topic")
	assert.False(t, ok)
	assert.Nil(t, roles)
}

// TestGetRolesByChannelID_ClusterChannelID evaluates correctness of GetRoleByChannelID function against
// cluster channel ids. Essentially, the test evaluates that RolesByChannel
// operates on top of channelRoleMap, and correctly identifies and strips of the cluster channel ids.
func TestGetRolesByChannelID_ClusterChannelID(t *testing.T) {
	// creates a cluster channel id
	conClusterChannel := ChannelConsensusCluster("some-consensus-cluster-id")

	// the roles list should contain collection
	roles, ok := RolesByChannel(conClusterChannel)
	assert.True(t, ok)
	assert.Len(t, roles, 1)
	assert.Contains(t, roles, flow.RoleCollection)
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

// TestIsClusterChannelID verifies the correctness of IsClusterChannel method
// against cluster and non-cluster channel ids.
func TestIsClusterChannelID(t *testing.T) {
	// creates a consensus cluster channel and verifies it
	conClusterChannel := ChannelConsensusCluster("some-consensus-cluster-id")
	clusterChannelID, ok := IsClusterChannel(conClusterChannel)
	require.True(t, ok)
	require.Equal(t, clusterChannelID, consensusClusterPrefix)

	// creates a sync cluster channel and verifies it
	syncClusterID := ChannelSyncCluster("some-sync-cluster-id")
	clusterChannelID, ok = IsClusterChannel(syncClusterID)
	require.True(t, ok)
	require.Equal(t, clusterChannelID, syncClusterPrefix)

	// non-cluster channel should not be verified
	clusterChannelID, ok = IsClusterChannel("non-cluster-channel-id")
	require.False(t, ok)
	require.Empty(t, clusterChannelID)
}

// TestUniqueChannelIDsByRole_Uniqueness verifies that non-cluster channel IDs returned by
// UniqueChannelIDsByRole are unique based on their ids.
func TestUniqueChannelIDsByRole_Uniqueness(t *testing.T) {
	for _, role := range flow.Roles() {
		channels := UniqueChannelIDsByRole(role)
		visited := make(map[flow.Identifier]struct{})
		for _, channel := range channels {
			if _, ok := IsClusterChannel(channel); ok {
				continue
			}

			// non-cluster channels should be unique in their id
			id := channelRoleMap[channel].ID()
			_, ok := visited[id]
			require.False(t, ok)

			visited[id] = struct{}{}
		}
	}
}
