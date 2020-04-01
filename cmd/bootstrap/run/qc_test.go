package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/dkg"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/dkg/wrapper"
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
		identity := signer.Identity
		block.Identities = append(block.Identities, identity)
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
	identities := unittest.IdentityListFixture(n)

	stakingKeys := make([]crypto.PrivateKey, 0, len(identities))
	for _, identity := range identities {
		stakingKey := helper.MakeBLSKey(t)
		identity.StakingPubKey = stakingKey.PublicKey()
		stakingKeys = append(stakingKeys, stakingKey)
	}

	randomBKeys, groupKey, _ := unittest.RunDKG(t, n)

	pubData := dkg.PublicData{
		GroupPubKey:     groupKey,
		IDToParticipant: make(map[flow.Identifier]*dkg.Participant),
	}
	for i, identity := range identities {
		participant := dkg.Participant{
			Index:          uint(i),
			PublicKeyShare: randomBKeys[i].PublicKey(),
		}
		pubData.IDToParticipant[identity.NodeID] = &participant
	}

	signerData := SignerData{
		DKGState: wrapper.NewState(&pubData),
		Signers:  make([]Signer, n),
	}

	for i, identity := range identities {
		signerData.Signers[i].Identity = identity
		signerData.Signers[i].RandomBeaconPrivKey = randomBKeys[i]
		signerData.Signers[i].StakingPrivKey = stakingKeys[i]
	}

	return signerData
}
