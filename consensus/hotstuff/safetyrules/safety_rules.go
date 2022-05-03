package safetyrules

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SafetyRules produces votes for the given block
type SafetyRules struct {
	signer        hotstuff.Signer
	forks         hotstuff.ForksReader
	persist       hotstuff.Persister
	committee     hotstuff.VoterCommittee // only produce votes when we are valid committee members
	lastVotedView uint64                  // need to keep track of the last view we voted for so we don't double vote accidentally
}

// New creates a new SafetyRules instance
func New(
	signer hotstuff.Signer,
	forks hotstuff.ForksReader,
	persist hotstuff.Persister,
	committee hotstuff.VoterCommittee,
	lastVotedView uint64,
) *SafetyRules {

	return &SafetyRules{
		signer:        signer,
		forks:         forks,
		persist:       persist,
		committee:     committee,
		lastVotedView: lastVotedView,
	}
}

// ProduceVote will make a decision on whether it will vote for the given proposal, the returned
// error indicates whether to vote or not.
// In order to ensure that only a safe node will be voted, SafetyRules will ask Forks whether a vote is a safe node or not.
// The curView is taken as input to ensure SafetyRules will only vote for proposals at current view and prevent double voting.
// Returns:
//  * (vote, nil): On the _first_ block for the current view that is safe to vote for.
//    Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
//  * (nil, model.NoVoteError): If the voter decides that it does not want to vote for the given block.
//    This is a sentinel error and _expected_ during normal operation.
// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (v *SafetyRules) ProduceVote(proposal *model.Proposal, curView uint64) (*model.Vote, error) {
	block := proposal.Block
	// sanity checks:
	if curView != block.View {
		return nil, fmt.Errorf("expecting block for current view %d, but block's view is %d", curView, block.View)
	}
	if curView <= v.lastVotedView {
		return nil, fmt.Errorf("current view (%d) must be larger than the last voted view (%d)", curView, v.lastVotedView)
	}

	// Do not produce a vote for blocks where we are not a valid committee member.
	// HotStuff will ask for a vote for the first block of the next epoch, even if we
	// have zero weight in the next epoch. Such vote can't be used to produce valid QCs.
	_, err := v.committee.IdentityByEpoch(block.View, v.committee.Self())
	if model.IsInvalidSignerError(err) {
		return nil, model.NoVoteError{Msg: "not voting committee member for block"}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get self identity: %w", err)
	}

	// generate vote if block is safe to vote for
	if !v.forks.IsSafeBlock(block) {
		return nil, model.NoVoteError{Msg: "not safe block"}
	}
	vote, err := v.signer.CreateVote(block)
	if err != nil {
		return nil, fmt.Errorf("could not vote for block: %w", err)
	}

	// vote for the current view has been produced, update lastVotedView
	// to prevent from voting for the same view again
	v.lastVotedView = curView
	err = v.persist.PutVoted(curView)
	if err != nil {
		return nil, fmt.Errorf("could not persist last voted: %w", err)
	}

	return vote, nil
}

// ProduceTimeout takes current view, highest locally known QC and TC and decides whether to produce timeout for current view.
// Returns:
//  * (timeout, nil): On the _first_ block for the current view that is safe to vote for.
//    Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
//  * (nil, model.NoTimeoutError): If the safety module decides that it is not safe to timeout under current conditions.
//    This is a sentinel error and _expected_ during normal operation.
// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (v *SafetyRules) ProduceTimeout(curView uint64, highestQC *flow.QuorumCertificate, highestTC *flow.TimeoutCertificate) (*model.TimeoutObject, error) {
	panic("to be implemented")
}

// IsSafeToVote checks if this proposal is valid in terms of voting rules, if voting for this proposal won't break safety rules.
func (v *SafetyRules) IsSafeToVote(proposal *model.Proposal) bool {
	panic("to be implemented")
}

// IsSafeToTimeout checks if it's safe to timeout with proposed data, if timing out won't break safety rules.
func (v *SafetyRules) IsSafeToTimeout(curView uint64, highestQC *flow.QuorumCertificate, highestTC *flow.TimeoutCertificate) bool {
	panic("to be implemented")
}
