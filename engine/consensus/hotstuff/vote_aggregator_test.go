package hotstuff

import (
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

const VOTE_SIZE int = 10
const THRESHOLD float32 = 0.66667
const STAKE uint64 = 10

// threshold stake would be 5,
// meaning that it is enough to build a QC when receiving 5 votes
const VALIDATOR_SIZE = 7

type State struct {
}

// AtBlockID provides a mock function with given fields: blockID
func (_m *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	var r0 protocol.Snapshot
	return r0
}

// AtNumber provides a mock function with given fields: number
func (_m *State) AtNumber(number uint64) protocol.Snapshot {
	var r0 protocol.Snapshot
	return r0
}

// Final provides a mock function with given fields:
func (_m *State) Final() protocol.Snapshot {
	var r0 protocol.Snapshot
	return r0
}

// Mutate provides a mock function with given fields:
func (_m *State) Mutate() protocol.Mutator {
	var r0 protocol.Mutator
	return r0
}

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

// receive vote1 and 2 vote2
// the stake of vote2 should only be accumulated once
func TestDuplicateVotes(t *testing.T) {

}

// store one vote in the memory
// receive another vote with the same voter and the same view
// should trigger ErrDoubleVote
func TestErrDoubleVote(t *testing.T) {
}

// store random votes and QCs from view 1 to 3
// prune by view 3
func TestPruneByView(t *testing.T) {
}
