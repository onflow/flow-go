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
	LockedOneChainView uint64
	// HighestAcknowledgedView is the highest view where we have voted or triggered a timeout
	HighestAcknowledgedView uint64
	// LastTimeout is the last timeout that was produced by this node (may be nil if no timeout occurred yet)
	LastTimeout *model.TimeoutObject
}

// SafetyRules enforces all consensus rules that guarantee safety. It produces votes for
// the given blocks or TimeoutObject for the given views, only if all safety rules are satisfied.
// In particular, SafetyRules guarantees a foundational security theorem for HotStuff (incl.
// the DiemBFT / Jolteon variant), which we utilize also outside of consensus (e.g. queuing pending
// blocks for execution, verification, sealing etc):
//
//	THEOREM: For each view, there can be at most 1 certified block.
//
// Implementations are generally *not* concurrency safe.
type SafetyRules interface {
	// ProduceVote takes a block proposal and current view, and decides whether to vote for the block.
	// Voting is deterministic, i.e. voting for same proposal will always result in the same vote.
	// Returns:
	//  * (vote, nil): On the _first_ block for the current view that is safe to vote for.
	//    Subsequently, voter does _not_ vote for any _other_ block with the same (or lower) view.
	//    SafetyRules internally caches and persists its latest vote. As long as the SafetyRules' internal
	//    state remains unchanged, ProduceVote will return its cached for identical inputs.
	//  * (nil, model.NoVoteError): If the safety module decides that it is not safe to vote for the given block.
	//    This is a sentinel error and _expected_ during normal operation.
	// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
	ProduceVote(proposal *model.SignedProposal, curView uint64) (*model.Vote, error)

	// ProduceTimeout takes current view, highest locally known QC and TC (optional, must be nil if and
	// only if QC is for previous view) and decides whether to produce timeout for current view.
	// Returns:
	//  * (timeout, nil): It is safe to timeout for current view using newestQC and lastViewTC.
	//  * (nil, model.NoTimeoutError): If replica is not part of the authorized consensus committee (anymore) and
	//    therefore is not authorized to produce a valid timeout object. This sentinel error is _expected_ during
	//    normal operation, e.g. during the grace-period after Epoch switchover or after the replica self-ejected.
	// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
	ProduceTimeout(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*model.TimeoutObject, error)

	// SignOwnProposal takes an unsigned block proposal and produces a vote for it. Vote is a cryptographic commitment
	// to the proposal. By adding the vote to an unsigned proposal, the caller constructs a signed block proposal. This
	// method has to be used only by the leader, which must be the proposer of the block (or an exception is returned).
	// Implementors must guarantee that:
	// - vote on the proposal satisfies safety rules
	// - maximum one proposal is signed per view
	// Returns:
	//  * (vote, nil): the passed unsigned proposal is a valid one, and it's safe to make a proposal.
	//    Subsequently, leader does _not_ produce any _other_ proposal with the same (or lower) view.
	//  * (nil, model.NoVoteError): according to HotStuff's Safety Rules, it is not safe to sign the given proposal.
	//    This could happen because we have already proposed or timed out for the given view.
	//    This is a sentinel error and _expected_ during normal operation.
	// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
	SignOwnProposal(unsignedProposal *model.Proposal) (*model.Vote, error)
}
