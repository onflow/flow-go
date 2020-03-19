package signature_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/test"
	"github.com/dapperlabs/flow-go/model/encoding"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
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

func TestAggregationPerformance(t *testing.T) {
	t.Run("check how long it takes to reconstruct 254 votes", testAggregationPerformance)
}

func testOKSigningRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBeaconKeys, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create proposal
	block := test.MakeBlock(1)
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
	valid, err = signature.NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(proposal.RandomBeaconSignature, block, randomBeaconKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}

func testOKSigningVoteRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create vote
	block := test.MakeBlock(1)
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
	valid, err = signature.NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, block, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}

func testProposalIsVote(t *testing.T) {
	_, signers, _, _, _, _, _ := test.MakeSignerAndVerifier(t, 4, false)
	signer := signers[0]

	// make a vote from proposal
	block := test.MakeBlock(1)
	proposal, err := signer.Propose(block)
	require.NoError(t, err)

	// make a vote
	proposerVote := proposal.ProposerVote()
	vote, err := signer.VoteFor(block)

	assert.Equal(t, proposerVote.Signature.StakingSignature, vote.Signature.StakingSignature)
	assert.Equal(t, proposerVote.Signature.RandomBeaconSignature, vote.Signature.RandomBeaconSignature)
}

func testWrongBlockIDRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create vote
	block, withWrongBlockID := test.MakeBlock(1), test.MakeBlock(2)
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
	valid, err = signature.NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, withWrongBlockID, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongViewRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create vote
	block, withWrongView := test.MakeBlock(1), test.MakeBlock(2)
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
	valid, err = signature.NewRandomBeaconAwareSigVerifier().VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, withWrongView, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

type TestRandomBeaconSigVerifier struct {
	// the hasher for signer random beacon signature
	randomBeaconHasher crypto.Hasher
}

func NewTestRandomBeaconSigVerifier(tag string) TestRandomBeaconSigVerifier {
	return TestRandomBeaconSigVerifier{
		randomBeaconHasher: crypto.NewBLS_KMAC(tag),
	}
}

func (v *TestRandomBeaconSigVerifier) VerifyRandomBeaconSig(sigShare crypto.Signature, block *model.Block, signerPubKey crypto.PublicKey) (bool, error) {
	msg := signature.BlockToBytesForSign(block)
	valid, err := signerPubKey.Verify(sigShare, msg, v.randomBeaconHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify random beacon signature: %w", err)
	}
	return valid, nil
}

func (v *TestRandomBeaconSigVerifier) VerifyRandomBeaconThresholdSig(sig crypto.Signature, block *model.Block, groupPubKey crypto.PublicKey) (bool, error) {
	msg := signature.BlockToBytesForSign(block)
	// the reconstructed signature is also a BLS signature which can be verified by the group public key
	valid, err := groupPubKey.Verify(sig, msg, v.randomBeaconHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify reconstructed random beacon sig: %w", err)
	}
	return valid, nil
}

func testWrongKMacTagRBEnabled(t *testing.T) {
	_, signers, _, _, _, randomBKeys, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create vote
	block := test.MakeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify random beacon sig by a unstaked verifier
	verifier := NewTestRandomBeaconSigVerifier("wrongtag")
	valid, err := verifier.VerifyRandomBeaconThresholdSig(vote.Signature.RandomBeaconSignature, block, randomBKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongSignerRBEnabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, randomBKeys, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create vote
	block := test.MakeBlock(1)
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
	_, signers, verifier, _, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create proposal
	block := test.MakeBlock(1)
	proposal, err := signers[0].Propose(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(proposal.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}
func testOKSigningVoteRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create vote
	block := test.MakeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)
}

func testWrongBlockIDRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create vote
	block, withWrongBlockID := test.MakeBlock(1), test.MakeBlock(2)
	withWrongBlockID.View = block.View // withWrongBlockID has the different BlockID
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, withWrongBlockID, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongViewRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create vote
	block, withWrongView := test.MakeBlock(1), test.MakeBlock(2)
	withWrongView.BlockID = block.BlockID // withWrongView has the different View
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, withWrongView, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongKMacTagRBDisabled(t *testing.T) {
	ps, ids := test.NewProtocolState(t, 4)
	stakingKeys, err := test.AddStakingPrivateKeys(ids)
	require.NoError(t, err)

	var signer hotstuff.Signer
	var verifier hotstuff.SigVerifier

	// creating signer
	signer, err = test.NewStakingProvider(ps, "wrongtag", ids[0], stakingKeys[0]) // bad signer init with a wrong tag
	require.NoError(t, err)
	verifier, err = test.NewStakingProvider(ps, encoding.CollectorVoteTag, ids[1], stakingKeys[1])
	require.NoError(t, err)

	// create vote
	block := test.MakeBlock(1)
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, stakingKeys[0].PublicKey())
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testWrongSignerRBDisabled(t *testing.T) {
	_, signers, verifier, _, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create vote
	block := test.MakeBlock(1)
	vote, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	wrongStakingPubKey := stakingKeys[1].PublicKey()

	// verify staking sig
	valid, err := verifier.VerifyStakingSig(vote.Signature.StakingSignature, block, wrongStakingPubKey)
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testAggregateRBEnabled(t *testing.T) {
	_, signers, verifier, aggregator, stakingKeys, _, dkg := test.MakeSignerAndVerifier(t, 4, true)

	// create block
	block := test.MakeBlock(1)

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
	_, signers, _, aggregator, _, _, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create block
	block1, block2 := test.MakeBlock(1), test.MakeBlock(2)

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
	_, signers, verifier, aggregator, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create block
	block := test.MakeBlock(1)

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
	_, _, _, _, _, _, dkg2 := test.MakeSignerAndVerifier(t, 5, true)
	// verify aggregated random beacon sig
	valid, err = verifier.VerifyRandomBeaconThresholdSig(aggsig.RandomBeaconSignature, block, dkg2.GroupPubKey)
	// fail because the group public key was wrong
	require.NoError(t, err)
	assert.Equal(t, false, valid)
}

func testAggregateWithInsufficientSharesRBEnabled(t *testing.T) {
	_, signers, _, aggregator, _, _, _ := test.MakeSignerAndVerifier(t, 4, true)

	// create block
	block := test.MakeBlock(1)

	// create votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)

	// aggregate
	_, err = aggregator.Aggregate(block, []*model.SingleSignature{vote1.Signature})
	// needs two shares, but got only one
	assert.Error(t, err)
}

func testAggregateRBDisabled(t *testing.T) {
	_, signers, verifier, aggregator, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create block
	block := test.MakeBlock(1)

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
	_, signers, _, aggregator, _, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create block
	block1, block2 := test.MakeBlock(1), test.MakeBlock(2)

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
	_, signers, verifier, aggregator, stakingKeys, _, _ := test.MakeSignerAndVerifier(t, 4, false)

	// create block
	block := test.MakeBlock(1)

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

func testAggregationPerformance(t *testing.T) {
	_, signers, _, aggregator, stakingKeys, _, dkg := test.MakeSignerAndVerifierWithFakeDKG(t, 254)
	require.NotNil(t, dkg)

	threshold := len(signers)*2/3 + 1

	// create block
	block := test.MakeBlock(1)

	// create votes and signatures
	sigs := make([]*model.SingleSignature, 0, threshold)
	pubkeys := make([]crypto.PublicKey, 0, threshold)

	for i := 0; i < threshold; i++ {
		vote, err := signers[i].VoteFor(block)
		require.NoError(t, err)
		sigs = append(sigs, vote.Signature)
		pubkeys = append(pubkeys, stakingKeys[i].PublicKey())
	}

	// aggregate
	timer := time.Now()
	// The aggregation will fail, because we used fake random beacon private keys.
	// However it's OK for the aggregation to fail, because the failure comes from verifying the aggregated
	// signature. Since we only care about the timing, by the time the verification has failed, the aggregation
	// has been done. The timing result should be the same as using the real random beacon private keys.

	// Why using fake random beacon private keys here?
	// because generating 254 DKG keys is very slow on one machine.
	// TODO: we could generate 254 DKG keys on power machines, and store the generated keys in files,
	// and load them to tests
	aggregator.Aggregate(block, sigs)
	// log performance
	duration := time.Now().Sub(timer)

	t.Logf("Aggregating %v signatures for %v nodes took: %s\n", threshold, len(signers), duration)
}
