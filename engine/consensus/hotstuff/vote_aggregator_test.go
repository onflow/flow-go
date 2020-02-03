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

const VOTE_SIZE int = 10
const THRESHOLD float32 = 0.66667

// threshold stake would be 5,
// meaning that it is enough to build a QC when receiving 5 votes
const VALIDATOR_SIZE = 7

// receive 5 valid incorporated votes in total
// a QC will be generated on receiving the 5th vote
func TestHappyPathForIncorporatedVotes(t *testing.T) {
	va := newMockVoteAggregator(t)
	testView := uint64(5)
	block := newMockBlock(testView)
	for i := 1; i <= 5; i++ {
		vote := newMockVote(testView, block.BlockID(), uint32(i))
		qc, err := va.StoreVoteAndBuildQC(vote, block)
		if i < 5 {
			require.NotNil(t, err)
			require.True(t, errors.Is(err, types.ErrInsufficientVotes{}))
			fmt.Println(err.Error())
		} else {
			require.Nil(t, err)
			require.NotNil(t, qc)
			require.Equal(t, block.BlockID(), qc.BlockID)
			fmt.Println("QC generated")
		}
	}
}

// receive 5 valid votes in total
// 1. the first 3 votes are pending (threshold stake is not reached)
//    then receive the missing block (extracting a vote from the block as the 4th vote)
//    and no QC should be generated
//    when receive the 5th vote, a QC should be generated
// 2. all 5 votes are pending
//    then receive the missing block and a QC should be generated
// receive 7 votes in total
// 1. the first 2 of them are invalid
//    no QC should be generated until receiving the 7th vote
// 2. when receive the 5th vote, a QC should be generated
//    on receiving the 6th vote, return the same QC generated before
func TestHappyPathForPendingVotes(t *testing.T) {

}

// receive 7 votes in total
// 1. 3 of them are invalid and located every other valid vote
//    i.e., 1.valid, 2. invalid, ..., 6. valid, 7. invalid
//    no QC should be generated in the whole process
func TestUnHappyPathForIncorporatedVotes(t *testing.T) {

}

// receive 7 votes in total
// 1. all 7 votes are pending and invalid (cannot pass ValidatePendingVotes), no votes should be stored
// 2. all 7 votes are pending and can pass ValidatePendingVotes, but cannot pass ValidateIncorporatedVotes
//    when receiving the block, no vote should be moved to incorporatedVotes, and no QC should be generated
func TestUnHappyPathForPendingVotes(t *testing.T) {

}

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
	require.NoError(t, err)
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

// store random votes and QCs from view 1 to 3
// prune by view 3
func TestPruneByView(t *testing.T) {
	va := newMockVoteAggregator(t)
	pruneByView := 3
	for i := 1; i <= int(pruneByView); i++ {
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
	return types.NewBlockProposal(block, nil)
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
	ids := unittest.IdentityListFixture(7, unittest.WithRole(flow.RoleConsensus))
	snapshot := protocol.NewMockSnapshot(ctrl)
	mockProtocolState := mocks.NewMockState(ctrl)
	mockProtocolState.EXPECT().AtBlockID(gomock.Any()).Return(snapshot).AnyTimes()
	snapshot.EXPECT().Identities(gomock.Any()).Return(ids, nil).AnyTimes()
	viewState := &ViewState{protocolState: mockProtocolState}
	voteValidator := &Validator{viewState: viewState}

	return NewVoteAggregator(zerolog.Logger{}, 0, viewState, voteValidator)
}
