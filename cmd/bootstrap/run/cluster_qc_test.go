package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenerateClusterRootQC(t *testing.T) {
	participants := createClusterParticipants(t, 3)

	block := unittest.BlockFixture()

	block.Payload.Seals = nil
	block.Payload.Guarantees = nil
	block.Header.ParentID = flow.ZeroID
	block.Header.View = 3
	block.Header.Height = 0
	block.Header.PayloadHash = block.Payload.Hash()

	clusterBlock := cluster.Block{
		Header: &flow.Header{
			ParentID: flow.ZeroID,
			View:     42,
		},
	}
	payload := cluster.EmptyPayload(flow.ZeroID)
	clusterBlock.SetPayload(payload)

	_, err := GenerateClusterRootQC(participants, model.ToIdentityList(participants), &clusterBlock)
	require.NoError(t, err)
}

func createClusterParticipants(t *testing.T, n int) []model.NodeInfo {
	ids := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleCollection))

	networkKeys := unittest.NetworkingKeys(n)
	stakingKeys := unittest.StakingKeys(n)

	participants := make([]model.NodeInfo, n)
	for i, id := range ids {
		participants[i] = model.NewPrivateNodeInfo(
			id.NodeID,
			id.Role,
			id.Address,
			id.Weight,
			networkKeys[i],
			stakingKeys[i],
		)
	}

	return participants
}
