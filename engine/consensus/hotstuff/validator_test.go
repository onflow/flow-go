package hotstuff_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/test"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	hs "github.com/dapperlabs/flow-go/model/hotstuff"
)

func TestValidateVote(t *testing.T) {
	// Happy Path
	t.Run("A valid vote should be valid", testVoteOK)
	// Unhappy Path
	t.Run("A vote with invalid view should be rejected", testVoteInvalidView)
	t.Run("A vote with invalid block ID should be rejected", testVoteInvalidBlock)
	t.Run("A vote from unstaked node should be rejected", testVoteUnstakedNode)
	t.Run("A vote with invalid staking sig should be rejected", testVoteInvalidStaking)
	t.Run("A vote with invalid random beacon sig should be rejected", testVoteInvalidRandomB)
}

func TestValidateQC(t *testing.T) {
	// Happy Path
	t.Run("A valid QC should be valid", testQCOK)
	// Unhappy Path
	t.Run("A QC with invalid blockID should be rejected", testQCInvalidBlock)
	t.Run("A QC with invalid view should be rejected", testQCInvalidView)
	t.Run("A QC from unstaked nodes should be rejected", testQCHasUnstakedSigner)
	t.Run("A QC with insufficient stakes should be rejected", testQCHasInsufficentStake)
	t.Run("A QC with invalid staking sig should be rejected", testQCHasInvalidStakingSig)
	t.Run("A QC with invalid random beacon sig should be rejected", testQCHasInvalidRandomBSig)
}

func TestValidateProposal(t *testing.T) {
	// Happy Path
	t.Run("A valid proposal should be accepted", testProposalOK)
	// Unhappy Path
	t.Run("A proposal with invalid view should be rejected", testProposalInvalidView)
	t.Run("A proposal with invalid block ID should be rejected", testProposalInvalidBlock)
	t.Run("A proposal from unstaked node should be rejected", testProposalUnstakedNode)
	t.Run("A proposal with invalid staking sig should be rejected", testProposalInvalidStaking)
	t.Run("A proposal with invalid random beacon sig should be rejected", testProposalInvalidRandomB)
	t.Run("A proposal from the wrong leader should be rejected", testProposalWrongLeader)
	t.Run("A proposal with a QC pointing to a non-existing block, but equal to finalized view should be rejected", testProposalWrongParentEqual)
	t.Run("A proposal with a QC pointing to a non-existing block, but above finalized view should be rejected", testProposalWrongParentAbove)
	t.Run("A proposal with a QC pointing to a non-existing block, but below finalized view should be unverifiable", testProposalWrongParentBelow)
	t.Run("A proposal with a invalid QC should be rejected", testProposalInvalidQC)
}

func testVoteOK(t *testing.T) {
	vs, signers, ids, _, _ := createValidators(t, 3)
	v, signer, id := vs[0], signers[0], ids[0]

	// make block
	block := test.MakeBlock(3)

	// make vote
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// validate vote
	signerID, err := v.ValidateVote(vote, block)
	require.NoError(t, err)

	// validate signerID
	require.Equal(t, signerID, id)
}

func testVoteInvalidView(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v, signer := vs[0], signers[0]

	// make block
	block := test.MakeBlock(3)

	// make vote
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// signature is valid, but View is invalid
	vote.View = 4

	// validate vote
	_, err = v.ValidateVote(vote, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong view number")
}

func testVoteInvalidBlock(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v, signer := vs[0], signers[0]

	// make block
	block := test.MakeBlock(3)

	// make vote
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// signature is valid, but BlockID is invalid
	vote.BlockID = flow.HashToID([]byte{1, 2, 3})

	// validate vote
	_, err = v.ValidateVote(vote, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong block ID")
}

func testVoteUnstakedNode(t *testing.T) {
	vs, signers, ids, _, _ := createValidators(t, 3)
	v, signer, id := vs[0], signers[0], ids[0]

	// make block
	block := test.MakeBlock(3)

	// make vote
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// signer is now unstaked
	id.Stake = 0

	// validate vote
	_, err = v.ValidateVote(vote, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a staked node")
}

func testVoteInvalidStaking(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v, signer := vs[0], signers[0]

	// make block
	block := test.MakeBlock(3)

	// make vote
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// make vote from others
	voteFromOthers, err := signers[1].VoteFor(block)
	require.NoError(t, err)

	// take staking signature from others' vote
	vote.Signature.StakingSignature = voteFromOthers.Signature.StakingSignature

	// validate vote
	_, err = v.ValidateVote(vote, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid staking signature")
}

func testVoteInvalidRandomB(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v, signer := vs[0], signers[0]

	// make block
	block := test.MakeBlock(3)

	// make vote
	vote, err := signer.VoteFor(block)
	require.NoError(t, err)

	// make vote from others
	voteFromOthers, err := signers[1].VoteFor(block)
	require.NoError(t, err)

	// take random beacon signature from others' vote
	vote.Signature.RandomBeaconSignature = voteFromOthers.Signature.RandomBeaconSignature

	// validate vote
	_, err = v.ValidateVote(vote, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid random beacon signature")
}

func testQCOK(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	err = v.ValidateQC(qc, block)
	require.NoError(t, err)
}

func testQCInvalidBlock(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// replace with invalid blockID
	block.BlockID = flow.HashToID([]byte{1, 2})

	// validate QC
	err = v.ValidateQC(qc, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "qc.BlockID")
}

func testQCInvalidView(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// replace with invalid view
	block.View = 5

	// validate QC
	err = v.ValidateQC(qc, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "qc.View")
}

func testQCHasUnstakedSigner(t *testing.T) {
	vs, signers, ids, _, _ := createValidators(t, 3)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// one signer is unstaked
	ids[2].Stake = 0

	// validate QC
	err = v.ValidateQC(qc, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unstaked identities")
}

func testQCHasInsufficentStake(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})

	// only take 2 signers
	aggsig.SignerIDs = []flow.Identifier{
		aggsig.SignerIDs[0],
		aggsig.SignerIDs[1],
	}

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	err = v.ValidateQC(qc, block)
	// a valid QC require `2/3 + 1` of the stakes, this QC only has 2/3 stakes, which is insufficient
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient stake")
}

func testQCHasInvalidRandomBSig(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 3)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})
	require.NoError(t, err)

	// making bad votes
	invalidBlock := test.MakeBlock(4)
	badvote1, err := signers[0].VoteFor(invalidBlock)
	require.NoError(t, err)
	badvote2, err := signers[1].VoteFor(invalidBlock)
	require.NoError(t, err)
	badvote3, err := signers[2].VoteFor(invalidBlock)
	require.NoError(t, err)

	// aggregate bad votes to make a bad random beacon sig
	badaggsig, err := signers[0].Aggregate(invalidBlock, []*hs.SingleSignature{badvote1.Signature, badvote2.Signature, badvote3.Signature})
	require.NoError(t, err)

	// the random beacon signatures and the signers are now mismatch
	aggsig.RandomBeaconSignature = badaggsig.RandomBeaconSignature

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	err = v.ValidateQC(qc, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reconstructed random beacon signature in QC is invalid")
}

func testQCHasInvalidStakingSig(t *testing.T) {
	vs, signers, _, _, _ := createValidators(t, 4)
	v := vs[0]

	// make block
	block := test.MakeBlock(3)

	// make votes
	vote1, err := signers[0].VoteFor(block)
	require.NoError(t, err)
	vote2, err := signers[1].VoteFor(block)
	require.NoError(t, err)
	vote3, err := signers[2].VoteFor(block)
	require.NoError(t, err)
	vote4, err := signers[3].VoteFor(block)
	require.NoError(t, err)

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})
	require.NoError(t, err)

	// manually aggregate sigs from different votes (1,2,4)
	badaggsig, err := signers[0].Aggregate(block, []*hs.SingleSignature{vote1.Signature, vote2.Signature, vote4.Signature})
	require.NoError(t, err)

	// the staking signatures and the signers are now mismatch
	aggsig.StakingSignatures = badaggsig.StakingSignatures

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	err = v.ValidateQC(qc, block)
	require.Error(t, err)
	require.Contains(t, err.Error(), "aggregated staking signature in QC is invalid")
}

func testProposalOK(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks has the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	err = v.ValidateProposal(proposal)
	require.NoError(t, err)
}

func testProposalInvalidView(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks has the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	// signer is still the leader of view 7, but View is invalid
	proposal.Block.View = 7

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid block")
}

func testProposalInvalidBlock(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks has the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	// invalid block id
	proposal.Block.BlockID = flow.HashToID([]byte{1, 2, 3})

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid block")
}

func testProposalUnstakedNode(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, ids, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v, id := vs[index], ids[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks has the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	// signer is now unstaked
	id.Stake = 0

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a staked node")
}

func testProposalInvalidStaking(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// make a different proposal to take its signature
	proposal7, err := bp.MakeBlockProposal(qc, 7)

	// invalid staking sig
	proposal.StakingSignature = proposal7.StakingSignature

	// mock forks has the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid staking signature")
}

func testProposalInvalidRandomB(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// make a different proposal to take its signature
	proposal7, err := bp.MakeBlockProposal(qc, 7)

	// invalid random beacon sig
	proposal.RandomBeaconSignature = proposal7.RandomBeaconSignature

	// mock forks has the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid random beacon signature")
}

func testProposalWrongLeader(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// make a view where signer is not the leader of
	wrongViewAsLeader := uint64(5)

	// build proposal
	proposal, err := bp.MakeBlockProposal(qc, wrongViewAsLeader)

	// mock forks doesn't have the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	// validate proposal
	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong leader")
}

func testProposalWrongParentEqual(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	// create validators and other dependencies
	vs, signers, _, viewstates, f := createValidators(t, n)

	// make a block producer and a qc of view 3
	bp, qc, _ := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks doesn't have the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(nil, false)

	// proposal.Block.QC.View equals to finalized view
	f.On("FinalizedView").Return(qcview)

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Equal(t, err, &hs.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockID})
}

func testProposalWrongParentAbove(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	vs, signers, _, viewstates, f := createValidators(t, n)
	// make a block producer and a qc of view 3
	bp, qc, _ := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks doesn't have the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(nil, false)

	// proposal.Block.QC.View is above finalized view
	f.On("FinalizedView").Return(uint64(qcview - 1))

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Equal(t, err, &hs.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockID})
}

func testProposalWrongParentBelow(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	vs, signers, _, viewstates, f := createValidators(t, n)
	// make a block producer and a qc of view 3
	bp, qc, _ := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks doesn't have the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(nil, false)

	// proposal.Block.QC.View is below finalized view
	f.On("FinalizedView").Return(uint64(5))

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Equal(t, err, hs.ErrUnverifiableBlock)
}

func testProposalInvalidQC(t *testing.T) {
	n, qcview, view := 3, uint64(3), uint64(4)

	// index-th node as the signer
	index := leaderOfView(n, view)

	vs, signers, _, viewstates, f := createValidators(t, n)
	// make a block producer and a qc of view 3
	bp, qc, block := makeBlockProducerAndQC(t, signers, viewstates, index, qcview)

	// validator
	v := vs[index]

	// build proposal for view 4
	proposal, err := bp.MakeBlockProposal(qc, view)
	require.NoError(t, err)

	// mock forks doesn't have the parent block
	f.On("GetBlock", proposal.Block.QC.BlockID).Return(block, true)

	// make QC invalid
	proposal.Block.QC.View = 10

	err = v.ValidateProposal(proposal)
	require.Error(t, err)
	require.Contains(t, err.Error(), "qc.View")
}

type FakeBuilder struct{}

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

// create validator for `n` nodes in a cluster with the index-th node as the signer
func createValidators(t *testing.T, n int) ([]*hotstuff.Validator, []*signature.RandomBeaconAwareSigProvider, flow.IdentityList, []*hotstuff.ViewState, *mocks.ForksReader) {
	ps, ids := test.NewProtocolState(t, n)

	stakingKeys, err := test.AddStakingPrivateKeys(ids)
	require.NoError(t, err)

	randomBKeys, dkgPubData, err := test.AddRandomBeaconPrivateKeys(t, ids)

	signers := make([]*signature.RandomBeaconAwareSigProvider, n)
	viewstates := make([]*hotstuff.ViewState, n)
	validators := make([]*hotstuff.Validator, n)

	f := &mocks.ForksReader{}

	for i := 0; i < n; i++ {
		// create signer
		signer, err := test.NewRandomBeaconSigProvider(ps, dkgPubData, ids[i], stakingKeys[i], randomBKeys[i])
		require.NoError(t, err)
		signers[i] = signer

		// create view state
		vs, err := hotstuff.NewViewState(ps, dkgPubData, ids[i].NodeID, filter.HasRole(flow.RoleConsensus))
		require.NoError(t, err)
		viewstates[i] = vs

		// create validator
		v := hotstuff.NewValidator(vs, f, signer)
		validators[i] = v
	}
	return validators, signers, ids, viewstates, f
}

func createBlockProducer(t *testing.T, signer hotstuff.Signer, vs *hotstuff.ViewState) *hotstuff.BlockProducer {
	builder := &FakeBuilder{}
	bp, err := hotstuff.NewBlockProducer(signer, vs, builder)
	require.NoError(t, err)
	return bp
}

// makeBlockProducerAndQC  initialized a BlockProducer, block and qc pointing to this block. The qc is constructed from the votes of the provided signers
func makeBlockProducerAndQC(t *testing.T, signers []*signature.RandomBeaconAwareSigProvider, viewstates []*hotstuff.ViewState, index int, view uint64) (*hotstuff.BlockProducer, *hs.QuorumCertificate, *hs.Block) {
	signer, viewstate := signers[index], viewstates[index]
	n := len(signers)

	// make block
	block := test.MakeBlock(int(view))

	// make votes
	sigs := make([]*hs.SingleSignature, n)
	for i := 0; i < n; i++ {
		vote, err := signers[i].VoteFor(block)
		require.NoError(t, err)
		sigs[i] = vote.Signature
	}

	// manually aggregate sigs
	aggsig, err := signer.Aggregate(block, sigs)
	require.NoError(t, err)

	// make QC
	qc := &hs.QuorumCertificate{
		View:                block.View,
		BlockID:             block.BlockID,
		AggregatedSignature: aggsig,
	}

	// create block builder
	bp := createBlockProducer(t, signer, viewstate)

	return bp, qc, block
}

func leaderOfView(n int, view uint64) int {
	// referred to engine/consensus/hotstuff/viewstate.go#roundRobin
	return int(view) % n
}
