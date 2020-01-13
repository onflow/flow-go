package voteaggregator

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
	"math/rand"
	"testing"
)

const VOTE_SIZE int = 10
const THRESHOLD float32 = 0.66667
const STAKE uint64 = 10

// threshold stake would be 15
const VALIDATOR_SIZE = 20

type ViewParameter struct {
	thresholdStake float32
}

// receive 15 valid incorporated votes in total
// a QC will be generated on receiving the 15th vote
func TestHappyPathForIncorporatedVotes(t *testing.T) {

}

// receive 15 valid votes in total
// 1. the first 14 votes are pending (threshold stake is not reached)
//    then receive the missing block and no QC should be generated
//    when receive the 15th vote, a QC should be generated
// 2. all 15 votes are pending
//    then receive the missing block and a QC should be generated
// receive 20 votes in total
// 1.  the first 5 of them are invalid
//     no QC should be generated until receiving the 20th vote
func TestHappyPathForPendingVotes(t *testing.T) {

}

// receive 20 votes in total
// 1. 10 of them are invalid and located every other valid vote
//    i.e., 1.valid, 2. invalid, ..., 19. valid, 20. invalid
//    no QC should be generated in the whole process
func TestUnHappyPathForIncorporatedVotes(t *testing.T) {

}

// receive 20 votes in total
// 1. all 20 votes are pending and invalid (cannot pass ValidatePendingVotes), no votes should be stored
// 2. all 20 votes are pending and can pass ValidatePendingVotes, but cannot pass ValidateIncorporatedVotes
//    when receiving the block, no vote should be moved to incorporatedVotes, and no QC should be generated
func TestUnHappyPathForPendingVotes(t *testing.T) {

}

// receive 5 votes with the same view of a created QC, qcExistedError should be triggered
func TestQcExistedError(t *testing.T) {

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
