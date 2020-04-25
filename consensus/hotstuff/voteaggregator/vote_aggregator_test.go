// +build relic

package voteaggregator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	protomock "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestAggregator(t *testing.T) {
	suite.Run(t, new(AggregatorSuite))
}

type AggregatorSuite struct {
	suite.Suite
	participants flow.IdentityList
	qcs          map[flow.Identifier]*model.QuorumCertificate
	protocol     *protomock.State
	snapshot     *protomock.Snapshot
	signer       *mocks.Signer
	forks        *mocks.Forks
	committee    hotstuff.Committee
	validator    hotstuff.Validator
	aggregator   *VoteAggregator
}

func (as *AggregatorSuite) SetupTest() {

	// generate the validator set with qualified majority threshold of 5
	as.participants = unittest.IdentityListFixture(7, unittest.WithRole(flow.RoleConsensus))
	as.qcs = make(map[flow.Identifier]*model.QuorumCertificate)

	// create a mocked snapshot
	as.snapshot = &protomock.Snapshot{}
	as.snapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return as.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range as.participants {
		as.snapshot.On("Identity", participant.NodeID).Return(participant, nil)
	}

	// create a mocked protocol state
	as.protocol = &protomock.State{}
	as.protocol.On("Final").Return(as.snapshot)

	// create a mocked forks
	as.forks = &mocks.Forks{}

	// create MembersState
	var err error
	as.committee, err = committee.NewMainConsensusCommitteeState(as.protocol, as.participants[0].NodeID)
	require.NoError(as.T(), err)

	// created a mocked signer that can sign proposals
	as.signer = &mocks.Signer{}
	as.signer.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	as.signer.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	as.signer.On("CreateQC", mock.AnythingOfType("[]*model.Vote")).Return(
		func(votes []*model.Vote) *model.QuorumCertificate {
			qc, _ := as.qcs[votes[0].BlockID]
			return qc
		},
		func(votes []*model.Vote) error {
			_, ok := as.qcs[votes[0].BlockID]
			if !ok {
				return fmt.Errorf("no votes for block registered")
			}
			return nil
		},
	)

	// create a real validator
	as.validator = validator.New(as.committee, as.forks, as.signer)

	// create the aggregator
	as.aggregator = New(&mocks.Consumer{}, 0, as.committee, as.validator, as.signer)
}

func (as *AggregatorSuite) RegisterProposal(proposal *model.Proposal) {
	as.protocol.On("AtBlockID", proposal.Block.BlockID).Return(as.snapshot)
	as.forks.On("GetBlock", proposal.Block.BlockID).Return(proposal.Block, true)
	as.signer.On("VerifyProposal", proposal.Block.BlockID).Return(true, nil)
}

func (as *AggregatorSuite) RegisterVote(vote *model.Vote) {
	qc, ok := as.qcs[vote.BlockID]
	if !ok {
		qc = &model.QuorumCertificate{
			View:    vote.View,
			BlockID: vote.BlockID,
			SigData: []byte{},
		}
		as.qcs[vote.BlockID] = qc
	}
	qc.SignerIDs = append(qc.SignerIDs, vote.SignerID)
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should not be built when receiving the block because of insufficient votes
func (as *AggregatorSuite) TestOnlyReceiveBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	_ = as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.False(as.T(), built)
	require.Nil(as.T(), qc)
	require.NoError(as.T(), err)
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should not be built when receiving insufficient votes
func (as *AggregatorSuite) TestReceiveBlockBeforeInsufficientVotes() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	_ = as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 3; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		qc, built, err = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
		require.False(as.T(), built)
		require.Nil(as.T(), qc)
		require.NoError(as.T(), err)
	}
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// a QC should be built when receiving the block and the 4th vote (the block is counted as one vote)
func (as *AggregatorSuite) TestReceiveBlockBeforeSufficientVotes() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	_ = as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 4; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		qc, built, err = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
		if i == 3 {
			require.NoError(as.T(), err)
			require.True(as.T(), built)
			require.NotNil(as.T(), qc)
		}
	}
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// assume there are 7 nodes, meaning that the threshold is 5
// the same QC should be returned when receiving the block and the 5th vote
func (as *AggregatorSuite) TestReceiveVoteAfterQCBuilt() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	_ = as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	var qc *model.QuorumCertificate
	for i := 0; i < 4; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		qc, _, _ = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
	}
	finalVote := newMockVote(as, testView, bp.Block.BlockID, as.participants[4].NodeID)
	finalQC, built, err := as.aggregator.StoreVoteAndBuildQC(finalVote, bp.Block)
	require.NoError(as.T(), err)
	require.NotNil(as.T(), finalQC)
	require.True(as.T(), built)
	require.Equal(as.T(), qc, finalQC)
}

// HAPPY PATH (votes are valid and the block always arrives before votes)
// a QC should not be built when received the block and 3 votes (still insufficient) and a vote for a different block
func (as *AggregatorSuite) TestReceiveVoteForDifferentBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	_ = as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 3; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		_, _, _ = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
	}
	bp2 := newMockBlock(as, testView, as.participants[len(as.participants)-2].NodeID)
	_ = as.aggregator.StoreProposerVote(bp2.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp2.Block)
	voteForDifferentBlock := newMockVote(as, testView, bp2.Block.BlockID, as.participants[4].NodeID)
	qc, built, err := as.aggregator.StoreVoteAndBuildQC(voteForDifferentBlock, bp2.Block)
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 3 votes first, a QC should not be built when receiving the block because of insufficient votes
func (as *AggregatorSuite) TestReceiveInsufficientVotesBeforeBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	for i := 0; i < 3; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		ok := as.aggregator.StorePendingVote(vote)
		require.True(as.T(), ok)
	}
	ok := as.aggregator.StoreProposerVote(bp.ProposerVote())
	require.True(as.T(), ok)
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.False(as.T(), built)
	require.Nil(as.T(), qc)
	require.NoError(as.T(), err)
}

// PENDING PATH (votes are valid and the block arrives after votes)
// receive 6 votes first, a QC should be built when receiving the block
func (as *AggregatorSuite) TestReceiveSufficientVotesBeforeBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	_, _, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	for i := 0; i < 6; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		ok := as.aggregator.StorePendingVote(vote)
		require.True(as.T(), ok)
	}
	_ = as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.NoError(as.T(), err)
	require.NotNil(as.T(), qc)
	require.True(as.T(), built)
}

// UNHAPPY PATH
// highestPrunedView is 10, receive a vote with view 5 without the block
// the vote should not be stored
func (as *AggregatorSuite) TestStaleVoteWithoutBlock() {
	as.aggregator.PruneByView(10)
	testView := uint64(5)
	vote := newMockVote(as, testView, unittest.IdentifierFixture(), as.participants[0].NodeID)
	ok := as.aggregator.StorePendingVote(vote)
	require.False(as.T(), ok)
}

// UNHAPPY PATH
// build qc without storing proposer vote
func (as *AggregatorSuite) TestBuildQCWithoutProposerVote() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.Errorf(as.T(), err, fmt.Sprintf("could not get proposer vote for block: %x", bp.Block.BlockID))
	require.False(as.T(), built)
	require.Nil(as.T(), qc)
}

// UNHAPPY PATH
// highestPrunedView is 10, receive a block with view 5 which should be ignored
func (as *AggregatorSuite) TestStaleProposerVote() {
	as.aggregator.PruneByView(10)
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	ok := as.aggregator.StoreProposerVote(bp.ProposerVote())
	require.False(as.T(), ok)
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.Errorf(as.T(), err, fmt.Sprintf("could not get proposer vote for block: %x", bp.Block.BlockID))
	require.False(as.T(), built)
	require.Nil(as.T(), qc)
}

// UNHAPPY PATH
// store one vote in the memory
// receive another vote with the same voter and the same view
// should trigger notifier
func (as *AggregatorSuite) TestDoubleVote() {
	testview := uint64(5)
	notifier := &mocks.Consumer{}
	// mock two proposals at the same view
	bp1 := newMockBlock(as, testview, as.participants[0].NodeID)
	bp2 := newMockBlock(as, testview, as.participants[1].NodeID)
	// mock double voting
	vote1 := newMockVote(as, testview, bp1.Block.BlockID, as.participants[2].NodeID)
	vote2 := newMockVote(as, testview, bp2.Block.BlockID, as.participants[2].NodeID)
	// set notifier
	notifier.On("OnDoubleVotingDetected", vote1, vote2).Return().Once()
	as.aggregator.notifier = notifier

	as.aggregator.StoreProposerVote(bp1.ProposerVote())
	as.aggregator.StoreProposerVote(bp2.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp1.Block)
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp2.Block)
	qc, built, err := as.aggregator.StoreVoteAndBuildQC(vote1, bp1.Block)
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
	qc, built, err = as.aggregator.StoreVoteAndBuildQC(vote2, bp2.Block)
	notifier.AssertExpectations(as.T())
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
}

// INVALID VOTES
// receive 4 invalid votes, and then the block, no QC should be built
// should trigger the notifier when converting pending votes
func (as *AggregatorSuite) TestInvalidVotesOnly() {
	notifier := &mocks.Consumer{}
	as.aggregator.notifier = notifier
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		// vote view is invalid
		vote := newMockVote(as, testView-1, bp.Block.BlockID, as.participants[i].NodeID)
		notifier.On("OnInvalidVoteDetected", vote)
		as.aggregator.StorePendingVote(vote)
	}
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
	notifier.AssertExpectations(as.T())
}

// INVALID VOTES
// receive 1 invalid vote, and 4 valid votes, and then the block, a QC should be built
func (as *AggregatorSuite) TestVoteMixtureBeforeBlock() {
	testView := uint64(5)
	notifier := &mocks.Consumer{}
	as.aggregator.notifier = notifier
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	// testing invalid pending votes
	for i := 0; i < 5; i++ {
		var vote *model.Vote
		if i == 0 {
			// vote view is invalid
			vote = newMockVote(as, testView-1, bp.Block.BlockID, as.participants[i].NodeID)
			notifier.On("OnInvalidVoteDetected", vote)
		} else {
			vote = newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		}
		as.aggregator.StorePendingVote(vote)
	}
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.NotNil(as.T(), qc)
	require.True(as.T(), built)
	require.NoError(as.T(), err)
	notifier.AssertExpectations(as.T())
}

// INVALID VOTES
// receive the block, and 3 valid vote, and 1 invalid vote, no QC should be built
func (as *AggregatorSuite) TestVoteMixtureAfterBlock() {
	testView := uint64(5)
	notifier := &mocks.Consumer{}
	as.aggregator.notifier = notifier
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	// testing invalid pending votes
	for i := 0; i < 4; i++ {
		var vote *model.Vote
		if i < 3 {
			vote = newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		} else {
			// vote view is invalid
			vote = newMockVote(as, testView-1, bp.Block.BlockID, as.participants[i].NodeID)
			notifier.On("OnInvalidVoteDetected", vote)
		}
		qc, built, err := as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
		require.Nil(as.T(), qc)
		require.False(as.T(), built)
		require.NoError(as.T(), err)
		notifier.AssertExpectations(as.T())
	}
}

// DUPLICATION
// receive the block, and the same valid votes for 5 times, no QC should be built
func (as *AggregatorSuite) TestDuplicateVotesAfterBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[1].NodeID)
	for i := 0; i < 4; i++ {
		qc, built, err := as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
		require.Nil(as.T(), qc)
		require.False(as.T(), built)
		require.NoError(as.T(), err)
		// only one vote and the primary vote are added
		require.Equal(as.T(), 2, len(as.aggregator.blockIDToVotingStatus[bp.Block.BlockID].votes))
	}
}

// DUPLICATION
// receive same valid votes for 5 times, then the block, no QC should be built
func (as *AggregatorSuite) TestDuplicateVotesBeforeBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[1].NodeID)
	for i := 0; i < 4; i++ {
		_ = as.aggregator.StorePendingVote(vote)
	}
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
}

// ORDER
// receive 5 votes, and the block, the QC should contain leader's vote, and the first 4 votes.
func (as *AggregatorSuite) TestVoteOrderAfterBlock() {
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[len(as.participants)-1].NodeID)
	var voteList []*model.Vote
	voteList = append(voteList, bp.ProposerVote())
	for i := 0; i < 5; i++ {
		vote := newMockVote(as, testView, bp.Block.BlockID, as.participants[i].NodeID)
		as.aggregator.StorePendingVote(vote)
		voteList = append(voteList, vote)
	}
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	_, exists := as.aggregator.blockIDToVotingStatus[bp.Block.BlockID].votes[bp.ProposerVote().ID()]
	require.NotNil(as.T(), qc)
	require.True(as.T(), built)
	require.NoError(as.T(), err)
	require.True(as.T(), exists)
	// first five votes including proposer vote should be stored in voting status
	// while the 6th vote shouldn't
	for i := 0; i < 6; i++ {
		_, exists := as.aggregator.blockIDToVotingStatus[bp.Block.BlockID].votes[voteList[i].ID()]
		if i < 5 {
			require.True(as.T(), exists)
		} else {
			require.False(as.T(), exists)
		}
	}
}

// PRUNE
// receive votes for view 2, 3, 4, 5 without the block
// prune by 4, should have only vote for view 5 left
func (as *AggregatorSuite) TestPartialPruneBeforeBlock() {
	pruneView := uint64(4)
	var voteList []*model.Vote
	var blockList []*model.Block
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(as, view, unittest.IdentifierFixture())
		vote := newMockVote(as, view, bp.Block.BlockID, as.participants[i].NodeID)
		as.aggregator.StorePendingVote(vote)
		voteList = append(voteList, vote)
		blockList = append(blockList, bp.Block)
	}
	// before pruning
	_, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(as.aggregator)
	require.Equal(as.T(), 4, viewToBlockLen)
	require.Equal(as.T(), 4, viewToVoteLen)
	require.Equal(as.T(), 4, pendingVoteLen)
	// after pruning
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 1, viewToBlockLen)
	require.Equal(as.T(), 1, viewToVoteLen)
	require.Equal(as.T(), 1, pendingVoteLen)
	// the remaining vote should be the vote that has view at 5
	lastVote := voteList[len(voteList)-1]
	require.Equal(as.T(), uint64(5), lastVote.View)
	_, exists := as.aggregator.viewToVoteID[uint64(5)]
	require.True(as.T(), exists)
	_, exists = as.aggregator.viewToBlockIDSet[uint64(5)]
	require.True(as.T(), exists)
	_, exists = as.aggregator.pendingVotes.votes[lastVote.BlockID].voteMap[lastVote.ID()]
	require.True(as.T(), exists)
}

// PRUNE
// receive the block and the votes for view 2, 3, 4, 5
// prune by 4, should have only vote for view 5 left
func (as *AggregatorSuite) TestPartialPruneAfterBlock() {
	pruneView := uint64(4)
	var voteList []*model.Vote
	var blockList []*model.Block
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(as, view, as.participants[i].NodeID)
		as.aggregator.StoreProposerVote(bp.ProposerVote())
		_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
		vote := newMockVote(as, view, bp.Block.BlockID, as.participants[i].NodeID)
		_, _, _ = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
		voteList = append(voteList, vote)
		blockList = append(blockList, bp.Block)
	}
	// before pruning
	_, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(as.aggregator)
	require.Equal(as.T(), 4, viewToBlockLen)
	require.Equal(as.T(), 4, viewToVoteLen)
	require.Equal(as.T(), 4, votingStatusLen)
	// after pruning
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 1, viewToBlockLen)
	require.Equal(as.T(), 1, viewToVoteLen)
	require.Equal(as.T(), 1, votingStatusLen)
	// the remaining vote should be the vote that has view at 5
	lastVote := voteList[len(voteList)-1]
	require.Equal(as.T(), uint64(5), lastVote.View)
	_, exists := as.aggregator.viewToVoteID[uint64(5)]
	require.True(as.T(), exists)
	_, exists = as.aggregator.viewToBlockIDSet[uint64(5)]
	require.True(as.T(), exists)
	_, exists = as.aggregator.blockIDToVotingStatus[lastVote.BlockID].votes[lastVote.ID()]
	require.True(as.T(), exists)
}

// PRUNE
// receive votes for view 2, 3, 4, 5 without the block
// prune by 6, should have no vote left
func (as *AggregatorSuite) TestFullPruneBeforeBlock() {
	pruneView := uint64(6)
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		vote := newMockVote(as, view, unittest.IdentifierFixture(), as.participants[i].NodeID)
		as.aggregator.StorePendingVote(vote)
	}
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 0, viewToBlockLen)
	require.Equal(as.T(), 0, viewToVoteLen)
	require.Equal(as.T(), 0, pendingVoteLen)
}

// PRUNE
// receive the block and the votes for view 2, 3, 4, 5
// prune by 6, should have no vote left
func (as *AggregatorSuite) TestFullPruneAfterBlock() {
	pruneView := uint64(6)
	for i := 2; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(as, view, as.participants[i].NodeID)
		as.aggregator.StoreProposerVote(bp.ProposerVote())
		_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
		vote := newMockVote(as, view, bp.Block.BlockID, as.participants[i].NodeID)
		_, _, _ = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
	}
	_, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(as.aggregator)
	require.Equal(as.T(), 4, viewToBlockLen)
	require.Equal(as.T(), 4, viewToVoteLen)
	require.Equal(as.T(), 4, votingStatusLen)
	// after pruning
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 0, viewToBlockLen)
	require.Equal(as.T(), 0, viewToVoteLen)
	require.Equal(as.T(), 0, votingStatusLen)
}

// PRUNE
// receive votes for view 3, 4, 5 without block
// prune by 2 twice, should have all votes there
func (as *AggregatorSuite) TestNonePruneBeforeBlock() {
	pruneView := uint64(2)
	for i := 3; i <= 5; i++ {
		view := uint64(i)
		vote := newMockVote(as, view, unittest.IdentifierFixture(), as.participants[i].NodeID)
		as.aggregator.StorePendingVote(vote)
	}
	_, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(as.aggregator)
	require.Equal(as.T(), 3, viewToBlockLen)
	require.Equal(as.T(), 3, viewToVoteLen)
	require.Equal(as.T(), 3, pendingVoteLen)
	// after pruning
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ := getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 3, viewToBlockLen)
	require.Equal(as.T(), 3, viewToVoteLen)
	require.Equal(as.T(), 3, pendingVoteLen)
	// prune twice
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, pendingVoteLen, _ = getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 3, viewToBlockLen)
	require.Equal(as.T(), 3, viewToVoteLen)
	require.Equal(as.T(), 3, pendingVoteLen)
}

// PRUNE
// receive the block and the votes for view 3, 4, 5
// prune by 2 twice, should have all votes there
func (as *AggregatorSuite) TestNonePruneAfterBlock() {
	pruneView := uint64(2)
	for i := 3; i <= 5; i++ {
		view := uint64(i)
		bp := newMockBlock(as, view, as.participants[i].NodeID)
		as.aggregator.StoreProposerVote(bp.ProposerVote())
		_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
		vote := newMockVote(as, view, bp.Block.BlockID, as.participants[i].NodeID)
		_, _, _ = as.aggregator.StoreVoteAndBuildQC(vote, bp.Block)
	}
	_, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(as.aggregator)
	require.Equal(as.T(), 3, viewToBlockLen)
	require.Equal(as.T(), 3, viewToVoteLen)
	require.Equal(as.T(), 3, votingStatusLen)
	// after pruning
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen := getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 3, viewToBlockLen)
	require.Equal(as.T(), 3, viewToVoteLen)
	require.Equal(as.T(), 3, votingStatusLen)
	// prune twice
	as.aggregator.PruneByView(pruneView)
	prunedView, viewToBlockLen, viewToVoteLen, _, votingStatusLen = getStateLength(as.aggregator)
	require.Equal(as.T(), pruneView, prunedView)
	require.Equal(as.T(), 3, viewToBlockLen)
	require.Equal(as.T(), 3, viewToVoteLen)
	require.Equal(as.T(), 3, votingStatusLen)
}

// RANDOM BEACON
// if there are 7 nodes, it requires 4 votes for random beacon,
// and receives the block from proposer who has 80% stake,
// then no qc should be built
func (as *AggregatorSuite) TestOnlyProposerVote() {
	// first node has 80% of stake
	as.participants[0].Stake = 5600
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[0].NodeID)
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	qc, built, err := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
}

// RANDOM BEACON
// if there are 7 nodes, it requires 4 votes for random beacon,
// and receives the block from proposer who has 40% stake,
// and a vote whose sender has 40% stake,
// then no qc should be built
func (as *AggregatorSuite) TestInSufficientRBSig() {
	// first two node has 80% of stake in total
	as.participants[0].Stake = 2800
	as.participants[1].Stake = 2800
	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[0].NodeID)
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, _, _ = as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	vote1 := newMockVote(as, testView, bp.Block.BlockID, as.participants[1].NodeID)
	qc, built, err := as.aggregator.StoreVoteAndBuildQC(vote1, bp.Block)
	require.Nil(as.T(), qc)
	require.False(as.T(), built)
	require.NoError(as.T(), err)
}

// RANDOM BEACON
// if there are 7 nodes, it requires 4 votes for random beacon,
// and receives the block from proposer who has 40% stake,
// and a vote whose sender has 40% stake,
// and another two votes with random stake,
// then a qc should be built
func (as *AggregatorSuite) TestSufficientRBSig() {
	// Define Stakes (number is percent)
	as.participants[0].Stake = 40 // accumulated:  40%
	as.participants[1].Stake = 20 // accumulated:  60%
	as.participants[2].Stake = 6  // accumulated:  66%
	as.participants[3].Stake = 1  // accumulated:  67%
	as.participants[4].Stake = 13 // accumulated:  80%
	as.participants[5].Stake = 10 // accumulated:  90%
	as.participants[6].Stake = 10 // accumulated: 100%

	testView := uint64(5)
	bp := newMockBlock(as, testView, as.participants[0].NodeID)
	as.aggregator.StoreProposerVote(bp.ProposerVote())
	_, built, _ := as.aggregator.BuildQCOnReceivedBlock(bp.Block)
	require.False(as.T(), built)
	vote1 := newMockVote(as, testView, bp.Block.BlockID, as.participants[1].NodeID)
	_, built, _ = as.aggregator.StoreVoteAndBuildQC(vote1, bp.Block)
	require.False(as.T(), built)
	vote2 := newMockVote(as, testView, bp.Block.BlockID, as.participants[2].NodeID)
	qc, built, err := as.aggregator.StoreVoteAndBuildQC(vote2, bp.Block)
	require.NoError(as.T(), err)
	require.False(as.T(), built)
	require.Nil(as.T(), qc)
	vote3 := newMockVote(as, testView, bp.Block.BlockID, as.participants[3].NodeID)
	qc, built, err = as.aggregator.StoreVoteAndBuildQC(vote3, bp.Block)
	require.NoError(as.T(), err)
	require.True(as.T(), built)
	require.NotNil(as.T(), qc)
}

func newMockBlock(as *AggregatorSuite, view uint64, proposerID flow.Identifier) *model.Proposal {
	blockHeader := unittest.BlockHeaderFixture()
	blockHeader.View = view
	block := &model.Block{
		View:       view,
		BlockID:    blockHeader.ID(),
		ProposerID: proposerID,
	}
	sig := crypto.Signature{}
	bp := &model.Proposal{
		Block:   block,
		SigData: sig,
	}
	as.RegisterProposal(bp)
	return bp
}

func newMockVote(as *AggregatorSuite, view uint64, blockID flow.Identifier, signerID flow.Identifier) *model.Vote {
	vote := &model.Vote{
		View:     view,
		BlockID:  blockID,
		SignerID: signerID,
		SigData:  []byte{},
	}
	as.RegisterVote(vote)
	return vote
}

func getStateLength(aggregator *VoteAggregator) (uint64, int, int, int, int) {
	return aggregator.highestPrunedView, len(aggregator.viewToBlockIDSet), len(aggregator.viewToVoteID), len(aggregator.pendingVotes.votes), len(aggregator.blockIDToVotingStatus)
}
