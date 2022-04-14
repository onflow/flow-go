package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SafetyRules enforces all consensus rules that guarantee safety. It produces votes for
// the given blocks or TimeoutObject for the givens, only if all safety rules are satisfied. 
type SafetyRules interface {
	// ProduceVote takes a block proposal and current view, and decides whether to vote for the block.
	// Voting is deterministic meaning voting for same proposal will always result in the same vote.
	// Returns:
	//  * (vote, nil): On the _first_ block for the current view that is safe to vote for.
	//    Subsequently, voter does _not_ vote for any _other_  block with the same (or lower) view.
	//    SafetyRules internally caches and persists its latest vote. As long as the SafetyRules' internal
	//    state remains unchanged, ProduceVote will return its cached for identical inputs.   

	//  * (nil, model.NoVoteError): If the safety module decides that it is not safe to vote for the given block.
	//    This is a sentinel error and _expected_ during normal operation.
	// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
	ProduceVote(proposal *model.Proposal, curView uint64) (*model.Vote, error)
	// ProduceTimeout takes current view, highest locally known QC and last round TC (may be nil) and decides whether to produce timeout for current view.
	// Returns:
	//  * (timeout, nil): It is safe to timeout for current view using highestQC and highestTC.
	//  * (nil, model.NoTimeoutError): If the safety module decides that it is not safe to timeout under current conditions.
	//    This is a sentinel error and _expected_ during normal operation.
	// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
	ProduceTimeout(curView uint64, highestQC *flow.QuorumCertificate, highestTC *flow.TimeoutCertificate) (*model.TimeoutObject, error)
	// IsSafeToVote checks if this proposal is valid in terms of voting rules, if voting for this proposal won't break safety rules.
	IsSafeToVote(proposal *model.Proposal) bool
	// IsSafeToTimeout checks if it's safe to timeout with proposed data, if timing out won't break safety rules.
	IsSafeToTimeout(curView uint64, highestQC *flow.QuorumCertificate, highestTC *flow.TimeoutCertificate) bool
}
