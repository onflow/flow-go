package topology

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	mock2 "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// createMockStateForCollectionNodes is a test helper function that generate a mock state
// clustering collection nodes into `clusterNum` clusters.
func CreateMockStateForCollectionNodes(t *testing.T, collectorIds flow.IdentityList, clusterNum uint) protocol.State {
	state := new(mock2.State)
	snapshot := new(mock2.Snapshot)
	epochQuery := new(mock2.EpochQuery)
	epoch := new(mock2.Epoch)
	assignments := unittest.ClusterAssignment(clusterNum, collectorIds)
	clusters, err := flow.NewClusterList(assignments, collectorIds)
	require.NoError(t, err)

	epoch.On("Clustering").Return(clusters, nil)
	epochQuery.On("Current").Return(epoch)
	snapshot.On("Epochs").Return(epochQuery)
	state.On("Final").Return(snapshot, nil)

	return state
}
