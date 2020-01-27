package hotstuff

import (
	"log"
	"math/rand"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

const VOTE_SIZE int = 10
const THRESHOLD float32 = 0.66667
const STAKE uint64 = 10

// threshold stake would be 5,
// meaning that it is enough to build a QC when receiving 5 votes
const VALIDATOR_SIZE = 7

// receive 5 valid incorporated votes in total
// a QC will be generated on receiving the 5th vote
func TestHappyPathForIncorporatedVotes(t *testing.T) {

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
	var vaLogger zerolog.Logger

	va := NewVoteAggregator(vaLogger, &ViewState{}, &Validator{})
	bp1 := &types.BlockProposal{
		Block: &types.Block{
			QC:          &types.QuorumCertificate{},
			View:        1,
			PayloadHash: []byte("first block"),
		},
	}
	vote1 := newMockVote(1, bp1.BlockMRH(), uint32(1))
	va.StoreVoteAndBuildQC(vote1, bp1)
	bp2 := &types.BlockProposal{
		Block: &types.Block{
			View:        1,
			PayloadHash: []byte("second block"),
		},
	}
	vote2 := newMockVote(1, bp2.BlockMRH(), uint32(1))
	_, err := va.StoreVoteAndBuildQC(vote2, bp2)
	if err != nil {
		switch err.(type) {
		case types.ErrDoubleVote:
			log.Printf("double vote detected %v", err)
		default:
			t.Errorf("double vote not detected")
		}
	} else {
		t.Errorf("double vote not detected")
	}
}

// store random votes and QCs from view 1 to 3
// prune by view 3
func TestPruneByView(t *testing.T) {

}

func generateMockVotes(size int, view uint64) []*types.Vote {
	var votes []*types.Vote
	for i := 0; i < size; i++ {
		mockVote := newMockVote(view, [32]byte{}, rand.Uint32())
		votes = append(votes, mockVote)
	}

	return votes
}

func newMockVote(view uint64, blockMRH [32]byte, signer uint32) *types.Vote {
	vote := &types.Vote{
		View:     view,
		BlockMRH: blockMRH,
		Signature: &types.Signature{
			RawSignature: [32]byte{},
			SignerIdx:    signer,
		},
	}

	return vote
}
