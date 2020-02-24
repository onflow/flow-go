package hotstuff

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	mockverifier "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/engine/consensus/hotstuff/mocks"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/hotstuff"
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
// highestPrunedView is 10, receive a block with view 5
func TestStaleProposerVote(t *testing.T) {
	va, ids := newMockVoteAggregator(t)
	va.highestPrunedView = 10
	testView := uint64(5)
	bp := newMockBlock(testView, ids[len(ids)-1].NodeID)
	ok := va.StoreProposerVote(bp.ProposerVote())
	require.False(t, ok)
	qc, built, err := va.BuildQCOnReceivedBlock(bp.Block)
	require.Error(t, err)
	require.False(t, built)
	require.Nil(t, qc)
}

// UNHAPPY PATH (double voting)
// store one vote in the memory
// receive another vote with the same voter and the same view
// should trigger ErrDoubleVote
//func TestErrDoubleVote(t *testing.T) {
//	va, ids := newMockVoteAggregator(t)
//	// mock blocks and votes
//	bp1 := newMockBlock(10, ids[0].NodeID)
//	bp2 := newMockBlock(10, ids[1].NodeID)
//	vote1 := newMockVote(10, bp1.Block.BlockID, ids[0].NodeID)
//	_, err := va.StoreVoteAndBuildQC(vote1, b1)
//	// vote2 is double voting
//	vote2 := newMockVote(10, b2.BlockID(), uint32(1))
//	_, err = va.StoreVoteAndBuildQC(vote2, b2)
//	if err != nil {
//		switch err.(type) {
//		case types.ErrDoubleVote:
//			fmt.Printf("double vote detected %v", err)
//		default:
//			fmt.Printf("error detected %v", err)
//		}
//	} else {
//		t.Errorf("double vote not detected")
//	}
//}
//
//// INVALID VOTES
//// receive 4 invalid votes, and then the block, no QC should be built
//func TestInvalidVotesOnly(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	testView := uint64(5)
//	block := newMockBlock(testView)
//	// testing invalid pending votes
//	for i := 0; i < 4; i++ {
//		// vote view is invalid
//		vote := newMockVote(testView-1, block.BlockID(), uint32(i))
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//	}
//	qc, err := va.BuildQCOnReceivingBlock(block)
//	require.Nil(t, qc)
//	require.NotNil(t, err)
//	// no votes should be added to blockHashToVotingStatus since the view is invalid
//	require.Equal(t, 1, len(va.blockHashToVotingStatus))
//	// the only vote added should be the primary vote
//	primaryVote, exists := va.blockHashToVotingStatus[block.BlockID()].votes[block.ToVote().ID()]
//	require.True(t, exists)
//	require.Equal(t, block.ToVote(), primaryVote)
//}
//
//// INVALID VOTES
//// receive 1 invalid vote, and 4 valid votes, and then the block, a QC should be built
//func TestVoteMixtureBeforeBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	testView := uint64(5)
//	block := newMockBlock(testView)
//	// testing invalid pending votes
//	for i := 0; i < 5; i++ {
//		var vote *types.Vote
//		if i == 0 {
//			// vote view is invalid
//			vote = newMockVote(testView-1, block.BlockID(), uint32(i))
//		} else {
//			vote = newMockVote(testView, block.BlockID(), uint32(i))
//		}
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//	}
//	qc, err := va.BuildQCOnReceivingBlock(block)
//	require.Nil(t, err)
//	require.NotNil(t, qc)
//}
//
//// INVALID VOTES
//// receive the block, and 3 valid vote, and 1 invalid vote, no QC should be built
//func TestVoteMixtureAfterBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	testView := uint64(5)
//	block := newMockBlock(testView)
//	qc, err := va.BuildQCOnReceivingBlock(block)
//	require.Nil(t, qc)
//	require.NotNil(t, err)
//	// testing invalid pending votes
//	for i := 0; i < 4; i++ {
//		var vote *types.Vote
//		if i < 3 {
//			vote = newMockVote(testView, block.BlockID(), uint32(i))
//		} else {
//			// vote view is invalid
//			vote = newMockVote(testView-1, block.BlockID(), uint32(i))
//		}
//		qc, err = va.StoreVoteAndBuildQC(vote, block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//	}
//}
//
//// DUPLICATION
//// receive the block, and the same valid votes for 5 times, no QC should be built
//func TestDuplicateVotesAfterBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	testView := uint64(5)
//	block := newMockBlock(testView)
//	qc, err := va.BuildQCOnReceivingBlock(block)
//	require.Nil(t, qc)
//	require.NotNil(t, err)
//	vote := newMockVote(testView, block.BlockID(), uint32(1))
//	// testing invalid pending votes
//	for i := 0; i < 4; i++ {
//		qc, err = va.StoreVoteAndBuildQC(vote, block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//		// only this vote and the primary vote are added
//		require.Equal(t, 2, len(va.blockHashToVotingStatus[block.BlockID()].votes))
//	}
//}
//
//// DUPLICATION
//// receive same valid votes for 5 times, then the block, no QC should be built
//func TestDuplicateVotesBeforeBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	testView := uint64(5)
//	block := newMockBlock(testView)
//	vote := newMockVote(testView, block.BlockID(), uint32(1))
//	for i := 0; i < 4; i++ {
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//	}
//	require.Equal(t, 1, len(va.pendingVoteMap[block.BlockID()].voteMap))
//	require.Equal(t, 1, len(va.pendingVoteMap[block.BlockID()].orderedVotes))
//	qc, err := va.BuildQCOnReceivingBlock(block)
//	require.Nil(t, qc)
//	require.NotNil(t, err)
//	require.Equal(t, 2, len(va.blockHashToVotingStatus[block.BlockID()].votes))
//}
//
//// ORDER
//// receive 5 votes, and the block, the QC should contain leader's vote, and the first 4 votes.
//func TestVoteOrderAfterBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	testView := uint64(5)
//	block := newMockBlock(testView)
//	var voteList []*types.Vote
//	voteList = append(voteList, block.ToVote())
//	for i := 0; i < 5; i++ {
//		vote := newMockVote(testView, block.BlockID(), uint32(i))
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//		voteList = append(voteList, vote)
//	}
//	qc, err := va.BuildQCOnReceivingBlock(block)
//	require.Nil(t, err)
//	require.NotNil(t, qc)
//
//	for i := 0; i < 6; i++ {
//		if i < 5 {
//			require.True(t, qc.AggregatedSignature.Signers[voteList[i].Signature.SignerIdx])
//		} else {
//			require.False(t, qc.AggregatedSignature.Signers[voteList[i].Signature.SignerIdx])
//		}
//	}
//}
//
//// PRUNE
//// receive votes for view 2, 3, 4, 5 without the block
//// prune by 4, should have only vote for view 5 left
//func TestPartialPruneBeforeBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	pruneView := uint64(4)
//	var voteList []*types.Vote
//	for i := 2; i <= 5; i++ {
//		view := uint64(i)
//		vote := newMockVote(view, unittest.IdentifierFixture(), uint32(i))
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//		voteList = append(voteList, vote)
//	}
//	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 4, len(va.viewToIDToVote))
//	require.Equal(t, 4, len(va.pendingVoteMap))
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 1, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 1, len(va.viewToIDToVote))
//	require.Equal(t, 1, len(va.pendingVoteMap))
//	testVote := voteList[len(voteList)-1]
//	require.NotNil(t, va.viewToBlockIDStrSet[uint64(5)][testVote.BlockID])
//	voter, err := va.voteValidator.ValidateVote(testVote, nil)
//	require.Nil(t, err)
//	require.NotNil(t, voter)
//	require.Equal(t, testVote, va.viewToIDToVote[uint64(5)][voter.ID()])
//	_, exists := va.pendingVoteMap[testVote.BlockID]
//	require.True(t, exists)
//}
//
//// PRUNE
//// receive the block and the votes for view 2, 3, 4, 5
//// prune by 4, should have only vote for view 5 left
//func TestPartialPruneAfterBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	pruneView := uint64(4)
//	var voteList []*types.Vote
//	var blockList []*types.BlockProposal
//	for i := 2; i <= 5; i++ {
//		view := uint64(i)
//		block := newMockBlock(view)
//		blockList = append(blockList, block)
//		qc, err := va.BuildQCOnReceivingBlock(block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//		vote := newMockVote(view, block.BlockID(), uint32(i))
//		qc, err = va.StoreVoteAndBuildQC(vote, block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//		voteList = append(voteList, vote)
//	}
//	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 4, len(va.viewToIDToVote))
//	require.Equal(t, 4, len(va.blockHashToVotingStatus))
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 1, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 1, len(va.viewToIDToVote))
//	require.Equal(t, 1, len(va.blockHashToVotingStatus))
//	testVote := voteList[len(voteList)-1]
//	testBlock := blockList[len(blockList)-1]
//	require.NotNil(t, va.viewToBlockIDStrSet[uint64(5)][testVote.BlockID])
//	voter, err := va.voteValidator.ValidateVote(testVote, testBlock)
//	require.Nil(t, err)
//	require.NotNil(t, voter)
//	require.Equal(t, testVote, va.viewToIDToVote[uint64(5)][voter.ID()])
//	_, exists := va.blockHashToVotingStatus[testVote.BlockID]
//	require.True(t, exists)
//}
//
//// PRUNE
//// receive votes for view 2, 3, 4, 5 without the block
//// prune by 6, should have no vote left
//func TestFullPruneBeforeBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	pruneView := uint64(6)
//	for i := 2; i <= 5; i++ {
//		view := uint64(i)
//		vote := newMockVote(view, unittest.IdentifierFixture(), uint32(i))
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//	}
//	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 4, len(va.viewToIDToVote))
//	require.Equal(t, 4, len(va.pendingVoteMap))
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 0, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 0, len(va.viewToIDToVote))
//	require.Equal(t, 0, len(va.pendingVoteMap))
//}
//
//// PRUNE
//// receive the block and the votes for view 2, 3, 4, 5
//// prune by 6, should have no vote left
//func TestFullPruneAfterBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	pruneView := uint64(6)
//	var voteList []*types.Vote
//	for i := 2; i <= 5; i++ {
//		view := uint64(i)
//		block := newMockBlock(view)
//		qc, err := va.BuildQCOnReceivingBlock(block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//		vote := newMockVote(view, block.BlockID(), uint32(i))
//		qc, err = va.StoreVoteAndBuildQC(vote, block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//		voteList = append(voteList, vote)
//	}
//	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 4, len(va.viewToIDToVote))
//	require.Equal(t, 4, len(va.blockHashToVotingStatus))
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 0, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 0, len(va.viewToIDToVote))
//	require.Equal(t, 0, len(va.blockHashToVotingStatus))
//}
//
//// PRUNE
//// receive votes for view 3, 4, 5 without block
//// prune by 2 twice, should have all votes there
//func TestNonePruneBeforeBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	pruneView := uint64(2)
//	for i := 3; i <= 5; i++ {
//		view := uint64(i)
//		vote := newMockVote(view, unittest.IdentifierFixture(), uint32(i))
//		err := va.StorePendingVote(vote)
//		require.Nil(t, err)
//	}
//	require.Equal(t, 3, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 3, len(va.viewToIDToVote))
//	require.Equal(t, 3, len(va.pendingVoteMap))
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 3, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 3, len(va.viewToIDToVote))
//	require.Equal(t, 3, len(va.pendingVoteMap))
//	// prune twice
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 3, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 3, len(va.viewToIDToVote))
//	require.Equal(t, 3, len(va.pendingVoteMap))
//}
//
//// PRUNE
//// receive the block and the votes for view 3, 4, 5
//// prune by 2 twice, should have all votes there
//func TestNonePruneAfterBlock(t *testing.T) {
//	va := newMockVoteAggregator(t)
//	pruneView := uint64(2)
//	for i := 3; i <= 5; i++ {
//		view := uint64(i)
//		block := newMockBlock(view)
//		qc, err := va.BuildQCOnReceivingBlock(block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//		vote := newMockVote(view, block.BlockID(), uint32(i))
//		qc, err = va.StoreVoteAndBuildQC(vote, block)
//		require.Nil(t, qc)
//		require.NotNil(t, err)
//	}
//	require.Equal(t, 3, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 3, len(va.viewToIDToVote))
//	require.Equal(t, 3, len(va.blockHashToVotingStatus))
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 3, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 3, len(va.viewToIDToVote))
//	require.Equal(t, 3, len(va.blockHashToVotingStatus))
//	// prune twice
//	va.PruneByView(pruneView)
//	require.Equal(t, pruneView, va.lastPrunedView)
//	require.Equal(t, 3, len(va.viewToBlockIDStrSet))
//	require.Equal(t, 3, len(va.viewToIDToVote))
//	require.Equal(t, 3, len(va.blockHashToVotingStatus))
//}

func newMockBlock(view uint64, proposerID flow.Identifier) *hotstuff.Proposal {
	blockHeader := unittest.BlockHeaderFixture()
	blockHeader.View = view
	block := &hotstuff.Block{
		View:       view,
		BlockID:    blockHeader.ID(),
		ProposerID: proposerID,
	}
	sig := crypto.Signature{}
	return &hotstuff.Proposal{
		Block:            block,
		StakingSignature: sig,
	}
}

func newMockVote(view uint64, blockID flow.Identifier, signerID flow.Identifier) *hotstuff.Vote {
	sig := &hotstuff.SingleSignature{
		SignerID:         signerID,
		StakingSignature: []byte{},
	}
	vote := &hotstuff.Vote{
		BlockID:   blockID,
		View:      view,
		Signature: sig,
	}

	return vote
}

func newMockVoteAggregator(t *testing.T) (*VoteAggregator, flow.IdentityList) {
	ctrl := gomock.NewController(t)
	ids := unittest.IdentityListFixture(VALIDATOR_SIZE, unittest.WithRole(flow.RoleConsensus))
	mockSnapshot := protocol.NewMockSnapshot(ctrl)
	mockProtocolState := protocol.NewMockState(ctrl)
	mockProtocolState.EXPECT().AtBlockID(gomock.Any()).Return(mockSnapshot).AnyTimes()
	mockProtocolState.EXPECT().Final().Return(mockSnapshot).AnyTimes()
	signer := unittest.IdentityFixture()
	mockSnapshot.EXPECT().Identity(gomock.AssignableToTypeOf(signer.NodeID)).Return(signer, nil).AnyTimes()
	mockSnapshot.EXPECT().Identities(gomock.Any()).DoAndReturn(func(f ...flow.IdentityFilter) (flow.IdentityList, error) {
		return ids.Filter(f...), nil
	}).AnyTimes()
	viewState, err := NewViewState(mockProtocolState, ids[len(ids)-1].NodeID, filter.HasRole(flow.RoleConsensus))
	require.NoError(t, err)
	mockSigVerifier := mockverifier.NewMockSigVerifier(ctrl)
	mockSigVerifier.EXPECT().VerifyAggregatedRandomBeaconSignature(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigVerifier.EXPECT().VerifyAggregatedStakingSignature(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigVerifier.EXPECT().VerifyRandomBeaconSig(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigVerifier.EXPECT().VerifyStakingSig(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSigAggregator := mockverifier.NewMockSigAggregator(ctrl)
	mockSigAggregator.EXPECT().Aggregate(gomock.Any(), gomock.Any()).Return(&hotstuff.AggregatedSignature{}, nil).AnyTimes()
	mockSigAggregator.EXPECT().CanReconstruct(gomock.Any()).Return(true).AnyTimes()
	voteValidator := &Validator{viewState: viewState, sigVerifier: mockSigVerifier}
	return NewVoteAggregator(&mockdist.Consumer{}, 0, viewState, voteValidator, mockSigAggregator), ids
}
