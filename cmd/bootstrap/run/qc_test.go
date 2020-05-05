package run

import (
	"testing"
	"crypto/rand"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/dkg"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/dkg/wrapper"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/dapperlabs/flow-go/crypto"
)

func TestGenerateGenesisQC(t *testing.T) {
	participantData := createSignerData(t, 3)

	block := unittest.BlockFixture()
	block.Payload.Identities = flow.IdentityList{
		unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)),
		unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
	}
	block.Payload.Guarantees = nil
	block.Payload.Seals = nil
	for _, participant := range participantData.Participants {
		block.Payload.Identities = append(block.Payload.Identities, participant.Identity())
	}
	block.Header.Height = 0
	block.Header.ParentID = flow.ZeroID
	block.Header.View = 3
	block.Header.PayloadHash = block.Payload.Hash()

	_, err := GenerateGenesisQC(participantData, &block)
	require.NoError(t, err)
}

func createSignerData(t *testing.T, n int) ParticipantData {
	identities := unittest.IdentityListFixture(n)

	networkingKeys, err := unittest.NetworkingKeys(n)
	require.NoError(t, err)

	stakingKeys, err := unittest.StakingKeys(n)
	require.NoError(t, err)

	seed := make([]byte, crypto.SeedMinLenDKG)
	_, err = rand.Read(seed)
	require.NoError(t, err)
	randomBSKs, randomBPKs, groupKey, err := crypto.ThresholdSignKeyGen(n, seed)

	pubData := dkg.PublicData{
		GroupPubKey:     groupKey,
		IDToParticipant: make(map[flow.Identifier]*dkg.Participant),
	}
	for i, identity := range identities {
		participant := dkg.Participant{
			Index:          uint(i),
			PublicKeyShare: randomBPKs[i],
		}
		pubData.IDToParticipant[identity.NodeID] = &participant
	}

	participantData := ParticipantData{
		DKGState:     wrapper.NewState(&pubData),
		Participants: make([]Participant, n),
	}

	for i, identity := range identities {
		participantData.Participants[i].NodeInfo = bootstrap.NewPrivateNodeInfo(
			identity.NodeID,
			identity.Role,
			identity.Address,
			identity.Stake,
			networkingKeys[i],
			stakingKeys[i],
		)

		// add random beacon private key
		participantData.Participants[i].RandomBeaconPrivKey = randomBSKs[i]
	}

	return participantData
}
