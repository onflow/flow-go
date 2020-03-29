package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/test"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisQC(t *testing.T) {
	signerData := createSignerData(t, 3)

	block := unittest.BlockFixture()
	block.Identities = flow.IdentityList{
		unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
	}
	for _, signer := range signerData.Signers {
		block.Identities = append(block.Identities, signer.Identity())
	}
	block.ParentID = flow.ZeroID
	block.View = 3
	block.Seals = []*flow.Seal{&flow.Seal{}}
	block.Guarantees = nil
	block.PayloadHash = block.Payload.Hash()

	_, err := GenerateGenesisQC(signerData, &block)
	require.NoError(t, err)
}

func createSignerData(t *testing.T, n int) SignerData {
	_, ids := test.NewProtocolState(t, n)

	networkingKeys, err := unittest.NetworkingKeys(n)
	require.NoError(t, err)

	stakingKeys, err := unittest.StakingKeys(n)
	require.NoError(t, err)

	randomBKeys, dkgState, err := test.AddRandomBeaconPrivateKeys(t, ids)
	require.NoError(t, err)

	signerData := SignerData{
		DKGState: dkgState,
		Signers:  make([]Signer, n),
	}

	for i, id := range ids {
		signerData.Signers[i].NodeInfo = bootstrap.NewPrivateNodeInfo(
			id.NodeID,
			id.Role,
			id.Address,
			id.Stake,
			networkingKeys[i],
			stakingKeys[i],
		)

		// add random beacon private key
		signerData.Signers[i].RandomBeaconPrivKey = randomBKeys[i]
	}

	return signerData
}
