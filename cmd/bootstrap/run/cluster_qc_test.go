package run

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenerateClusterRootQC(t *testing.T) {
	participants := createClusterParticipants(t, 3)

	clusterBlock, err := cluster.NewRootBlock(
		cluster.UntrustedBlock{
			HeaderBody: flow.HeaderBody{
				ChainID:   flow.Emulator,
				ParentID:  flow.ZeroID,
				Timestamp: uint64(time.Now().UnixMilli()),
				View:      42,
			},
			Payload: *cluster.NewEmptyPayload(flow.ZeroID),
		},
	)
	require.NoError(t, err)

	orderedParticipants := model.ToIdentityList(participants).Sort(flow.Canonical[flow.Identity]).ToSkeleton()
	_, err = GenerateClusterRootQC(participants, orderedParticipants, clusterBlock)
	require.NoError(t, err)
}

func createClusterParticipants(t *testing.T, n int) []model.NodeInfo {
	ids := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleCollection))

	networkKeys := unittest.NetworkingKeys(n)
	stakingKeys := unittest.StakingKeys(n)

	participants := make([]model.NodeInfo, n)
	for i, id := range ids {
		var err error
		participants[i], err = model.NewPrivateNodeInfo(
			id.NodeID,
			id.Role,
			id.Address,
			id.InitialWeight,
			networkKeys[i],
			stakingKeys[i],
		)
		require.NoError(t, err)
	}

	return participants
}
