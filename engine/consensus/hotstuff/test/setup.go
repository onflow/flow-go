package test

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/protocol"
	mockProtocol "github.com/dapperlabs/flow-go/protocol/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// generete a random BLS private key
func NextBLSKey() (crypto.PrivateKey, error) {
	seed := make([]byte, 48)
	_, err := rand.Read(seed)
	if err != nil {
		return nil, err
	}
	sk, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	return sk, err
}

// make a random block seeded by input
func MakeBlock(seed int) *model.Block {
	id := flow.MakeID(struct {
		BlockID int
	}{
		BlockID: seed,
	})
	return &model.Block{
		BlockID: id,
		View:    uint64(seed),
	}
}

// create a protocol state with N identities
func NewProtocolState(t *testing.T, n int) (protocol.State, flow.IdentityList) {
	ctrl := gomock.NewController(t)
	// mock identity list
	ids := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleConsensus))

	// mock protocol state
	mockProtocolState := mockProtocol.NewMockState(ctrl)
	mockSnapshot := mockProtocol.NewMockSnapshot(ctrl)
	mockProtocolState.EXPECT().AtBlockID(gomock.Any()).Return(mockSnapshot).AnyTimes()
	mockProtocolState.EXPECT().Final().Return(mockSnapshot).AnyTimes()
	for _, id := range ids {
		mockSnapshot.EXPECT().Identity(id.NodeID).Return(id, nil).AnyTimes()
	}
	mockSnapshot.EXPECT().Identities(gomock.Any()).DoAndReturn(func(f ...flow.IdentityFilter) (flow.IdentityList, error) {
		return ids.Filter(f...), nil
	}).AnyTimes()
	return mockProtocolState, ids
}

// create N private keys and assign them to identities' StakingPubKey
func AddStakingPrivateKeys(ids flow.IdentityList) ([]crypto.PrivateKey, error) {
	sks := []crypto.PrivateKey{}
	for i := 0; i < len(ids); i++ {
		sk, err := NextBLSKey()
		if err != nil {
			return nil, fmt.Errorf("cannot create mock private key: %w", err)
		}
		ids[i].StakingPubKey = sk.PublicKey()
		sks = append(sks, sk)
	}
	return sks, nil
}

// create N private keys and assign them to identities' RandomBeaconPubKey
func AddRandomBeaconPrivateKeys(t *testing.T, ids flow.IdentityList) ([]crypto.PrivateKey, *hotstuff.DKGPublicData, error) {
	sks, groupPubKey, keyShares := unittest.RunDKGKeys(t, len(ids))
	for i := 0; i < len(ids); i++ {
		sk := sks[i]
		ids[i].RandomBeaconPubKey = sk.PublicKey()
	}

	dkgMap := make(map[flow.Identifier]*hotstuff.DKGParticipant)
	for i, id := range ids {
		dkgMap[id.NodeID] = &hotstuff.DKGParticipant{
			Id:             id.NodeID,
			PublicKeyShare: keyShares[i],
			DKGIndex:       i,
		}
	}
	dkgPubData := hotstuff.DKGPublicData{
		GroupPubKey:           groupPubKey,
		IdToDKGParticipantMap: dkgMap,
	}
	return sks, &dkgPubData, nil
}

// fake N private keys and assign them to identities' RandomBeaconPubKey
// useful for meansure signing random beacon sigs without verifying the reconstructed sigs
func AddFakeRandomBeaconPrivateKeys(t *testing.T, ids flow.IdentityList) ([]crypto.PrivateKey, *hotstuff.DKGPublicData, error) {
	sks := []crypto.PrivateKey{}
	for i := 0; i < len(ids); i++ {
		sk, err := NextBLSKey()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create mock private key: %w", err)
		}
		ids[i].RandomBeaconPubKey = sk.PublicKey()
		sks = append(sks, sk)
	}

	dkgMap := make(map[flow.Identifier]*hotstuff.DKGParticipant)
	for i, id := range ids {
		dkgMap[id.NodeID] = &hotstuff.DKGParticipant{
			Id:             id.NodeID,
			PublicKeyShare: sks[i].PublicKey(),
			DKGIndex:       i,
		}
	}

	// fake the group pub key as the first pub key
	groupPubKey := sks[0].PublicKey()

	dkgPubData := hotstuff.DKGPublicData{
		GroupPubKey:           groupPubKey,
		IdToDKGParticipantMap: dkgMap,
	}
	return sks, &dkgPubData, nil
}

// create a new StakingSigProvider
func NewStakingProvider(ps protocol.State, tag string, id *flow.Identity, sk crypto.PrivateKey) (*signature.StakingSigProvider, error) {
	vs, err := hotstuff.NewViewState(ps, nil, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, sk)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := signature.NewStakingSigProvider(vs, tag, me)
	return &sigProvider, nil
}

// create a new RandomBeaconAwareSigProvider
func NewRandomBeaconSigProvider(ps protocol.State, dkgPubData *hotstuff.DKGPublicData, id *flow.Identity, stakingKey crypto.PrivateKey, randomBeaconKey crypto.PrivateKey) (*signature.RandomBeaconAwareSigProvider, error) {
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, stakingKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := signature.NewRandomBeaconAwareSigProvider(vs, me, randomBeaconKey)
	return &sigProvider, nil
}

// setup necessary signer, verifier, and aggregator for a cluster of N nodes
func MakeSignerAndVerifier(t *testing.T, n int, enableRandomBeacon bool) (
	flow.IdentityList,
	[]hotstuff.Signer,
	hotstuff.SigVerifier,
	hotstuff.SigAggregator,
	[]crypto.PrivateKey, // stakingKeys
	[]crypto.PrivateKey, // randomBKeys
	*hotstuff.DKGPublicData,
) {

	assert.NotEqual(t, n, 0)

	ps, ids := NewProtocolState(t, n)
	stakingKeys, err := AddStakingPrivateKeys(ids)
	require.NoError(t, err)

	signers := make([]hotstuff.Signer, n)
	var verifier hotstuff.SigVerifier
	var aggregator hotstuff.SigAggregator

	if !enableRandomBeacon {
		// if random beacon is disabled, create staking sig provider
		for i := 0; i < n; i++ {
			signer, err := NewStakingProvider(ps, encoding.ConsensusVoteTag, ids[i], stakingKeys[i])
			require.NoError(t, err)
			signers[i] = signer
			if i == 0 {
				verifier = signer
				aggregator = signer
			}
		}
		return ids, signers, verifier, aggregator, stakingKeys, nil, nil
	}

	randomBKeys, dkgPubData, err := AddRandomBeaconPrivateKeys(t, ids)
	for i := 0; i < n; i++ {
		signer, err := NewRandomBeaconSigProvider(ps, dkgPubData, ids[i], stakingKeys[i], randomBKeys[i])
		require.NoError(t, err)
		signers[i] = signer
		if i == 0 {
			verifier = signer
			aggregator = signer
		}
	}
	return ids, signers, verifier, aggregator, stakingKeys, randomBKeys, dkgPubData
}

// MakeSignerAndVerifierWithFakeDKG generates staking and random beacon keys for n parties.
// The random beacon key shares are FAKE: they are individually-generated BLS keys (not generated by a DGK).
// The reason we don't use real DKG-generated signatures is the runtime of the DKG, which would unnecessarily extend testing.
func MakeSignerAndVerifierWithFakeDKG(t *testing.T, n int) (
	flow.IdentityList,
	[]hotstuff.Signer,
	hotstuff.SigVerifier,
	hotstuff.SigAggregator,
	[]crypto.PrivateKey, // stakingKeys
	[]crypto.PrivateKey, // randomBKeys
	*hotstuff.DKGPublicData,
) {
	assert.NotEqual(t, n, 0)

	ps, ids := NewProtocolState(t, n)
	stakingKeys, err := AddStakingPrivateKeys(ids)
	require.NoError(t, err)

	signers := make([]hotstuff.Signer, n)
	var verifier hotstuff.SigVerifier
	var aggregator hotstuff.SigAggregator

	randomBKeys, dkgPubData, err := AddFakeRandomBeaconPrivateKeys(t, ids)
	for i := 0; i < n; i++ {
		signer, err := NewRandomBeaconSigProvider(ps, dkgPubData, ids[i], stakingKeys[i], randomBKeys[i])
		require.NoError(t, err)
		signers[i] = signer
		if i == 0 {
			verifier = signer
			aggregator = signer
		}
	}
	return ids, signers, verifier, aggregator, stakingKeys, randomBKeys, dkgPubData
}
