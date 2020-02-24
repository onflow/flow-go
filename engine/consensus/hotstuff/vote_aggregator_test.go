package hotstuff

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	mockverifier "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/engine/consensus/hotstuff/mocks"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	hotmodel "github.com/dapperlabs/flow-go/model/hotstuff"
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
// a QC should be built when receiving the 5th vote (the block is counted as one vote)
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
// the same QC should be returned when receiving the 4th vote
func TestReceiveVoteAfterQCBuilt(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	_ = va.StoreProposerVote(bp.ProposerVote())
	_, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 4; i++ {
		vote := newMockVote(testView, bp.Block.BlockID, ids[i].NodeID)
		_, built, err = va.StoreVoteAndBuildQC(vote, bp.Block)
	}
	finalVote := newMockVote(testView, bp.Block.BlockID, ids[4].NodeID)
	finalQC, built, err := va.StoreVoteAndBuildQC(finalVote, bp.Block)
	require.Nil(t, err)
	require.NotNil(t, finalQC)
	require.True(t, built)
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
	va.highestPrunedView = 10
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
// highestPrunedView is 10, receive a block with view 5
func TestStaleProposerVote(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	va.highestPrunedView = 10
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
	va, ids := newMockVoteAggregator(t)
	testview := uint64(5)
	// mock double proposal
	bp1 := newMockBlock(testview, ids[0].NodeID)
	bp2 := newMockBlock(testview, ids[1].NodeID)
	va.StoreProposerVote(bp1.ProposerVote())
	va.StoreProposerVote(bp2.ProposerVote())
	_, _, _ = va.BuildQCOnReceivedBlock(bp1.Block)
	_, _, _ = va.BuildQCOnReceivedBlock(bp2.Block)
	// mock double voting
	vote1 := newMockVote(testview, bp1.Block.BlockID, ids[2].NodeID)
	qc, built, err := va.StoreVoteAndBuildQC(vote1, bp1.Block)
	require.Nil(t, qc)
	require.False(t, built)
	require.NoError(t, err)
	vote2 := newMockVote(testview, bp2.Block.BlockID, ids[2].NodeID)
	notifier := &mockdist.Consumer{}
	va.notifier = notifier
	notifier.On("OnDoubleVotingDetected", vote1, vote2).Return().Once()
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
	require.NotNil(t, qc)
	require.True(t, built)
	require.NoError(t, err)
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
	require.Equal(t, 4, len(va.viewToBlockIDSet))
	require.Equal(t, 4, len(va.viewToVoteID))
	require.Equal(t, 4, len(va.pendingVotes.votes))
	// after pruning
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 1, len(va.viewToBlockIDSet))
	require.Equal(t, 1, len(va.viewToVoteID))
	require.Equal(t, 1, len(va.pendingVotes.votes))
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
	require.Equal(t, 4, len(va.viewToBlockIDSet))
	require.Equal(t, 4, len(va.viewToVoteID))
	require.Equal(t, 4, len(va.blockIDToVotingStatus))
	// after pruning
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 1, len(va.viewToBlockIDSet))
	require.Equal(t, 1, len(va.viewToVoteID))
	require.Equal(t, 1, len(va.blockIDToVotingStatus))
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
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 0, len(va.viewToVoteID))
	require.Equal(t, 0, len(va.viewToBlockIDSet))
	require.Equal(t, 0, len(va.pendingVotes.votes))
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
	require.Equal(t, 4, len(va.viewToBlockIDSet))
	require.Equal(t, 4, len(va.viewToVoteID))
	require.Equal(t, 4, len(va.blockIDToVotingStatus))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 0, len(va.viewToVoteID))
	require.Equal(t, 0, len(va.viewToBlockIDSet))
	require.Equal(t, 0, len(va.blockIDToVotingStatus))
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
	require.Equal(t, 3, len(va.viewToBlockIDSet))
	require.Equal(t, 3, len(va.viewToVoteID))
	require.Equal(t, 3, len(va.pendingVotes.votes))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 3, len(va.viewToVoteID))
	require.Equal(t, 3, len(va.viewToBlockIDSet))
	require.Equal(t, 3, len(va.pendingVotes.votes))
	// prune twice
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 3, len(va.viewToBlockIDSet))
	require.Equal(t, 3, len(va.viewToVoteID))
	require.Equal(t, 3, len(va.pendingVotes.votes))
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
	require.Equal(t, 3, len(va.viewToVoteID))
	require.Equal(t, 3, len(va.viewToBlockIDSet))
	require.Equal(t, 3, len(va.blockIDToVotingStatus))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 3, len(va.viewToBlockIDSet))
	require.Equal(t, 3, len(va.viewToVoteID))
	require.Equal(t, 3, len(va.blockIDToVotingStatus))
	// prune twice
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.highestPrunedView)
	require.Equal(t, 3, len(va.viewToVoteID))
	require.Equal(t, 3, len(va.viewToBlockIDSet))
	require.Equal(t, 3, len(va.blockIDToVotingStatus))
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
	signer := unittest.IdentityFixture()
	mockSnapshot.EXPECT().Identity(gomock.AssignableToTypeOf(signer.NodeID)).Return(signer, nil).AnyTimes()
	mockSnapshot.EXPECT().Identities(gomock.Any()).DoAndReturn(func(f ...flow.IdentityFilter) (flow.IdentityList, error) {
		return ids.Filter(f...), nil
	}).AnyTimes()
	// mock sig verifier and aggregator
	mockSigVerifier := mockverifier.NewMockSigVerifier(ctrl)
	mockSigVerifier.EXPECT().VerifyAggregatedRandomBeaconSignature(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigVerifier.EXPECT().VerifyAggregatedStakingSignature(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigVerifier.EXPECT().VerifyRandomBeaconSig(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigVerifier.EXPECT().VerifyStakingSig(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigAggregator := mockverifier.NewMockSigAggregator(ctrl)
	mockSigAggregator.EXPECT().Aggregate(gomock.Any(), gomock.Any()).Return(&hotmodel.AggregatedSignature{}, nil).AnyTimes()
	mockSigAggregator.EXPECT().CanReconstruct(gomock.Any()).Return(true).AnyTimes()

	viewState, _ := NewViewState(mockProtocolState, ids[len(ids)-1].NodeID, filter.HasRole(flow.RoleConsensus))
	voteValidator := &Validator{viewState: viewState, sigVerifier: mockSigVerifier}
	return NewVoteAggregator(&mockdist.Consumer{}, 0, viewState, voteValidator, mockSigAggregator), ids
}
