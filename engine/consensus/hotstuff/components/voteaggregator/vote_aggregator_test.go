package voteaggregator

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
	"math/rand"
	"testing"
)

const VOTE_SIZE int = 10
const THRESHOLD float32 = 0.66667
const STAKE uint64 = 10

// threshold stake would be 5
const VALIDATOR_SIZE = 7

type ViewParameter struct {
	thresholdStake float32
}

// receive 5 valid incorporated votes in total
// a QC will be generated on receiving the 5th vote
func TestHappyPathForIncorporatedVotes(t *testing.T) {

}

// receive 5 valid votes in total
// 1. the first 4 votes are pending (threshold stake is not reached)
//    then receive the missing block and no QC should be generated
//    when receive the 5th vote, a QC should be generated
// 2. all 5 votes are pending
//    then receive the missing block and a QC should be generated
// receive 7 votes in total
// 1. the first 2 of them are invalid
//    no QC should be generated until receiving the 7th vote
// 2. when receive the 5th vote, a QC should be generated
//    on receiving the 6th vote, the old QC will be returned, no new QC should be generated
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

// receive a vote with the same view of a created QC, qcExistedError should be triggered
func TestErrExistingQC_Error(t *testing.T) {

}

func (vp *ViewParameter) ThresholdState() float32 {
	return vp.thresholdStake
}

func generateMockVotes(size int) []*types.Vote {
	var votes []*types.Vote
	for i := 0; i < size; i++ {
		vote := &types.Vote{
			View:     uint64(i),
			BlockMRH: []byte{},
			Signature: &types.Signature{
				RawSignature: [32]byte{},
				SignerIdx:    0,
			},
		}
		rand.Read(vote.BlockMRH[:])
		votes = append(votes, vote)
	}

	return votes
}

func generateMockProtocolState(t *testing.T) {
}
