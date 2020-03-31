package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/test"
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
		id := signer.Identity
		block.Identities = append(block.Identities, &id)
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

	stakingKeys, err := test.AddStakingPrivateKeys(ids)
	require.NoError(t, err)

	randomBKeys, dkgState, err := test.AddRandomBeaconPrivateKeys(t, ids)
	require.NoError(t, err)

	signerData := SignerData{
		DKGState: dkgState,
		Signers:  make([]Signer, n),
	}

	for i, id := range ids {
		signerData.Signers[i].Identity = *id
		signerData.Signers[i].RandomBeaconPrivKey = randomBKeys[i]
		signerData.Signers[i].StakingPrivKey = stakingKeys[i]
	}

	return signerData
}
