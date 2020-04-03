package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/test"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateClusterGenesisQC(t *testing.T) {
	signers := createClusterSigners(t, 3)

	block := unittest.BlockFixture()
	block.Identities = flow.IdentityList{
		unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
	}

	for _, signer := range signers {
		block.Identities = append(block.Identities, signer.Identity())
	}

	block.ParentID = flow.ZeroID
	block.View = 3
	block.Seals = []*flow.Seal{&flow.Seal{}}
	block.Guarantees = nil
	block.PayloadHash = block.Payload.Hash()

	clusterBlock := cluster.Block{
		Header: flow.Header{
			ParentID: flow.ZeroID,
			View:     42,
		},
		Payload: cluster.Payload{
			Collection: flow.LightCollection{Transactions: nil},
		},
	}
	clusterBlock.PayloadHash = clusterBlock.Payload.Hash()

	_, err := GenerateClusterGenesisQC(signers, &block, &clusterBlock)
	require.NoError(t, err)
}

func createClusterSigners(t *testing.T, n int) []model.NodeInfo {
	_, ids := test.NewProtocolState(t, n)

	stakingKeys, err := test.AddStakingPrivateKeys(ids)
	require.Nil(t, err)

	networkKeys, err := unittest.NetworkingKeys(len(ids))
	require.Nil(t, err)

	signers := make([]model.NodeInfo, n)

	for i, id := range ids {
		signers[i] = model.NewPrivateNodeInfo(
			id.NodeID,
			id.Role,
			id.Address,
			id.Stake,
			networkKeys[i],
			stakingKeys[i],
		)
	}

	return signers
}
