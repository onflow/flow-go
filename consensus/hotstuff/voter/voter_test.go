package voter

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProduceVote(t *testing.T) {
	t.Run("should vote for block", testVoterOK)
	t.Run("should not vote for unsafe block", testUnsafe)
	t.Run("should not vote for block with its view below the current view", testBelowVote)
	t.Run("should not vote for block with its view above the current view", testAboveVote)
	t.Run("should not vote for block with the same view as the last voted view", testEqualLastVotedView)
	t.Run("should not vote for block with its view below the last voted view", testBelowLastVotedView)
	t.Run("should not vote for the same view again", testVotingAgain)
	t.Run("should not vote while not a committee member", testVotingWhileNonCommitteeMember)
}

func createVoter(t *testing.T, blockView uint64, lastVotedView uint64, isBlockSafe, isCommitteeMember bool) (*model.Block, *model.Vote, *Voter) {
	block := helper.MakeBlock(t, helper.WithBlockView(blockView))
	expectVote := makeVote(block)

	forks := &mocks.ForksReader{}
	forks.On("IsSafeBlock", block).Return(isBlockSafe)

	persist := &mocks.Persister{}
	persist.On("PutVoted", mock.Anything).Return(nil)

	signer := &mocks.SignerVerifier{}
	signer.On("CreateVote", mock.Anything).Return(expectVote, nil)

	committee := &mocks.Committee{}
	me := unittest.IdentityFixture()
	committee.On("Self").Return(me.NodeID, nil)
	if isCommitteeMember {
		committee.On("Identity", mock.Anything, me.NodeID).Return(me, nil)
	} else {
		committee.On("Identity", mock.Anything, me.NodeID).Return(nil, model.ErrInvalidSigner)
	}

	voter := New(signer, forks, persist, committee, lastVotedView)
	return block, expectVote, voter
}

func testVoterOK(t *testing.T) {
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(3), uint64(2), true, true

	// create voter
	block, expectVote, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	// produce vote
	vote, err := voter.ProduceVoteIfVotable(block, curView)

	require.NoError(t, err)
	require.Equal(t, vote, expectVote)
}

func testUnsafe(t *testing.T) {
	// create unsafe block
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(3), uint64(2), false, true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not safe")
	require.True(t, model.IsNoVoteError(err))
}

func testBelowVote(t *testing.T) {
	// curView < blockView
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(2), uint64(2), true, true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expecting block for current view")
}

func testAboveVote(t *testing.T) {
	// curView > blockView
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(4), uint64(2), true, true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expecting block for current view")
}

func testEqualLastVotedView(t *testing.T) {
	// curView == lastVotedView
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(3), uint64(3), true, true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be larger than the last voted view")
}

func testBelowLastVotedView(t *testing.T) {
	// curView < lastVotedView
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(3), uint64(4), true, true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be larger than the last voted view")
}

func testVotingAgain(t *testing.T) {
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(3), uint64(2), true, true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	// produce vote
	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.NoError(t, err)

	// produce vote again for the same view
	_, err = voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be larger than the last voted view")
}

func testVotingWhileNonCommitteeMember(t *testing.T) {
	blockView, curView, lastVotedView, isBlockSafe, isCommitteeMember := uint64(3), uint64(3), uint64(2), true, false

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe, isCommitteeMember)

	// produce vote
	_, err := voter.ProduceVoteIfVotable(block, curView)

	require.Error(t, err)
	require.True(t, model.IsNoVoteError(err))
}

func makeVote(block *model.Block) *model.Vote {
	return &model.Vote{
		BlockID: block.BlockID,
		View:    block.View,
		SigData: nil, // signature doesn't matter in this test case
	}
}
