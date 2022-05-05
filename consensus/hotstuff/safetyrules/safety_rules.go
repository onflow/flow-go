package safetyrules

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// SafetyRules produces votes for the given block
type SafetyRules struct {
	signer     hotstuff.Signer
	forks      hotstuff.ForksReader
	persist    hotstuff.Persister
	committee  hotstuff.Replicas // only produce votes when we are valid committee members
	safetyData *hotstuff.SafetyData
}

var _ hotstuff.SafetyRules = (*SafetyRules)(nil)

// New creates a new SafetyRules instance
func New(
	signer hotstuff.Signer,
	forks hotstuff.ForksReader,
	persist hotstuff.Persister,
	committee hotstuff.Replicas,
	lastVotedView uint64,
) *SafetyRules {

	return &SafetyRules{
		signer:    signer,
		forks:     forks,
		persist:   persist,
		committee: committee,
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
func (r *SafetyRules) ProduceVote(proposal *model.Proposal, curView uint64) (*model.Vote, error) {
	block := proposal.Block
	// sanity checks:
	if curView != block.View {
		return nil, fmt.Errorf("expecting block for current view %d, but block's view is %d", curView, block.View)
	}

	err := r.IsSafeToVote(proposal)
	if err != nil {
		return nil, fmt.Errorf("not safe to vote for proposal %x: %w", proposal.Block.BlockID, err)
	}

	// Do not produce a vote for blocks where we are not a valid committee member.
	// HotStuff will ask for a vote for the first block of the next epoch, even if we
	// have zero weight in the next epoch. Such vote can't be used to produce valid QCs.
	_, err = r.committee.IdentityByEpoch(block.View, r.committee.Self())
	if model.IsInvalidSignerError(err) {
		return nil, model.NoVoteError{Msg: "not voting committee member for block"}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get self identity: %w", err)
	}

	vote, err := r.signer.CreateVote(block)
	if err != nil {
		return nil, fmt.Errorf("could not vote for block: %w", err)
	}

	// vote for the current view has been produced, update safetyData
	r.safetyData.HighestAcknowledgedView = curView
	if r.safetyData.LockedOneChainView < block.QC.View {
		r.safetyData.LockedOneChainView = block.QC.View
	}

	err = r.persist.PutSafetyData(r.safetyData)
	if err != nil {
		return nil, fmt.Errorf("could not persist safety data: %w", err)
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
func (r *SafetyRules) ProduceTimeout(curView uint64, highestQC *flow.QuorumCertificate, highestTC *flow.TimeoutCertificate) (*model.TimeoutObject, error) {
	lastTimeout := r.safetyData.LastTimeout
	if lastTimeout != nil && lastTimeout.View == curView {
		return lastTimeout, nil
	}

	if !r.IsSafeToTimeout(curView, highestQC, highestTC) {
		return nil, model.NoTimeoutError{Msg: "not safe to time out under current conditions"}
	}

	timeout, err := r.signer.CreateTimeout(curView, highestQC, highestTC)
	if err != nil {
		return nil, fmt.Errorf("could not timeout at view %d: %w", curView, err)
	}

	r.safetyData.HighestAcknowledgedView = curView
	r.safetyData.LastTimeout = timeout

	err = r.persist.PutSafetyData(r.safetyData)
	if err != nil {
		return nil, fmt.Errorf("could not persist safety data: %w", err)
	}

	return timeout, nil
}

// IsSafeToVote checks if this proposal is valid in terms of voting rules, if voting for this proposal won't break safety rules.
func (r *SafetyRules) IsSafeToVote(proposal *model.Proposal) error {
	blockView := proposal.Block.View
	qcView := proposal.Block.QC.View

	// block's view must be larger than the view of the included QC
	if blockView <= qcView {
		return fmt.Errorf("block's view %d must be larger than the view of the included QC %d", blockView, qcView)
	}

	// block's view must be greater than the view that we have voted for
	if proposal.Block.View <= r.safetyData.HighestAcknowledgedView {
		return model.NoVoteError{Msg: "not safe to vote, we have already voted for this view"}
	}

	// This check satisfies voting rule 1 and 2a.
	if blockView == qcView+1 {
		return nil
	}

	// This check satisfies voting rule 1 and 2b.
	lastViewTC := proposal.LastViewTC
	if lastViewTC != nil {
		if blockView == lastViewTC.View+1 {
			if qcView >= lastViewTC.TOHighestQC.View {
				return nil
			} else {
				return fmt.Errorf("QC's view %d should be at least %d", qcView, lastViewTC.TOHighestQC.View)
			}
		} else {
			return fmt.Errorf("last view TC %d is not sequential for block %d", lastViewTC.View, blockView)
		}
	}

	return fmt.Errorf("block's view %d is not sequential %d, last view TC not included", blockView, qcView)
}

// IsSafeToTimeout checks if it's safe to timeout with proposed data, if timing out won't break safety rules.
func (r *SafetyRules) IsSafeToTimeout(curView uint64, highestQC *flow.QuorumCertificate, highestTC *flow.TimeoutCertificate) bool {
	if highestQC.View < r.safetyData.LockedOneChainView ||
		curView+1 <= r.safetyData.HighestAcknowledgedView ||
		curView <= highestQC.View {
		return false
	}

	return (curView == highestQC.View+1) || (curView == highestTC.View+1)
}
