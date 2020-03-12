package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/protocol"
	mockProtocol "github.com/dapperlabs/flow-go/protocol/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomBeaconEnabledSigning(t *testing.T) {
	// Happy Paths
	t.Run("signature from a proposal should be accepted by random beacon enabled verifier", testOKSigningRBEnabled)
	t.Run("signature from a vote should be accepted by random beacon enabled verifier", testOKSigningVoteRBEnabled)
	t.Run("a proposal is also a vote", testProposalIsVote)
	// Unhappy Paths
	t.Run("signature for block with wrong BlockID should be rejected", testWrongBlockIDRBEnabled)
	t.Run("signature for block with wrong View should be rejected", testWrongViewRBEnabled)
	t.Run("signature for block signed with wrong KMac Tag should be rejected", testWrongKMacTagRBEnabled)
	t.Run("signature claimed from a wrong signer should be rejected", testWrongSignerRBEnabled)
}

func TestRandomBeaconDisabledSigning(t *testing.T) {
	// Happy Path
	t.Run("signature from a proposal should be accepted by random beacon disabled verifier", testOKSigningRBDisabled)
	t.Run("signature from a vote should be accepted by random beacon disabled verifier", testOKSigningVoteRBDisabled)
	// Unhappy Path
	t.Run("signature for block with wrong BlockID should be rejected", testWrongBlockIDRBDisabled)
	t.Run("signature for block with wrong View should be rejected", testWrongViewRBDisabled)
	t.Run("signature for block signed with wrong KMAC Tag should be rejected", testWrongKMacTagRBDisabled)
	t.Run("signature claimed from a wrong signer should be rejected", testWrongSignerRBDisabled)
}

func TestRandomBeaconEnabledAggregation(t *testing.T) {
	// Happy Path
	t.Run("signatures can be aggregated and verified", testAggregateRBEnabled)
	// Unhappy Path
	t.Run("aggregated signatures for different blocks should be rejected", testAggregateDifferentBlockRBEnabled)
	t.Run("aggregated signatures claimed from a wrong signer should be rejected", testAggregateWithWrongSignerRBEnabled)
	t.Run("aggregated with insufficent shares should be rejected", testAggregateWithInsufficientSharesRBEnabled)
}

func TestRandomBeaconDisabledAggregation(t *testing.T) {
	// Happy Path
	t.Run("signatures can be aggregated and verified", testAggregateRBDisabled)
	// Unhappy Path
	t.Run("aggregated signatures for different blocks should be rejected", testAggregateWithWrongBlockIDRBDisabled)
	t.Run("aggregated signatures claimed from a wrong signer should be rejected", testAggregateWithWrongSignerRBDisabled)
}

func testOKSigningRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBeaconKeys, _ := makeSignerAndVerifier(t, 4, true)

	// create proposal
	block := makeBlock(1)
	proposal, err := signers[0].Propose(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(proposal.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// verify random beacon sig by a sig provider
	valid, err = verifier.VerifyRandomBeaconSig(proposal.RandomBeaconSignature, block, randomBeaconKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// verify random beacon sig by a unstaked verifier
	valid, err = NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(proposal.RandomBeaconSignature, block, randomBeaconKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}

func testOKSigningVoteRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := makeSignerAndVerifier(t, 4, true)

	// create vote
	block := makeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// verify random beacon sig by a sig provider
	valid, err = verifier.VerifyRandomBeaconSig(vote.Signature.RandomBeaconSignature, block, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// verify random beacon sig by a unstaked verifier
	valid, err = NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, block, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}

func testProposalIsVote(t *testing.T) {
	_, signers, _, _, _, _, _ := makeSignerAndVerifier(t, 4, false)
	signer := signers[0]

	// make a vote from proposal
	block := makeBlock(1)
	proposal, err := signer.Propose(block)
	require.NoError(t, err)

	// make a vote
	proposerVote := proposal.ProposerVote()
	vote, err := signer.VoteFor(block)

	assert.Equal(t, proposerVote.Signature.StakingSignature, vote.Signature.StakingSignature)
	assert.Equal(t, proposerVote.Signature.RandomBeaconSignature, vote.Signature.RandomBeaconSignature)
}

func testWrongBlockIDRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := makeSignerAndVerifier(t, 4, true)

	// create vote
	block, withWrongBlockID := makeBlock(1), makeBlock(2)
	withWrongBlockID.View = block.View // withWrongBlockID has the different BlockID
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, withWrongBlockID, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)

	// verify random beacon sig by a sig provider
	valid, err = verifier.VerifyRandomBeaconSig(vote.Signature.RandomBeaconSignature, withWrongBlockID, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)

	// verify random beacon sig by a unstaked verifier
	valid, err = NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, withWrongBlockID, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongViewRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := makeSignerAndVerifier(t, 4, true)

	// create vote
	block, withWrongView := makeBlock(1), makeBlock(2)
	withWrongView.BlockID = block.BlockID // withWrongView has the different View
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, withWrongView, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)

	// verify random beacon sig by a sig provider
	valid, err = verifier.VerifyRandomBeaconSig(vote.Signature.RandomBeaconSignature, withWrongView, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)

	// verify random beacon sig by a unstaked verifier
	valid, err = NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, withWrongView, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongKMacTagRBEnabled(t *testing.T) {
	_, signers, _, _, _, randomBKeys, _ := makeSignerAndVerifier(t, 4, true)

	// create vote
	block := makeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify random beacon sig by a unstaked verifier
	verifier := RandomBeaconSigVerifier{
		randomBeaconHasher: crypto.NewBLS_KMAC("wrongtag"),
	}
	valid, err := verifier.VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, block, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongSignerRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := makeSignerAndVerifier(t, 4, true)

	// create vote
	block := makeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	wrongStakingPubKey, wrongRandBPubKey := stakingKeys[1].PublicKey(), randomBKeys[1].PublicKey()

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, wrongStakingPubKey)
	require.NoError(t, err)
	assert.Equal(t, false, valid)

	// verify random beacon sig by a unstaked verifier
	valid, err = verifier.VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, block, wrongRandBPubKey)
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testOKSigningRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create proposal
	block := makeBlock(1)
	proposal, err := signers[0].Propose(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(proposal.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}
func testOKSigningVoteRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create vote
	block := makeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}

func testWrongBlockIDRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create vote
	block, withWrongBlockID := makeBlock(1), makeBlock(2)
	withWrongBlockID.View = block.View // withWrongBlockID has the different BlockID
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, withWrongBlockID, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongViewRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create vote
	block, withWrongView := makeBlock(1), makeBlock(2)
	withWrongView.BlockID = block.BlockID // withWrongView has the different View
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, withWrongView, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongKMacTagRBDisabled(t *testing.T) {
	ps, ids := newProtocolState(t, 4)
	stakingKeys, err := addStakingPrivateKeys(ids)
	require.NoError(t, err)

	var signer hotstuff.Signer
	var verifier hotstuff.SigVerifier

	// creating signer
	signer, err = newStakingSigProvider(ps, "wrongtag", ids[0], stakingKeys[0]) // bad signer init with a wrong tag
	require.NoError(t, err)
	verifier, err = newStakingSigProvider(ps, encoding.CollectorVoteTag, ids[1], stakingKeys[1])
	require.NoError(t, err)

	// create vote
	block := makeBlock(1)
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongSignerRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create vote
	block := makeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	wrongStakingPubKey := stakingKeys[1].PublicKey()

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, wrongStakingPubKey)
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testAggregateRBEnabled(t *testing.T) {
	_, signers, verifier, aggregator, stakingKeys, _, dkg := makeSignerAndVerifier(t, 4, true)

	// create block
	block := makeBlock(1)

	// create votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)

	// aggregate
	aggsig, err := aggregator.Aggregate(block, []*model.SingleSignature{vote1.Signature, vote2.Signature})
	require.NoError(t, err)

	// verify aggregated staking sig
	valid, err := verifier.VerifyStakingAggregatedSig(aggsig.StakingSignatures, block, []crypto.PublicKey{
		stakingKeys[0].PublicKey(),
		stakingKeys[1].PublicKey(),
	})
	require.NoError(t, err)
	assert.True(t, valid)

	// verify aggregated random beacon sig
	valid, err = verifier.VerifyRandomBeaconThresholdSig(aggsig.RandomBeaconSignature, block, dkg.GroupPubKey)
	require.NoError(t, err)
	assert.True(t, valid)
}

func testAggregateDifferentBlockRBEnabled(t *testing.T) {
	_, signers, _, aggregator, _, _, _ := makeSignerAndVerifier(t, 4, true)

	// create block
	block1, block2 := makeBlock(1), makeBlock(2)

	// create votes
	vote1, err := signers[0].VoteFor(block1)
	require.NoError(t, err)

	vote2, err := signers[1].VoteFor(block2)
	require.NoError(t, err)

	// aggregate
	_, err = aggregator.Aggregate(block1, []*model.SingleSignature{vote1.Signature, vote2.Signature})
	assert.Error(t, err)
}

func testAggregateWithWrongSignerRBEnabled(t *testing.T) {
	_, signers, verifier, aggregator, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, true)

	// create block
	block := makeBlock(1)

	// create votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)

	// aggregate
	aggsig, err := aggregator.Aggregate(block, []*model.SingleSignature{vote1.Signature, vote2.Signature})
	require.NoError(t, err)

	// verify aggregated staking sig
	valid, err := verifier.VerifyStakingAggregatedSig(aggsig.StakingSignatures, block, []crypto.PublicKey{
		stakingKeys[0].PublicKey(),
		stakingKeys[3].PublicKey(), // this was not the signer
	})
	require.NoError(t, err)
	assert.Equal(t, false, valid)

	// create a different dkg group with 5 nodes
	_, _, _, _, _, _, dkg2 := makeSignerAndVerifier(t, 5, true)
	// verify aggregated random beacon sig
	valid, err = verifier.VerifyRandomBeaconThresholdSig(aggsig.RandomBeaconSignature, block, dkg2.GroupPubKey)
	// fail because the group public key was wrong
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testAggregateWithInsufficientSharesRBEnabled(t *testing.T) {
	_, signers, _, aggregator, _, _, _ := makeSignerAndVerifier(t, 4, true)

	// create block
	block := makeBlock(1)

	// create votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// aggregate
	_, err = aggregator.Aggregate(block, []*model.SingleSignature{vote1.Signature})
	// needs two shares, but got only one
	assert.Error(t, err)
}

func testAggregateRBDisabled(t *testing.T) {
	_, signers, verifier, aggregator, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create block
	block := makeBlock(1)

	// create votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)

	// aggregate
	aggsig, err := aggregator.Aggregate(block, []*model.SingleSignature{vote1.Signature, vote2.Signature})
	require.NoError(t, err)

	// verify aggregated staking sig
	valid, err := verifier.VerifyStakingAggregatedSig(aggsig.StakingSignatures, block, []crypto.PublicKey{
		stakingKeys[0].PublicKey(),
		stakingKeys[1].PublicKey(),
	})
	require.NoError(t, err)
	assert.True(t, valid)
}

func testAggregateWithWrongBlockIDRBDisabled(t *testing.T) {
	_, signers, _, aggregator, _, _, _ := makeSignerAndVerifier(t, 4, false)

	// create block
	block1, block2 := makeBlock(1), makeBlock(2)

	// create votes
	vote1, err := signers[0].VoteFor(block1)
	require.NoError(t, err)

	vote2, err := signers[1].VoteFor(block2)
	require.NoError(t, err)

	// aggregate
	_, err = aggregator.Aggregate(block1, []*model.SingleSignature{vote1.Signature, vote2.Signature})
	assert.Error(t, err)
}

func testAggregateWithWrongSignerRBDisabled(t *testing.T) {
	_, signers, verifier, aggregator, stakingKeys, _, _ := makeSignerAndVerifier(t, 4, false)

	// create block
	block := makeBlock(1)

	// create votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)

	// aggregate
	aggsig, err := aggregator.Aggregate(block, []*model.SingleSignature{vote1.Signature, vote2.Signature})
	require.NoError(t, err)

	// verify aggregated staking sig
	valid, err := verifier.VerifyStakingAggregatedSig(aggsig.StakingSignatures, block, []crypto.PublicKey{
		stakingKeys[0].PublicKey(),
		stakingKeys[3].PublicKey(), // this was not the signer
	})
	require.NoError(t, err)
	assert.Equal(t, false, valid)
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

// make a random block seeded by input
func makeBlock(seed int) *model.Block {
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

// create a new StakingSigProvider
func newStakingSigProvider(ps protocol.State, tag string, id *flow.Identity, sk crypto.PrivateKey) (*StakingSigProvider, error) {
	vs, err := hotstuff.NewViewState(ps, nil, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, sk)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := NewStakingSigProvider(vs, tag, me)
	return &sigProvider, nil
}

// create a new RandomBeaconAwareSigProvider
func newRandomBeaconSigProvider(ps protocol.State, dkgPubData *hotstuff.DKGPublicData, tag string, id *flow.Identity, stakingKey crypto.PrivateKey, randomBeaconKey crypto.PrivateKey) (*RandomBeaconAwareSigProvider, error) {
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, stakingKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := NewRandomBeaconAwareSigProvider(vs, me, randomBeaconKey)
	return &sigProvider, nil
}

// setup necessary signer, verifier, and aggregator for a cluster of N nodes
func makeSignerAndVerifier(t *testing.T, n int, enableRandomBeacon bool) (
	flow.IdentityList,
	[]hotstuff.Signer,
	hotstuff.SigVerifier,
	hotstuff.SigAggregator,
	[]crypto.PrivateKey, // stakingKeys
	[]crypto.PrivateKey, // randomBKeys
	*hotstuff.DKGPublicData,
) {

	assert.NotEqual(t, n, 0)

	ps, ids := newProtocolState(t, n)
	stakingKeys, err := addStakingPrivateKeys(ids)
	require.NoError(t, err)

	signers := make([]hotstuff.Signer, n)
	var verifier hotstuff.SigVerifier
	var aggregator hotstuff.SigAggregator

	if !enableRandomBeacon {
		// if random beacon is disabled, create staking sig provider
		for i := 0; i < n; i++ {
			signer, err := newStakingSigProvider(ps, encoding.ConsensusVoteTag, ids[i], stakingKeys[i])
			require.NoError(t, err)
			signers[i] = signer
			if i == 0 {
				verifier = signer
				aggregator = signer
			}
		}
		return ids, signers, verifier, aggregator, stakingKeys, nil, nil
	}

	randomBKeys, dkgPubData, err := addRandomBeaconPrivateKeys(t, ids)
	for i := 0; i < n; i++ {
		signer, err := newRandomBeaconSigProvider(ps, dkgPubData, encoding.ConsensusVoteTag, ids[i], stakingKeys[i], randomBKeys[i])
		require.NoError(t, err)
		signers[i] = signer
		if i == 0 {
			verifier = signer
			aggregator = signer
		}
	}
	return ids, signers, verifier, aggregator, stakingKeys, randomBKeys, dkgPubData
}
