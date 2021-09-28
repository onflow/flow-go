package run

import (
	"crypto/rand"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenerateRootQC(t *testing.T) {
	participantData := createSignerData(t, 3)

	block := unittest.BlockFixture()
	block.Payload.Guarantees = nil
	block.Payload.Seals = nil
	block.Header.Height = 0
	block.Header.ParentID = flow.ZeroID
	block.Header.View = 3
	block.Header.PayloadHash = block.Payload.Hash()

	_, err := GenerateRootQC(&block, votes, participantData)
	require.NoError(t, err)
}

func createSignerData(t *testing.T, n int) *ParticipantData {
	identities := unittest.IdentityListFixture(n)

	networkingKeys, err := unittest.NetworkingKeys(n)
	require.NoError(t, err)

	stakingKeys, err := unittest.StakingKeys(n)
	require.NoError(t, err)

	seed := make([]byte, crypto.SeedMinLenDKG)
	_, err = rand.Read(seed)
	require.NoError(t, err)
	randomBSKs, randomBPKs, groupKey, err := crypto.ThresholdSignKeyGen(n,
		signature.RandomBeaconThreshold(n), seed)
	require.NoError(t, err)

	participantLookup := make(map[flow.Identifier]flow.DKGParticipant)
	participants := make([]Participant, n)

	for i, identity := range identities {

		// add to lookup
		lookupParticipant := flow.DKGParticipant{
			Index:    uint(i),
			KeyShare: randomBPKs[i],
		}
		participantLookup[identity.NodeID] = lookupParticipant

		// add to participant list
		nodeInfo := bootstrap.NewPrivateNodeInfo(
			identity.NodeID,
			identity.Role,
			identity.Address,
			identity.Stake,
			networkingKeys[i],
			stakingKeys[i],
		)
		participants[i] = Participant{
			NodeInfo:            nodeInfo,
			RandomBeaconPrivKey: randomBSKs[i],
		}
	}

	participantData := &ParticipantData{
		Participants: participants,
		Lookup:       participantLookup,
		GroupKey:     groupKey,
	}

	return participantData
}
