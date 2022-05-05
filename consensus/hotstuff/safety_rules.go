package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type SafetyData struct {
	// LockedOneChainView is the head block's view of the newest 1-chain this replica has voted for.
	// The 1-chain can be indirect.
	//     <·· <QC>[B0] <- <QC_B0>[B1] <- [my vote for B1]
	// In the depicted scenario, the replica voted for block B1, which forms a (potentially indirect)
	// 1-chain on top of B0. The replica updated LockedOneChainView to the max of the current value and
	// QC_B0.View = B0.View. Thereby, the safety module guarantees that the replica will not sign
	// a TimeoutObject that would allow a malicious leader to fork below the latest finalized block.
	LockedOneChainView uint64 // highest preferred QC, replica updates this value when voting for block (equivalent to highestQCView from Safety rules algorithm)
	// HighestAcknowledgedView is the highest view where we have voted or triggered a timeout
	HighestAcknowledgedView uint64
	// LastTimeout is the last timeout that was produced by this node
	LastTimeout *model.TimeoutObject
}

// SafetyRules enforces all consensus rules that guarantee safety. It produces votes for
// the given blocks or TimeoutObject for the given views, only if all safety rules are satisfied.
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
}
