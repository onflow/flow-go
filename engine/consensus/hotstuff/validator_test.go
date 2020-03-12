package hotstuff_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	hs "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"

	mockProtocol "github.com/dapperlabs/flow-go/protocol/mocks"
)

type FakeBuilder struct {
}

// the fake builder takes
func (b *FakeBuilder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	var payloadHash flow.Identifier
	rand.Read(payloadHash[:])

	// construct default block on top of the provided parent
	header := &flow.Header{
		Timestamp:   time.Now().UTC(),
		PayloadHash: payloadHash,
	}

	// apply the custom fields setter of the consensus algorithm
	setter(header)
	return header, nil
}

func TestValidateVote(t *testing.T) {
	// Happy Path
	t.Run("A valid vote should be valid", testVoteOK)
	// Unhappy Path
	t.Run("A vote with invalid view should be rejected", testVoteInvalidView)
	t.Run("A vote with invalid block ID should be rejected", testVoteInvalidBlock)
	t.Run("A vote from unstaked node should be rejected", testVoteUnstakedNode)
	t.Run("A vote with invalid staking sig should be rejected", testVoteInvalidStaking)
	// t.Run("A vote with invalid random beacon sig should be rejected", testVoteInvalidRandomB)
}

// func TestValidateQC(t *testing.T) {
// 	// Happy Path
// 	t.Run("A valid QC should be valid", testQCOK)
// 	// Unhappy Path
// 	t.Run("A QC with invalid blockID should be rejected", testQCInvalidBlock)
// 	t.Run("A QC with invalid view should be rejected", testQCInvalidView)
// 	t.Run("A QC from unstaked nodes should be rejected", testQCHasUnstakedSigner)
// 	t.Run("A QC from duplicated nodes should be rejected", testQCHasDuplicatedSigner)
// 	t.Run("A QC with insufficient stakes should be rejected", testQCHasInsufficentStake)
// 	t.Run("A QC with invalid staking sig should be rejected", testQCHasInvalidStakingSig)
// 	t.Run("A QC with invalid random beacon sig should be rejected", testQCHasInvalidRandomBSig)
// }
//
// func TestValidateProposal(t *testing.T) {
// 	// Happy Path
// 	t.Run("A valid proposal should be accepted", testProposalOK)
// 	// Unhappy Path
// 	t.Run("A proposal with invalid view should be rejected", testProposalInvalidView)
// 	t.Run("A proposal with invalid block ID should be rejected", testProposalInvalidBlock)
// 	t.Run("A proposal from unstaked node should be rejected", testProposalUnstakedNode)
// 	t.Run("A proposal with invalid staking sig should be rejected", testProposalInvalidStaking)
// 	t.Run("A proposal with invalid random beacon sig should be rejected", testProposalInvalidRandomB)
// 	t.Run("A proposal from the wrong leader should be rejected", testProposalWrongLeader)
// 	t.Run("A proposal with a QC pointing to a non-existing block, but equal to finalized view should be rejected", testProposalWrongParentEqual)
// 	t.Run("A proposal with a QC pointing to a non-existing block, but above finalized view should be rejected", testProposalWrongParentAbove)
// 	t.Run("A proposal with a QC pointing to a non-existing block, but below finalized view should be unverifiable", testProposalWrongParentBelow)
// 	t.Run("A proposal with a invalid QC should be rejected", testProposalInvalidQC)
// }

func testVoteOK(t *testing.T) {
	ps, ids := newProtocolState(t, 3)
	stakingKeys, err := addStakingPrivateKeys(ids)
	randomBKeys, dkgPubData, err := addRandomBeaconPrivateKeys(t, ids)
	signer, err := newRandomBeaconSigProvider(ps, dkgPubData, encoding.ConsensusVoteTag, ids[0], stakingKeys[0], randomBKeys[0])
	id := ids[0]
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		t.Fatal(err)
	}
	f := &mocks.ForksReader{}
	v := hotstuff.NewValidator(vs, f, signer)

	block := makeBlock(3)

	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	signerID, err := v.ValidateVote(vote, block)
	require.NoError(t, err)
	assert.Equal(t, signerID, ids[0])
}

func testVoteInvalidView(t *testing.T) {
	ps, ids := newProtocolState(t, 3)
	stakingKeys, err := addStakingPrivateKeys(ids)
	randomBKeys, dkgPubData, err := addRandomBeaconPrivateKeys(t, ids)
	signer, err := newRandomBeaconSigProvider(ps, dkgPubData, encoding.ConsensusVoteTag, ids[0], stakingKeys[0], randomBKeys[0])
	id := ids[0]
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		t.Fatal(err)
	}
	f := &mocks.ForksReader{}
	v := hotstuff.NewValidator(vs, f, signer)

	block := makeBlock(3)

	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// signature is valid, but View is invalid
	vote.View = 4

	_, err = v.ValidateVote(vote, block)
	assert.Error(t, err)
}

func testVoteInvalidBlock(t *testing.T) {
	ps, ids := newProtocolState(t, 3)
	stakingKeys, err := addStakingPrivateKeys(ids)
	randomBKeys, dkgPubData, err := addRandomBeaconPrivateKeys(t, ids)
	signer, err := newRandomBeaconSigProvider(ps, dkgPubData, encoding.ConsensusVoteTag, ids[0], stakingKeys[0], randomBKeys[0])
	id := ids[0]
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		t.Fatal(err)
	}
	f := &mocks.ForksReader{}
	v := hotstuff.NewValidator(vs, f, signer)

	block := makeBlock(3)

	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// signature is valid, but BlockID is invalid
	vote.BlockID = flow.HashToID([]byte{1, 2, 3})

	_, err = v.ValidateVote(vote, block)
	assert.Error(t, err)
}

func testVoteUnstakedNode(t *testing.T) {
	ps, ids := newProtocolState(t, 3)
	stakingKeys, err := addStakingPrivateKeys(ids)
	randomBKeys, dkgPubData, err := addRandomBeaconPrivateKeys(t, ids)
	signer, err := newRandomBeaconSigProvider(ps, dkgPubData, encoding.ConsensusVoteTag, ids[0], stakingKeys[0], randomBKeys[0])
	id := ids[0]
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		t.Fatal(err)
	}
	f := &mocks.ForksReader{}
	v := hotstuff.NewValidator(vs, f, signer)

	block := makeBlock(3)

	// signer is now unstaked
	ids[0].Stake = 0

	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	_, err = v.ValidateVote(vote, block)
	assert.Error(t, err)
}

func testVoteInvalidStaking(t *testing.T) {
	ps, ids := newProtocolState(t, 3)
	stakingKeys, err := addStakingPrivateKeys(ids)
	randomBKeys, dkgPubData, err := addRandomBeaconPrivateKeys(t, ids)
	signer, err := newRandomBeaconSigProvider(ps, dkgPubData, encoding.ConsensusVoteTag, ids[0], stakingKeys[0], randomBKeys[0])
	id := ids[0]
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		t.Fatal(err)
	}
	f := &mocks.ForksReader{}
	v := hotstuff.NewValidator(vs, f, signer)

	block := makeBlock(3)

	// signer is now unstaked
	ids[0].Stake = 0

	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	_, err = v.ValidateVote(vote, block)
	assert.Error(t, err)
}

// make a random block seeded by input
func makeBlock(seed int) *hs.Block {
	id := flow.MakeID(struct {
		BlockID int
	}{
		BlockID: seed,
	})
	return &hs.Block{
		BlockID: id,
		View:    uint64(seed),
	}
}

// create a protocol state with N identities
func newProtocolState(t *testing.T, n int) (protocol.State, flow.IdentityList) {
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

// create a new RandomBeaconAwareSigProvider
func newRandomBeaconSigProvider(ps protocol.State, dkgPubData *hotstuff.DKGPublicData, tag string, id *flow.Identity, stakingKey crypto.PrivateKey, randomBeaconKey crypto.PrivateKey) (*signature.RandomBeaconAwareSigProvider, error) {
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

// create N private keys and assign them to identities' RandomBeaconPubKey
func addRandomBeaconPrivateKeys(t *testing.T, ids flow.IdentityList) ([]crypto.PrivateKey, *hotstuff.DKGPublicData, error) {
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

// generete a random BLS private key
func nextBLSKey() (crypto.PrivateKey, error) {
	seed := make([]byte, 48)
	_, err := rand.Read(seed)
	if err != nil {
		return nil, err
	}
	sk, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	return sk, err
}

// create N private keys and assign them to identities' StakingPubKey
func addStakingPrivateKeys(ids flow.IdentityList) ([]crypto.PrivateKey, error) {
	sks := []crypto.PrivateKey{}
	for i := 0; i < len(ids); i++ {
		sk, err := nextBLSKey()
		if err != nil {
			return nil, fmt.Errorf("cannot create mock private key: %w", err)
		}
		ids[i].StakingPubKey = sk.PublicKey()
		sks = append(sks, sk)
	}
	return sks, nil
}
