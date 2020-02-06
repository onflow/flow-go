package hotstuff

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protocol/mocks"
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
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, types.ErrInsufficientVotes{}))
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should not be built when receiving insufficient votes
func TestReceiveBlockBeforeInsufficientVotes(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	for i := 0; i < 3; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		require.True(t, errors.Is(err, types.ErrInsufficientVotes{}))
	}
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should be built when receiving the 5th vote (the block is counted as one vote)
func TestReceiveBlockBeforeSufficientVotes(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	for i := 0; i < 4; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		if i < 3 {
			require.Nil(t, qc)
			require.NotNil(t, err)
			require.True(t, errors.Is(err, types.ErrInsufficientVotes{}))
		} else {
			require.Nil(t, err)
			require.NotNil(t, qc)
			require.Equal(t, block.BlockID(), qc.BlockID)
		}
	}
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// the same QC should be returned when receiving the 4th vote
func TestReceiveVoteAfterQCBuilt(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	for i := 0; i < 4; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		if i < 3 {
			require.Nil(t, qc)
			require.NotNil(t, err)
			require.True(t, errors.Is(err, types.ErrInsufficientVotes{}))
		} else {
			require.Nil(t, err)
			require.NotNil(t, qc)
			require.Equal(t, block.BlockID(), qc.BlockID)
		}
	}
	finalVote := newMockVote(testView, block.BlockID(), uint32(4))
	finalQC, err := va.StoreVoteAndBuildQC(finalVote, block)
	require.Nil(t, err)
	require.NotNil(t, finalQC)
	require.Equal(t, qc, finalQC)
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 3 votes first, a QC should not be built when receiving the block because of insufficient votes
func TestReceiveInsufficientVotesBeforeBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	for i := 0; i < 3; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		err = va.StorePendingVote(vote)
		require.Nil(t, err)
	}
	qc, err = va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, types.ErrInsufficientVotes{}))
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 6 votes only, votes should be stored correctly
func TestReceivePendingVotesOnly(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	blockID := unittest.IdentifierFixture()
	for i := 0; i < 6; i++ {
		vote := newMockVote(testView, blockID, uint32(i))
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
		require.Equal(t, vote, va.pendingVoteMap[blockID.String()].orderedVotes[i])
		require.Equal(t, vote, va.pendingVoteMap[blockID.String()].voteMap[string(vote.ID())])
	}
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 6 votes first, a QC should be built when receiving the block
func TestReceiveSufficientVotesBeforeBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	for i := 0; i < 6; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		err = va.StorePendingVote(vote)
		require.Nil(t, err)
	}
	qc, err = va.BuildQCOnReceivingBlock(block)
	require.Nil(t, err)
	require.NotNil(t, qc)
	require.Equal(t, block.BlockID(), qc.BlockID)
}

// UNHAPPY PATH (votes are invalid and the block arrives before votes)
// ErrStaleVote should be raised when receiving stale votes (vote.view <= lastPruneView)
func TestErrStaleVote(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(0)
	block := newMockBlock(testView)
	vote := newMockVote(testView, block.BlockID(), uint32(1))
	err := va.StorePendingVote(vote)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, types.ErrStaleVote{
		Vote:          vote,
		FinalizedView: va.lastPrunedView,
	}))
	require.Equal(t, 0, len(va.pendingVoteMap))
}

// UNHAPPY PATH (double voting)
// store one vote in the memory
// receive another vote with the same voter and the same view
// should trigger ErrDoubleVote
func TestErrDoubleVote(t *testing.T) {
	va := newMockVoteAggregator(t)
	// mock blocks and votes
	b1 := newMockBlock(10)
	b2 := newMockBlock(10)
	vote1 := newMockVote(10, b1.BlockID(), uint32(1))
	_, err := va.StoreVoteAndBuildQC(vote1, b1)
	// vote2 is double voting
	vote2 := newMockVote(10, b2.BlockID(), uint32(1))
	_, err = va.StoreVoteAndBuildQC(vote2, b2)
	if err != nil {
		switch err.(type) {
		case types.ErrDoubleVote:
			fmt.Printf("double vote detected %v", err)
		default:
			fmt.Printf("error detected %v", err)
		}
	} else {
		t.Errorf("double vote not detected")
	}
}

// UNHAPPY PATH (votes with invalid view)
// receive 7 votes in total
// 3 of them are invalid and located every other valid vote
// i.e., 1.valid, 2. invalid, ..., 6. valid, 7. invalid
// no QC should be generated in the whole process
func TestUnHappyPathForIncorporatedVotes(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	for i := 0; i < 7; i++ {
		var vote *types.Vote
		if i == 1 || i == 3 || i == 5 {
			vote = newMockVote(testView-1, block.BlockID(), uint32(i))
		} else {
			vote = newMockVote(testView, block.BlockID(), uint32(i))
		}
		qc, err := va.StoreVoteAndBuildQC(vote, block)
		require.NotNil(t, err)
		require.Nil(t, qc)
		fmt.Println(err.Error())
	}
}

// UNHAPPY PATH
// receive 7 votes in total
// 1. all 7 votes are pending and invalid (cannot pass ValidatePendingVotes), no votes should be stored
// 2. all 7 votes are pending and can pass ValidatePendingVotes, but cannot pass ValidateIncorporatedVotes
//    when receiving the block, no vote should be moved to incorporatedVotes, and no QC should be generated
func TestUnHappyPathForPendingVotes(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	// testing invalid pending votes
	for i := 0; i < 7; i++ {
		// signerIndex is invalid
		vote := newMockVote(uint64(0), block.BlockID(), uint32(10))
		err := va.StorePendingVote(vote)
		require.NotNil(t, err)
		fmt.Println(err.Error())
		qc, err := va.BuildQCOnReceivingBlock(block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		fmt.Println(err.Error())
	}
	// testing valid pending votes with invalid view
	for i := 0; i < 7; i++ {
		// view is invalid
		vote := newMockVote(testView-1, block.BlockID(), uint32(10))
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
		qc, err := va.BuildQCOnReceivingBlock(block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		fmt.Println(err.Error())
	}
}

// INVALID VOTES
// receive 4 invalid votes, and then the block, no QC should be built
func TestInvalidVotesOnly(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		// vote view is invalid
		vote := newMockVote(testView-1, block.BlockID(), uint32(i))
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
	}
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	// no votes should be added to blockHashToVotingStatus since the view is invalid
	require.Equal(t, 1, len(va.blockHashToVotingStatus))
	// the only vote added should be the primary vote
	primaryVote, exists := va.blockHashToVotingStatus[block.BlockID().String()].votes[string(block.ToVote().ID())]
	require.True(t, exists)
	require.Equal(t, block.ToVote(), primaryVote)
}

// INVALID VOTES
// receive 1 invalid vote, and 4 valid votes, and then the block, a QC should be built
func TestVoteMixtureBeforeBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	// testing invalid pending votes
	for i := 0; i < 5; i++ {
		var vote *types.Vote
		if i == 0 {
			// vote view is invalid
			vote = newMockVote(testView-1, block.BlockID(), uint32(i))
		} else {
			vote = newMockVote(testView, block.BlockID(), uint32(i))
		}
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
	}
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, err)
	require.NotNil(t, qc)
}

// INVALID VOTES
// receive the block, and 3 valid vote, and 1 invalid vote, no QC should be built
func TestVoteMixtureAfterBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		var vote *types.Vote
		if i < 3 {
			vote = newMockVote(testView, block.BlockID(), uint32(i))
		} else {
			// vote view is invalid
			vote = newMockVote(testView-1, block.BlockID(), uint32(i))
		}
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		require.Nil(t, qc)
		require.NotNil(t, err)
	}
}

// DUPLICATION
// receive the block, and the same valid votes for 5 times, no QC should be built
func TestDuplicateVotesAfterBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	vote := newMockVote(testView, block.BlockID(), uint32(1))
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		// only this vote and the primary vote are added
		require.Equal(t, 2, len(va.blockHashToVotingStatus[block.BlockID().String()].votes))
	}
}

// DUPLICATION
// receive same valid votes for 5 times, then the block, no QC should be built
func TestDuplicateVotesBeforeBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	vote := newMockVote(testView, block.BlockID(), uint32(1))
	for i := 0; i < 4; i++ {
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
	}
	blockIDStr := block.BlockID().String()
	require.Equal(t, 1, len(va.pendingVoteMap[blockIDStr].voteMap))
	require.Equal(t, 1, len(va.pendingVoteMap[blockIDStr].orderedVotes))
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, qc)
	require.NotNil(t, err)
	require.Equal(t, 2, len(va.blockHashToVotingStatus[block.BlockID().String()].votes))
}

// ORDER
// receive 5 votes, and the block, the QC should contain leader's vote, and the first 4 votes.
func TestVoteOrderAfterBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	var voteList []*types.Vote
	voteList = append(voteList, block.ToVote())
	for i := 0; i < 5; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
		voteList = append(voteList, vote)
	}
	qc, err := va.BuildQCOnReceivingBlock(block)
	require.Nil(t, err)
	require.NotNil(t, qc)

	for i := 0; i < 6; i++ {
		if i < 5 {
			require.True(t, qc.AggregatedSignature.Signers[voteList[i].Signature.SignerIdx])
		} else {
			require.False(t, qc.AggregatedSignature.Signers[voteList[i].Signature.SignerIdx])
		}
	}
}

// PRUNE
// receive votes for view 2, 3, 4, 5 without the block
// prune by 4, should have only vote for view 5 left
func TestPartialPruneBeforeBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	pruneView := uint64(4)
	var voteList []*types.Vote
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		vote := newMockVote(view, unittest.IdentifierFixture(), uint32(i))
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
		voteList = append(voteList, vote)
	}
	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
	require.Equal(t, 4, len(va.viewToIDToVote))
	require.Equal(t, 4, len(va.pendingVoteMap))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.lastPrunedView)
	require.Equal(t, 1, len(va.viewToBlockIDStrSet))
	require.Equal(t, 1, len(va.viewToIDToVote))
	require.Equal(t, 1, len(va.pendingVoteMap))
	testVote := voteList[len(voteList)-1]
	require.True(t, va.viewToBlockIDStrSet[uint64(5)][testVote.BlockID.String()])
	voter, err := va.voteValidator.ValidateVote(testVote, nil)
	require.Nil(t, err)
	require.NotNil(t, voter)
	require.Equal(t, testVote, va.viewToIDToVote[uint64(5)][voter.ID()])
	_, exists := va.pendingVoteMap[testVote.BlockID.String()]
	require.True(t, exists)
}

// PRUNE
// receive the block and the votes for view 2, 3, 4, 5
// prune by 4, should have only vote for view 5 left
func TestPartialPruneAfterBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	pruneView := uint64(4)
	var voteList []*types.Vote
	var blockList []*types.BlockProposal
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		block := newMockBlock(view)
		blockList = append(blockList, block)
		qc, err := va.BuildQCOnReceivingBlock(block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		vote := newMockVote(view, block.BlockID(), uint32(i))
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		voteList = append(voteList, vote)
	}
	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
	require.Equal(t, 4, len(va.viewToIDToVote))
	require.Equal(t, 4, len(va.blockHashToVotingStatus))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.lastPrunedView)
	require.Equal(t, 1, len(va.viewToBlockIDStrSet))
	require.Equal(t, 1, len(va.viewToIDToVote))
	require.Equal(t, 1, len(va.blockHashToVotingStatus))
	testVote := voteList[len(voteList)-1]
	testBlock := blockList[len(blockList)-1]
	require.True(t, va.viewToBlockIDStrSet[uint64(5)][testVote.BlockID.String()])
	voter, err := va.voteValidator.ValidateVote(testVote, testBlock)
	require.Nil(t, err)
	require.NotNil(t, voter)
	require.Equal(t, testVote, va.viewToIDToVote[uint64(5)][voter.ID()])
	_, exists := va.blockHashToVotingStatus[testVote.BlockID.String()]
	require.True(t, exists)
}

// PRUNE
// receive votes for view 2, 3, 4, 5 without the block
// prune by 6, should have no vote left
func TestFullPruneBeforeBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	pruneView := uint64(6)
	var voteList []*types.Vote
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		vote := newMockVote(view, unittest.IdentifierFixture(), uint32(i))
		err := va.StorePendingVote(vote)
		require.Nil(t, err)
		voteList = append(voteList, vote)
	}
	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
	require.Equal(t, 4, len(va.viewToIDToVote))
	require.Equal(t, 4, len(va.pendingVoteMap))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.lastPrunedView)
	require.Equal(t, 0, len(va.viewToBlockIDStrSet))
	require.Equal(t, 0, len(va.viewToIDToVote))
	require.Equal(t, 0, len(va.pendingVoteMap))
}

// PRUNE
// receive the block and the votes for view 2, 3, 4, 5
// prune by 6, should have no vote left
func TestFullPruneAfterBlock(t *testing.T) {
	va := newMockVoteAggregator(t)
	pruneView := uint64(6)
	var voteList []*types.Vote
	var blockList []*types.BlockProposal
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		block := newMockBlock(view)
		blockList = append(blockList, block)
		qc, err := va.BuildQCOnReceivingBlock(block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		vote := newMockVote(view, block.BlockID(), uint32(i))
		qc, err = va.StoreVoteAndBuildQC(vote, block)
		require.Nil(t, qc)
		require.NotNil(t, err)
		voteList = append(voteList, vote)
	}
	require.Equal(t, 4, len(va.viewToBlockIDStrSet))
	require.Equal(t, 4, len(va.viewToIDToVote))
	require.Equal(t, 4, len(va.blockHashToVotingStatus))
	va.PruneByView(pruneView)
	require.Equal(t, pruneView, va.lastPrunedView)
	require.Equal(t, 0, len(va.viewToBlockIDStrSet))
	require.Equal(t, 0, len(va.viewToIDToVote))
	require.Equal(t, 0, len(va.blockHashToVotingStatus))
}

// store random votes and QCs from view 1 to 3
// prune by view 3
func TestPruneByView(t *testing.T) {
	va := newMockVoteAggregator(t)
	pruneByView := 3
	for i := 1; i <= pruneByView; i++ {
		view := uint64(i)
		block := unittest.BlockHeaderFixture()
		blockID := block.ID()
		blockIDStr := blockID.String()
		voter := unittest.IdentityFixture()
		vote := newMockVote(view, blockID, 2)
		pendingStatus := NewPendingStatus()
		pendingStatus.AddVote(vote)
		va.pendingVoteMap[blockIDStr] = pendingStatus
		vs := NewVotingStatus(uint64(10), vote.View, uint32(7), voter, vote.BlockID)
		vs.AddVote(vote)
		va.blockHashToVotingStatus[blockIDStr] = vs
		qc := &types.QuorumCertificate{
			View:    view,
			BlockID: blockID,
		}
		va.createdQC[blockIDStr] = qc
		va.viewToBlockIDStrSet[view] = map[string]bool{}
		va.viewToBlockIDStrSet[view][blockIDStr] = true
		va.viewToIDToVote[view] = map[flow.Identifier]*types.Vote{}
		va.viewToIDToVote[view][voter.ID()] = vote
	}

	// verify that all insertion is successful
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.viewToIDToVote))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.viewToBlockIDStrSet))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.createdQC))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.pendingVoteMap))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.blockHashToVotingStatus))

	va.PruneByView(uint64(pruneByView))

	// verify that all deletion is successful
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.viewToIDToVote))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.viewToBlockIDStrSet))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.blockHashToVotingStatus))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.createdQC))
	require.Equal(t, pruneByView-int(va.lastPrunedView), len(va.pendingVoteMap))
}

func newMockBlock(view uint64) *types.BlockProposal {
	blockHeader := unittest.BlockHeaderFixture()
	blockHeader.Number = view
	block := &types.Block{
		View:    view,
		BlockID: blockHeader.ID(),
	}
	sig := &types.Signature{
		RawSignature: [32]byte{},
		SignerIdx:    VALIDATOR_SIZE - 1,
	}
	return types.NewBlockProposal(block, sig)
}

func newMockVote(view uint64, blockID flow.Identifier, signerIndex uint32) *types.Vote {
	vote := &types.Vote{
		UnsignedVote: types.UnsignedVote{
			View:    view,
			BlockID: blockID,
		},
		Signature: &types.Signature{
			RawSignature: [32]byte{},
			SignerIdx:    signerIndex,
		},
	}

	return vote
}

func newMockVoteAggregator(t *testing.T) *VoteAggregator {
	ctrl := gomock.NewController(t)
	ids := unittest.IdentityListFixture(VALIDATOR_SIZE, unittest.WithRole(flow.RoleConsensus))
	snapshot := protocol.NewMockSnapshot(ctrl)
	mockProtocolState := mocks.NewMockState(ctrl)
	mockProtocolState.EXPECT().AtBlockID(gomock.Any()).Return(snapshot).AnyTimes()
	snapshot.EXPECT().Identities(gomock.Any()).Return(ids, nil).AnyTimes()
	viewState := &ViewState{protocolState: mockProtocolState}
	voteValidator := &Validator{viewState: viewState}

	return NewVoteAggregator(zerolog.Logger{}, 0, viewState, voteValidator)
}
