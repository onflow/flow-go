// +build relic

package voteaggregator

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "github.com/dapperlabs/flow-go/consensus/hotstuff/mock"
	hotmodel "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	mockdist "github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	protocol "github.com/dapperlabs/flow-go/protocol/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// threshold stake would be 5,
// meaning that it is enough to build a QC when receiving 5 votes
const VALIDATOR_SIZE = 7

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should not be built when receiving the block because of insufficient votes
func TestOnlyReceiveBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_ = va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.False(t, built)
	require.Nil(t, qc)
	require.NoError(t, err)
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should not be built when receiving insufficient votes
func TestReceiveBlockBeforeInsufficientVotes(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_ = va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 3; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		qc, built, err = va.StoreVoteAndBuildQC(vote, bp.Block)
		require.False(t, built)
		require.Nil(t, qc)
		require.NoError(t, err)
	}
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should be built when receiving the block and the 4th vote (the block is counted as one vote)
func TestReceiveBlockBeforeSufficientVotes(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_ = va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 4; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		qc, built, err = va.StoreVoteAndBuildQC(vote, bp.Block)
		if i == 3 {
			require.NoError(t, err)
			require.True(t, built)
			require.NotNil(t, qc)
		}
	}
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// the same QC should be returned when receiving the block and the 5th vote
func TestReceiveVoteAfterQCBuilt(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_ = va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	var qc *hotmodel.QuorumCertificate
	for i := 0; i < 4; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		qc, _, _ = va.StoreVoteAndBuildQC(vote, bp.Block)
	}
	finalVote := newMockVote(testView, bp.Block.BlockID, ids[4].NodeID)
	finalQC, built, err := va.StoreVoteAndBuildQC(finalVote, bp.Block)
	require.NoError(t, err)
	require.NotNil(t, finalQC)
	require.True(t, built)
	require.Equal(t, qc, finalQC)
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// a QC should not be built when received the block and 3 votes (still insufficient) and a vote for a different block
func TestReceiveVoteForDifferentBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_ = va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 3; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		_, _, _ = va.StoreVoteAndBuildQC(vote, bp.Block)
	}
	bp2 := newMockBlock(testView, ids[len(ids)-2].NodeID)
	_ = va.StoreProposerVote(bp2.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp2.Block)
	voteForDifferentBlock := newMockVote(testView, bp2.Block.BlockID, ids[4].NodeID)
	qc, built, err := va.StoreVoteAndBuildQC(voteForDifferentBlock, bp2.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 3 votes first, a QC should not be built when receiving the block because of insufficient votes
func TestReceiveInsufficientVotesBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	for i := 0; i < 3; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		ok := va.StorePendingVote(vote)
		require.True(t, ok)
	}
	ok := va.StoreProposerVote(bp.ProposerVote())
	require.True(t, ok)
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.False(t, built)
	require.Nil(t, qc)
	require.NoError(t, err)
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 6 votes first, a QC should be built when receiving the block
func TestReceiveSufficientVotesBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_, _, err := va.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 6; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		ok := va.StorePendingVote(vote)
		require.True(t, ok)
	}
	_ = va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.NoError(t, err)
	require.NotNil(t, qc)
	require.True(t, built)
}

// UNHAPPY PATH
// highestPrunedView is 10, receive a vote with view 5 without the block
// the vote should not be stored
func TestStaleVoteWithoutBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	va.PruneByView(10)
	testView := uint64(5)
	vote := newMockVote(testView, unittest.IdentifierFixture(), ids[0].NodeID)
	ok := va.StorePendingVote(vote)
	require.False(t, ok)
}

// UNHAPPY PATH
// build qc without storing proposer vote
func TestBuildQCWithoutProposerVote(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.Errorf(t, err, fmt.Sprintf("could not get proposer vote for block: %x", bp.Block.BlockID))
	require.False(t, built)
	require.Nil(t, qc)
}

// UNHAPPY PATH
// highestPrunedView is 10, receive a block with view 5 which should be ignored
func TestStaleProposerVote(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	va.PruneByView(10)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	ok := va.StoreProposerVote(bp.ProposerVote())
	require.False(t, ok)
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.Errorf(t, err, fmt.Sprintf("could not get proposer vote for block: %x", bp.Block.BlockID))
	require.False(t, built)
	require.Nil(t, qc)
}

// UNHAPPY PATH
// store one vote in the memory
// receive another vote with the same voter and the same view
// should trigger notifier
func TestDoubleVote(t *testing.T) {
	testview := uint64(5)
	va, ids := newMockVoteAggregator(t)
	notifier := &mockdist.Consumer{}
	// mock two proposals at the same view
	bp1 := newMockBlock(testview, ids[0].NodeID)
	bp2 := newMockBlock(testview, ids[1].NodeID)
	// mock double voting
	vote1 := newMockVote(testview, bp1.Block.BlockID, ids[2].NodeID)
	vote2 := newMockVote(testview, bp2.Block.BlockID, ids[2].NodeID)
	// set notifier
	notifier.On("OnDoubleVotingDetected", vote1, vote2).Return().Once()
	va.notifier = notifier

	va.StoreProposerVote(bp1.ProposerVote())
	va.StoreProposerVote(bp2.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp1.Block)
	_, _, _ = va.BuildQCOnReceivedBlock(bp2.Block)
	qc, built, err := va.StoreVoteAndBuildQC(vote1, bp1.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
	qc, built, err = va.StoreVoteAndBuildQC(vote2, bp2.Block)
	notifier.AssertExpectations(t)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
}

// INVALID VOTES
// receive 4 invalid votes, and then the block, no QC should be built
// should trigger the notifier when converting pending votes
func TestInvalidVotesOnly(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	notifier := &mockdist.Consumer{}
	va.notifier = notifier
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		// vote view is invalid
		vote := newMockVote(testView-1, bp.Block.BlockID, ids[i].NodeID)
		notifier.On("OnInvalidVoteDetected", vote)
		va.StorePendingVote(vote)
	}
	va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// INVALID VOTES
// receive 1 invalid vote, and 4 valid votes, and then the block, a QC should be built
func TestVoteMixtureBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	notifier := &mockdist.Consumer{}
	va.notifier = notifier
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	// testing invalid pending votes
	for i := 0; i < 5; i++ {
		var vote *hotmodel.Vote
		if i == 0 {
			// vote view is invalid
			vote = newMockVote(testView-1, bp.Block.BlockID, ids[i].NodeID)
			notifier.On("OnInvalidVoteDetected", vote)
		} else {
			vote = newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		}
		va.StorePendingVote(vote)
	}
	va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.NotNil(t, qc)
	require.True(t, built)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// INVALID VOTES
// receive the block, and 3 valid vote, and 1 invalid vote, no QC should be built
func TestVoteMixtureAfterBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	notifier := &mockdist.Consumer{}
	va.notifier = notifier
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		var vote *hotmodel.Vote
		if i < 3 {
			vote = newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		} else {
			// vote view is invalid
			vote = newMockVote(testView-1, bp.Block.BlockID, ids[i].NodeID)
			notifier.On("OnInvalidVoteDetected", vote)
		}
		qc, built, err := va.StoreVoteAndBuildQC(vote, bp.Block)
		require.Nil(t, qc)
		require.False(t, built)
		require.NoError(t, err)
		notifier.AssertExpectations(t)
	}
}

// DUPLICATION
// receive the block, and the same valid votes for 5 times, no QC should be built
func TestDuplicateVotesAfterBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	vote := newMockVote(testView, bp.Block.BlockID, ids[1].NodeID)
	for i := 0; i < 4; i++ {
		qc, built, err := va.StoreVoteAndBuildQC(vote, bp.Block)
		require.Nil(t, qc)
		require.False(t, built)
		require.NoError(t, err)
		// only one vote and the primary vote are added
		require.Equal(t, 2, len(va.blockIDToVotingStatus[bp.Block.BlockID].votes))
	}
}

// DUPLICATION
// receive same valid votes for 5 times, then the block, no QC should be built
func TestDuplicateVotesBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	vote := newMockVote(testView, bp.Block.BlockID, ids[1].NodeID)
	for i := 0; i < 4; i++ {
		_ = va.StorePendingVote(vote)
	}
	va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
}

// ORDER
// receive 5 votes, and the block, the QC should contain leader's vote, and the first 4 votes.
func TestVoteOrderAfterBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	var voteList []*hotmodel.Vote
	voteList = append(voteList, bp.ProposerVote())
	for i := 0; i < 5; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		va.StorePendingVote(vote)
		voteList = append(voteList, vote)
	}
	va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	_, exists := va.blockIDToVotingStatus[bp.Block.BlockID].votes[bp.ProposerVote().ID()]
	require.NotNil(t, qc)
	require.True(t, built)
	require.NoError(t, err)
	require.True(t, exists)
	// first five votes including proposer vote should be stored in voting status
	// while the 6th vote shouldn't
	for i := 0; i < 6; i++ {
		_, exists := va.blockIDToVotingStatus[bp.Block.BlockID].votes[voteList[i].ID()]
		if i < 5 {
			require.True(t, exists)
		} else {
			require.False(t, exists)
		}
	}
}

// PRUNE
// receive votes for view 2, 3, 4, 5 without the block
// prune by 4, should have only vote for view 5 left
func TestPartialPruneBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	pruneView := uint64(4)
	var voteList []*hotmodel.Vote
	var blockList []*hotmodel.Block
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(view, unittest.IdentifierFixture())
		vote := newMockVote(view, bp.Block.BlockID, ids[i].NodeID)
		va.StorePendingVote(vote)
		voteList = append(voteList, vote)
		blockList = append(blockList, bp.Block)
	}
	// before pruning
	_, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(va)
	require.Equal(t, 4, viewToBlockLen)
	require.Equal(t, 4, viewToVoteLen)
	require.Equal(t, 4, pendingVoteLen)
	// after pruning
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 1, viewToBlockLen)
	require.Equal(t, 1, viewToVoteLen)
	require.Equal(t, 1, pendingVoteLen)
	// the remaining vote should be the vote that has view at 5
	lastVote := voteList[len(voteList)-1]
	require.Equal(t, uint64(5), lastVote.View)
	_, exists := va.viewToVoteID[uint64(5)]
	require.True(t, exists)
	_, exists = va.viewToBlockIDSet[uint64(5)]
	require.True(t, exists)
	_, exists = va.pendingVotes.votes[lastVote.BlockID].voteMap[lastVote.ID()]
	require.True(t, exists)
}

// PRUNE
// receive the block and the votes for view 2, 3, 4, 5
// prune by 4, should have only vote for view 5 left
func TestPartialPruneAfterBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	pruneView := uint64(4)
	var voteList []*hotmodel.Vote
	var blockList []*hotmodel.Block
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(view, ids[i].NodeID)
		va.StoreProposerVote(bp.ProposerVote())
		_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
		vote := newMockVote(view, bp.Block.BlockID, ids[i].NodeID)
		_, _, _ = va.StoreVoteAndBuildQC(vote, bp.Block)
		voteList = append(voteList, vote)
		blockList = append(blockList, bp.Block)
	}
	// before pruning
	_, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(va)
	require.Equal(t, 4, viewToBlockLen)
	require.Equal(t, 4, viewToVoteLen)
	require.Equal(t, 4, votingStatusLen)
	// after pruning
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 1, viewToBlockLen)
	require.Equal(t, 1, viewToVoteLen)
	require.Equal(t, 1, votingStatusLen)
	// the remaining vote should be the vote that has view at 5
	lastVote := voteList[len(voteList)-1]
	require.Equal(t, uint64(5), lastVote.View)
	_, exists := va.viewToVoteID[uint64(5)]
	require.True(t, exists)
	_, exists = va.viewToBlockIDSet[uint64(5)]
	require.True(t, exists)
	_, exists = va.blockIDToVotingStatus[lastVote.BlockID].votes[lastVote.ID()]
	require.True(t, exists)
}

// PRUNE
// receive votes for view 2, 3, 4, 5 without the block
// prune by 6, should have no vote left
func TestFullPruneBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	pruneView := uint64(6)
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		vote := newMockVote(view, unittest.IdentifierFixture(), ids[i].NodeID)
		va.StorePendingVote(vote)
	}
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 0, viewToBlockLen)
	require.Equal(t, 0, viewToVoteLen)
	require.Equal(t, 0, pendingVoteLen)
}

// PRUNE
// receive the block and the votes for view 2, 3, 4, 5
// prune by 6, should have no vote left
func TestFullPruneAfterBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	pruneView := uint64(6)
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(view, ids[i].NodeID)
		va.StoreProposerVote(bp.ProposerVote())
		_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
		vote := newMockVote(view, bp.Block.BlockID, ids[i].NodeID)
		_, _, _ = va.StoreVoteAndBuildQC(vote, bp.Block)
	}
	_, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(va)
	require.Equal(t, 4, viewToBlockLen)
	require.Equal(t, 4, viewToVoteLen)
	require.Equal(t, 4, votingStatusLen)
	// after pruning
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 0, viewToBlockLen)
	require.Equal(t, 0, viewToVoteLen)
	require.Equal(t, 0, votingStatusLen)
}

// PRUNE
// receive votes for view 3, 4, 5 without block
// prune by 2 twice, should have all votes there
func TestNonePruneBeforeBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	pruneView := uint64(2)
	for i := 3; i <= 5; i++ {
		view := uint64(i)
		vote := newMockVote(view, unittest.IdentifierFixture(), ids[i].NodeID)
		va.StorePendingVote(vote)
	}
	_, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(va)
	require.Equal(t, 3, viewToBlockLen)
	require.Equal(t, 3, viewToVoteLen)
	require.Equal(t, 3, pendingVoteLen)
	// after pruning
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 3, viewToBlockLen)
	require.Equal(t, 3, viewToVoteLen)
	require.Equal(t, 3, pendingVoteLen)
	// prune twice
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ = getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 3, viewToBlockLen)
	require.Equal(t, 3, viewToVoteLen)
	require.Equal(t, 3, pendingVoteLen)
}

// PRUNE
// receive the block and the votes for view 3, 4, 5
// prune by 2 twice, should have all votes there
func TestNonePruneAfterBlock(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	pruneView := uint64(2)
	for i := 3; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(view, ids[i].NodeID)
		va.StoreProposerVote(bp.ProposerVote())
		_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
		vote := newMockVote(view, bp.Block.BlockID, ids[i].NodeID)
		_, _, _ = va.StoreVoteAndBuildQC(vote, bp.Block)
	}
	_, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(va)
	require.Equal(t, 3, viewToBlockLen)
	require.Equal(t, 3, viewToVoteLen)
	require.Equal(t, 3, votingStatusLen)
	// after pruning
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 3, viewToBlockLen)
	require.Equal(t, 3, viewToVoteLen)
	require.Equal(t, 3, votingStatusLen)
	// prune twice
	va.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen = getStateLength(va)
	require.Equal(t, pruneView, prunedView)
	require.Equal(t, 3, viewToBlockLen)
	require.Equal(t, 3, viewToVoteLen)
	require.Equal(t, 3, votingStatusLen)
}

// RANDOM BEACON
// if there are 7 nodes, it requires 4 votes for random beacon,
// and receives the block from proposer who has 80% stake,
// then no qc should be built
func TestOnlyProposerVote(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	// first node has 80% of stake
	ids[0].Stake = 5600
	testView := uint64(5)
	bp := newMockBlock(testView, ids[0].NodeID)
	va.StoreProposerVote(bp.ProposerVote())
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
}

// RANDOM BEACON
// if there are 7 nodes, it requires 4 votes for random beacon,
// and receives the block from proposer who has 40% stake,
// and a vote whose sender has 40% stake,
// then no qc should be built
func TestInSufficientRBSig(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	// first two node has 80% of stake in total
	ids[0].Stake = 2800
	ids[1].Stake = 2800
	testView := uint64(5)
	bp := newMockBlock(testView, ids[0].NodeID)
	va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	vote1 := newMockVote(testView, bp.Block.BlockID, ids[1].NodeID)
	qc, built, err := va.StoreVoteAndBuildQC(vote1, bp.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
}

// RANDOM BEACON
// if there are 7 nodes, it requires 4 votes for random beacon,
// and receives the block from proposer who has 40% stake,
// and a vote whose sender has 40% stake,
// and another two votes with random stake,
// then a qc should be built
func TestSufficientRBSig(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	// Define Stakes: first two node has 80% of stake in total
	ids[0].Stake = 40
	ids[1].Stake = 40
	ids[2].Stake = 4
	ids[3].Stake = 4
	ids[4].Stake = 4
	ids[5].Stake = 4
	ids[6].Stake = 4

	testView := uint64(5)
	bp := newMockBlock(testView, ids[0].NodeID)
	va.StoreProposerVote(bp.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp.Block)
	vote1 := newMockVote(testView, bp.Block.BlockID, ids[1].NodeID)
	_, _, _ = va.StoreVoteAndBuildQC(vote1, bp.Block)
	vote2 := newMockVote(testView, bp.Block.BlockID, ids[2].NodeID)
	qc, built, err := va.StoreVoteAndBuildQC(vote2, bp.Block)
	require.NoError(t, err)
	require.False(t, built)
	require.Nil(t, qc)
	vote3 := newMockVote(testView, bp.Block.BlockID, ids[3].NodeID)
	qc, built, err = va.StoreVoteAndBuildQC(vote3, bp.Block)
	require.NoError(t, err)
	require.True(t, built)
	require.NotNil(t, qc)
}

func newMockBlock(view uint64, proposerID flow.Identifier) *hotmodel.Proposal {
	blockHeader := unittest.BlockHeaderFixture()
	blockHeader.View = view
	block := &hotmodel.Block{
		View:       view,
		BlockID:    blockHeader.ID(),
		ProposerID: proposerID,
	}
	sig := crypto.Signature{}
	return &hotmodel.Proposal{
		Block:            block,
		StakingSignature: sig,
	}
}

func newMockVote(view uint64, blockID flow.Identifier, signerID flow.Identifier) *hotmodel.Vote {
	sig := &hotmodel.SingleSignature{
		SignerID:         signerID,
		StakingSignature: []byte{},
	}
	vote := &hotmodel.Vote{
		BlockID:   blockID,
		View:      view,
		Signature: sig,
	}

	return vote
}

func newMockVoteAggregator(t *testing.T) (*VoteAggregator, flow.IdentityList) {
	ctrl := gomock.NewController(t)
	// mock identity list
	ids := unittest.IdentityListFixture(VALIDATOR_SIZE, unittest.WithRole(flow.RoleConsensus))
	// mock protocol state
	mockProtocolState := protocol.NewMockState(ctrl)
	mockSnapshot := protocol.NewMockSnapshot(ctrl)
	mockProtocolState.EXPECT().AtBlockID(gomock.Any()).Return(mockSnapshot).AnyTimes()
	mockProtocolState.EXPECT().Final().Return(mockSnapshot).AnyTimes()
	// signer := unittest.IdentityFixture()
	for _, id := range ids {
		mockSnapshot.EXPECT().Identity(id.NodeID).Return(id, nil).AnyTimes()
	}
	mockSnapshot.EXPECT().Identities(gomock.Any()).DoAndReturn(func(f ...flow.IdentityFilter) (flow.IdentityList, error) {
		return ids.Filter(f...), nil
	}).AnyTimes()
	// mock sig verifier and aggregator
	mockSigVerifier := &mocks.SigVerifier{}
	mockSigVerifier.On("VerifyRandomBeaconThresholdSig", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	mockSigVerifier.On("VerifyStakingAggregatedSig", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	mockSigVerifier.On("VerifyRandomBeaconSig", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	mockSigVerifier.On("VerifyStakingSig", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	mockSigAggregator := &mocks.SigAggregator{}
	mockSigAggregator.On("Aggregate", mock.Anything, mock.Anything).Return(&hotmodel.AggregatedSignature{}, nil)
	mockSigAggregator.On("CanReconstruct", mock.AnythingOfType("int")).Return(func(numOfSigShares int) bool {
		return float64(numOfSigShares) > float64(VALIDATOR_SIZE)/2.0
	})

	viewState, _ := viewstate.New(mockProtocolState, nil, ids[len(ids)-1].NodeID, filter.HasRole(flow.RoleConsensus))
	voteValidator := validator.New(viewState, nil, mockSigVerifier)
	return New(&mockdist.Consumer{}, 0, viewState, voteValidator, mockSigAggregator), ids
}

func getStateLength(va *VoteAggregator) (uint64, int, int, int, int) {
	return va.highestPrunedView, len(va.viewToBlockIDSet), len(va.viewToVoteID), len(va.pendingVotes.votes), len(va.blockIDToVotingStatus)
}
