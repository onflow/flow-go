package topology

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// CreateMockStateForCollectionNodes is a test helper function that generate a mock state
// clustering collection nodes into `clusterNum` clusters.
func CreateMockStateForCollectionNodes(t *testing.T, collectorIds flow.IdentityList,
	clusterNum uint) (protocol.State, flow.ClusterList) {
	state := new(mockprotocol.State)
	snapshot := new(mockprotocol.Snapshot)
	epochQuery := new(mockprotocol.EpochQuery)
	epoch := new(mockprotocol.Epoch)
	assignments := unittest.ClusterAssignment(clusterNum, collectorIds)
	clusters, err := flow.NewClusterList(assignments, collectorIds)
	require.NoError(t, err)

	epoch.On("Clustering").Return(clusters, nil)
	epochQuery.On("Current").Return(epoch)
	snapshot.On("Epochs").Return(epochQuery)
	state.On("Final").Return(snapshot, nil)

	return state, clusters
}

// CheckConnectedness verifies graph as a whole is connected.
func CheckConnectedness(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList) {
	CheckGraphConnected(t, adjMap, ids, filter.Any)
}

// CheckConnectednessByChannelID verifies that the subgraph of nodes subscribed to a channelID is connected.
func CheckConnectednessByChannelID(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList,
	channelID string) {
	roles, ok := engine.RolesByChannelID(channelID)
	require.True(t, ok)
	CheckGraphConnected(t, adjMap, ids, filter.HasRole(roles...))
}

// CheckGraphConnected checks if the graph represented by the adjacency matrix is connected.
// It traverses the adjacency map starting from an arbitrary node and checks if all nodes that satisfy the filter
// were visited.
func CheckGraphConnected(t *testing.T, adjMap map[flow.Identifier]flow.IdentityList, ids flow.IdentityList, f flow.IdentityFilter) {

	// filter the ids and find the expected node count
	expectedIDs := ids.Filter(f)
	expectedCount := len(expectedIDs)

	// start with an arbitrary node which satisfies the filter
	startID := expectedIDs.Sample(1)[0].NodeID

	visited := make(map[flow.Identifier]bool)
	dfs(startID, adjMap, visited, f)

	// assert that expected number of nodes were visited by DFS
	require.Equal(t, expectedCount, len(visited))
}

// MockSubscriptionManager returns a list of mocked subscription manages for the input
// identities. It only mocks the GetChannelIDs method of the subscription manager. Other methods
// return an error, as they are not supposed to be invoked.
func MockSubscriptionManager(t *testing.T, ids flow.IdentityList) []network.SubscriptionManager {
	require.NotEmpty(t, ids)

	sms := make([]network.SubscriptionManager, len(ids))
	for i, id := range ids {
		sm := &mocknetwork.SubscriptionManager{}
		err := fmt.Errorf("this method should not be called on mock subscription manager")
		sm.On("Register", mock.Anything, mock.Anything).Return(err)
		sm.On("Unregister", mock.Anything).Return(err)
		sm.On("GetEngine", mock.Anything).Return(err)
		sm.On("GetChannelIDs").Return(engine.ChannelIDsByRole(id.Role))
		sms[i] = sm
	}

	return sms
}

// CheckMembership checks each identity in a top list belongs to all identity list.
func CheckMembership(t *testing.T, top flow.IdentityList, all flow.IdentityList) {
	for _, id := range top {
		require.Contains(t, all, id)
	}
}

// TODO: fix this test after we have fanout optimized.
// CheckTopologySize evaluates that overall topology size of a node is bound by the fanout of system.
func CheckTopologySize(t *testing.T, total int, top flow.IdentityList) {
	t.Skip("this test requires optimizing the fanout per topic")
	fanout := (total + 1) / 2
	require.True(t, len(top) <= fanout)
}

// ClusterNum is a test helper determines the number of clusters of specific `size`.
func ClusterNum(t *testing.T, ids flow.IdentityList, size int) int {
	collectors := ids.Filter(filter.HasRole(flow.RoleCollection))

	// we need at least two collector nodes to generate a cluster
	// and check the connectedness
	require.True(t, len(collectors) >= 2)
	require.True(t, size > 0)

	clusterNum := len(collectors) / size
	return int(math.Max(float64(clusterNum), 1))
}

// DFS is a test helper function checking graph connectedness. It fails if
// graph represented by `adjMap` is not connected, i.e., there is more than a single
// connected component.
func dfs(currentID flow.Identifier,
	adjMap map[flow.Identifier]flow.IdentityList,
	visited map[flow.Identifier]bool,
	filter flow.IdentityFilter) {

	if visited[currentID] {
		return
	}

	visited[currentID] = true

	for _, id := range adjMap[currentID].Filter(filter) {
		dfs(id.NodeID, adjMap, visited, filter)
	}
}

// uniquenessCheck is a test helper method that fails the test if all include any duplicate identity.
func uniquenessCheck(t *testing.T, ids flow.IdentityList) {
	seen := make(map[flow.Identity]struct{})
	for _, id := range ids {
		// checks if id is duplicate in ids list
		_, ok := seen[*id]
		require.False(t, ok)

		// marks id as seen
		seen[*id] = struct{}{}
	}
}

// clusterChannelIDs is a test helper method that returns all cluster-based channel ids.
func clusterChannelIDs(t *testing.T) []string {
	ccids := make([]string, 0)
	for _, channelID := range engine.ChannelIDs() {
		if _, ok := engine.IsClusterChannelID(channelID); !ok {
			continue
		}
		ccids = append(ccids, channelID)
	}

	require.NotEmpty(t, ccids)
	return ccids
}

// checkConnectednessByCluster is a test helper that checks all nodes belong to a cluster are connected.
func checkConnectednessByCluster(t *testing.T,
	adjMap map[flow.Identifier]flow.IdentityList,
	all flow.IdentityList,
	cluster flow.IdentityList) {
	CheckGraphConnected(t,
		adjMap,
		all,
		filter.In(cluster))
}
