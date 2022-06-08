package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestGetRolesByChannel_NonClusterChannel evaluates correctness of RolesByChannel function against
// inclusion and exclusion of roles. Essentially, the test evaluates that RolesByChannel
// operates on top of channelRoleMap.
func TestGetRolesByChannel_NonClusterChannel(t *testing.T) {
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

// TestGetRolesByChannel_ClusterChannel evaluates correctness of RolesByChannel function against
// cluster channels. Essentially, the test evaluates that RolesByChannel
// operates on top of channelRoleMap, and correctly identifies and strips of the cluster channel.
func TestGetRolesByChannel_ClusterChannel(t *testing.T) {
	// creates a cluster channel.
	conClusterChannel := ChannelConsensusCluster("some-consensus-cluster-id")

	// the roles list should contain collection
	roles, ok := RolesByChannel(conClusterChannel)
	assert.True(t, ok)
	assert.Len(t, roles, 1)
	assert.Contains(t, roles, flow.RoleCollection)
}

// TestGetChannelByRole evaluates retrieving channels associated with a role from the
// channelRoleMap using ChannelsByRole. Essentially it evaluates that ChannelsByRole
// operates on top of channelRoleMap.
func TestGetChannelByRole(t *testing.T) {
	// asserts topics by the role for verification node
	// it should have the topics of
	// - PushBlocks
	// - PushReceipts
	// - PushApprovals
	// - ProvideApprovalsByChunk
	// - ProvideChunks
	// - TestNetwork
	// - TestMetric
	// the roles list should contain collection and consensus roles
	topics := ChannelsByRole(flow.RoleVerification)
	assert.Len(t, topics, 8)
	assert.Contains(t, topics, PushBlocks)
	assert.Contains(t, topics, PushReceipts)
	assert.Contains(t, topics, PushApprovals)
	assert.Contains(t, topics, ProvideApprovalsByChunk)
	assert.Contains(t, topics, RequestChunks)
	assert.Contains(t, topics, TestMetrics)
	assert.Contains(t, topics, TestNetwork)
	assert.Contains(t, topics, SyncCommittee)
}

// TestIsClusterChannel verifies the correctness of ClusterChannel method
// against cluster and non-cluster channel.
func TestIsClusterChannel(t *testing.T) {
	// creates a consensus cluster channel and verifies it
	conClusterChannel := ChannelConsensusCluster("some-consensus-cluster-id")
	ok := IsClusterChannel(conClusterChannel)
	require.True(t, ok)

	// creates a sync cluster channel and verifies it
	syncClusterChannel := ChannelSyncCluster("some-sync-cluster-id")
	ok = IsClusterChannel(syncClusterChannel)
	require.True(t, ok)

	// non-cluster channel should not be verified
	ok = IsClusterChannel("non-cluster-channel-id")
	require.False(t, ok)
}

// TestUniqueChannels_Uniqueness verifies that non-cluster channels returned by
// UniqueChannels are unique based on their set of involved roles.
// We use the identifier of RoleList to determine their uniqueness.
func TestUniqueChannels_Uniqueness(t *testing.T) {
	for _, role := range flow.Roles() {
		uniques := UniqueChannels(ChannelsByRole(role))

		visited := make(map[flow.Identifier]struct{})
		for _, channel := range uniques {

			if IsClusterChannel(channel) {
				continue //only considering non-cluster channel in this test case
			}

			// non-cluster channels should be unique based on their RoleList identifier.
			id := channelRoleMap[channel].ID()
			_, duplicate := visited[id]
			require.False(t, duplicate)

			visited[id] = struct{}{}
		}
	}
}

// TestUniqueChannels_ClusterChannels verifies that if cluster channels have the RoleList the same as
// single non-cluster channel, then all cluster channels as well as the one non-cluster channel are returned
// by the UniqueChannels. In other words, neither cluster channels nor non-cluster ones are de-duplicated in the
// favor of each other.
// We use the identifier of RoleList to determine their uniqueness.
func TestUniqueChannels_ClusterChannels(t *testing.T) {
	channels := ChannelsByRole(flow.RoleCollection)
	consensusCluster := ChannelConsensusCluster(flow.Emulator)
	syncCluster := ChannelSyncCluster(flow.Emulator)
	channels = append(channels, consensusCluster, syncCluster)
	uniques := UniqueChannels(channels)
	// collection role has two cluster and one non-cluster channels all with the same RoleList.
	// Hence all of them should be returned as unique channels.
	require.Contains(t, uniques, syncCluster)      // cluster channel
	require.Contains(t, uniques, consensusCluster) // cluster channel
	require.Contains(t, uniques, PushTransactions) // non-cluster channel
}
