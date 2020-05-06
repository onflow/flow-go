package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateClusterGenesisQC(t *testing.T) {
	participants := createClusterParticipants(t, 3)

	block := unittest.BlockFixture()
	block.Payload.Identities = flow.IdentityList{
		unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus)),
	}
	for _, participant := range participants {
		block.Payload.Identities = append(block.Payload.Identities, participant.Identity())
	}

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

	_, err := GenerateClusterGenesisQC(participants, &block, &clusterBlock)
	require.NoError(t, err)
}

func createClusterParticipants(t *testing.T, n int) []model.NodeInfo {
	ids := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleCollection))

	networkKeys, err := unittest.NetworkingKeys(n)
	require.NoError(t, err)

	stakingKeys, err := unittest.StakingKeys(n)
	require.NoError(t, err)

	participants := make([]model.NodeInfo, n)
	for i, id := range ids {
		participants[i] = model.NewPrivateNodeInfo(
			id.NodeID,
			id.Role,
			id.Address,
			id.Stake,
			networkKeys[i],
			stakingKeys[i],
		)
	}

	return participants
}
