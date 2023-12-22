package run

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenerateRootQC(t *testing.T) {
	participantData := createSignerData(t, 3)

	block := unittest.GenesisFixture()

	votes, err := GenerateRootBlockVotes(block, participantData)
	require.NoError(t, err)

	_, invalid, err := GenerateRootQC(block, votes, participantData, participantData.Identities())
	require.NoError(t, err)
	require.Len(t, invalid, 0) // no invalid votes
}

func TestGenerateRootQCWithSomeInvalidVotes(t *testing.T) {
	participantData := createSignerData(t, 10)

	block := unittest.GenesisFixture()

	votes, err := GenerateRootBlockVotes(block, participantData)
	require.NoError(t, err)

	// make 2 votes invalid
	votes[0].SigData = unittest.SignatureFixture()   // make invalid signature
	votes[1].SignerID = unittest.IdentifierFixture() // make invalid signer

	_, invalid, err := GenerateRootQC(block, votes, participantData, participantData.Identities())
	require.NoError(t, err)
	require.Len(t, invalid, 2) // 2 invalid votes
}

func createSignerData(t *testing.T, n int) *ParticipantData {
	identities := unittest.IdentityListFixture(n).Sort(flow.Canonical)

	networkingKeys := unittest.NetworkingKeys(n)
	stakingKeys := unittest.StakingKeys(n)

	seed := make([]byte, crypto.KeyGenSeedMinLen)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	randomBSKs, randomBPKs, groupKey, err := crypto.BLSThresholdKeyGen(n,
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
			identity.Weight,
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
