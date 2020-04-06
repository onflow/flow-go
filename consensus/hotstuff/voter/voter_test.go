package voter

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

func TestProduceVote(t *testing.T) {
	t.Run("should vote for block", testVoterOK)
	t.Run("should not vote for unsafe block", testUnsafe)
	t.Run("should not vote for block with its view below the current view", testBelowVote)
	t.Run("should not vote for block with its view above the current view", testAboveVote)
	t.Run("should not vote for block with the same view as the last voted view", testEqualLastVotedView)
	t.Run("should not vote for block with its view below the last voted view", testBelowLastVotedView)
	t.Run("should not vote for the same view again", testVotingAgain)
}

func createVoter(t *testing.T, blockView uint64, lastVotedView uint64, isBlockSafe bool) (*model.Block, *model.Vote, *Voter) {
	block := helper.MakeBlock(t, helper.WithBlockView(blockView))
	expectVote := makeVote(block)

	forks := &mocks.ForksReader{}
	forks.On("IsSafeBlock", block).Return(isBlockSafe)

	signer := &mocks.Signer{}
	signer.On("CreateVote", mock.Anything).Return(expectVote, nil)

	voter := New(signer, forks, lastVotedView)
	return block, expectVote, voter
}

func testVoterOK(t *testing.T) {
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(3), uint64(2), true

	// create voter
	block, expectVote, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	// produce vote
	vote, err := voter.ProduceVoteIfVotable(block, curView)

	require.NoError(t, err)
	require.Equal(t, vote, expectVote)
}

func testUnsafe(t *testing.T) {
	// create unsafe block
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(3), uint64(2), false

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not safe")
}

func testBelowVote(t *testing.T) {
	// curView < blockView
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(2), uint64(2), true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not for current view")
}

func testAboveVote(t *testing.T) {
	// curView > blockView
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(4), uint64(2), true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not for current view")
}

func testEqualLastVotedView(t *testing.T) {
	// curView == lastVotedView
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(3), uint64(3), true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not above the last voted view")
}

func testBelowLastVotedView(t *testing.T) {
	// curView < lastVotedView
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(3), uint64(4), true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	_, err := voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not above the last voted view")
}

func testVotingAgain(t *testing.T) {
	blockView, curView, lastVotedView, isBlockSafe := uint64(3), uint64(3), uint64(2), true

	// create voter
	block, _, voter := createVoter(t, blockView, lastVotedView, isBlockSafe)

	// produce vote
	_, err := voter.ProduceVoteIfVotable(block, curView)

	require.NoError(t, err)

	// produce vote again for the same view
	_, err = voter.ProduceVoteIfVotable(block, curView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not above the last voted view")
}

func makeVote(block *model.Block) *model.Vote {
	return &model.Vote{
		BlockID: block.BlockID,
		View:    block.View,
		SigData: nil, // signature doesn't matter in this test case
	}
}
