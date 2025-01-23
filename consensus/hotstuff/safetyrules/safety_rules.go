package safetyrules

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// CAUTION Tech Debt: The current implementation does not use SafetyRules, when a leader signs their own proposal.
// This strongly complicates the safety argument, where enforcing the safety-critical property of only ever voting
// once per view is distributed across different layers in the software stack.
// For further details, see issue https://github.com/onflow/flow-go/issues/6389

// SafetyRules is a dedicated module that enforces consensus safety. This component has the sole authority to generate
// votes and timeouts. It follows voting and timeout rules for creating votes and timeouts respectively.
// Caller can be sure that created vote or timeout doesn't break safety and can be used in consensus process.
// SafetyRules relies on hotstuff.Persister to store latest state of hotstuff.SafetyData.
//
// The voting rules implemented by SafetyRules are:
//  1. Replicas vote in strictly increasing views. At most one vote can be signed per view.
//     Caution: The leader's block signature is formally a vote for their own proposal.
//  2. Each block has to include a TC or a QC from the previous view.
//     a. [Happy path] If the previous view resulted in a QC then the proposer should include it in their block.
//     b. [Recovery path] If the previous round did *not* result in a QC, the leader of the
//     subsequent round *must* include a valid TC for the previous view in its block.
//
// Condition 1 guarantees a foundational security theorem for HotStuff (incl. the DiemBFT / Jolteon variant):
//
//	THEOREM: For each view, there can be at most 1 certified block.
//
// NOT safe for concurrent use.
type SafetyRules struct {
	signer     hotstuff.Signer
	persist    hotstuff.Persister
	committee  hotstuff.DynamicCommittee // only produce votes when we are valid committee members
	safetyData *hotstuff.SafetyData
}

var _ hotstuff.SafetyRules = (*SafetyRules)(nil)

// New creates a new SafetyRules instance
func New(
	signer hotstuff.Signer,
	persist hotstuff.Persister,
	committee hotstuff.DynamicCommittee,
) (*SafetyRules, error) {
	// get the last stored safety data
	safetyData, err := persist.GetSafetyData()
	if err != nil {
		return nil, fmt.Errorf("could not recover safety data: %w", err)
	}

	return &SafetyRules{
		signer:     signer,
		persist:    persist,
		committee:  committee,
		safetyData: safetyData,
	}, nil
}

// ProduceVote will make a decision on whether it will vote for the given proposal, the returned
// error indicates whether to vote or not.
// To ensure that only safe proposals are being voted on, we check that the proposer is a valid committee member and that the
// proposal complies with voting rules.
// We expect that only well-formed proposals with valid signatures are submitted for voting.
// The curView is taken as input to ensure SafetyRules will only vote for proposals at current view and prevent double voting.
// Returns:
//   - (vote, nil): On the _first_ block for the current view that is safe to vote for.
//     Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
//   - (nil, model.NoVoteError): If the voter decides that it does not want to vote for the given block.
//     This is a sentinel error and _expected_ during normal operation.
//
// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (r *SafetyRules) ProduceVote(signedProposal *model.SignedProposal, curView uint64) (*model.Vote, error) {
	return r.produceVote(&signedProposal.Proposal, curView)
}

// produceVote implements the core Safety Rules to validate whether it is safe to vote.
// This method is to be used to vote for other leaders' blocks as well as this node's own proposals
// under construction. We explicitly codify the important aspect that a proposer's signature for their
// own block is conceptually also just a vote (we explicitly use that property when aggregating votes and
// including the proposer's own vote into a QC). In order to express this conceptual equivalence in code, the
// voting logic in Safety Rules must also operate on an unsigned Proposal.
//
// The curView is taken as input to ensure SafetyRules will only vote for proposals at current view and prevent double voting.
// Returns:
//   - (vote, nil): On the _first_ block for the current view that is safe to vote for.
//     Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
//   - (nil, model.NoVoteError): If the voter decides that it does not want to vote for the given block.
//     This is a sentinel error and _expected_ during normal operation.
//
// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (r *SafetyRules) produceVote(proposal *model.Proposal, curView uint64) (*model.Vote, error) {
	block := proposal.Block
	// sanity checks:
	if curView != block.View {
		return nil, fmt.Errorf("expecting block for current view %d, but block's view is %d", curView, block.View)
	}

	err := r.isSafeToVote(proposal)
	if err != nil {
		return nil, fmt.Errorf("not safe to vote for proposal %x: %w", proposal.Block.BlockID, err)
	}

	currentLeader, err := r.committee.LeaderForView(block.View)
	if err != nil {
		return nil, fmt.Errorf("expect to have a valid leader for view %d: %w", curView, err)
	}
	// This sanity check confirms that the proposal is from the correct leader of this view. In case this sanity check
	// fails, we return an exception, because the compliance layer should have verified this already. However, proposals
	// from this node might not go through the compliance engine, and must be signed before anyway. Therefore,
	// we still include this sanity check, but return an exception because signing a proposal should be only for views
	// where this node is actually the leader.
	if block.ProposerID != currentLeader {
		return nil, fmt.Errorf("incorrect proposal, as proposer %x is different from the leader %x for view %d", block.ProposerID, currentLeader, curView)
	}

	// In case this node is the leader, we can skip the following checks.
	// • If this node is ejected (check (ii) would fail), voting for any blocks or signing own proposals is of no harm.
	//   This is because all other honest nodes should have terminated their connection to us, so we are not risking
	//   to use up the networking bandwidth of honest nodes. This is relevant in case of self-ejection: a node
	//   operator suspecting their node's keys to be compromised can request for their node to be ejected to prevent
	//   malicious actors impersonating their node, launching an attack on the network, and the stake being slashed.
	//   The self-ejection mechanism corresponds to key-revocation and reduces attack surface for the network and
	//   the node operator's stake. In case of self-ejection, a node is no longer part of the network, hence it cannot
	//   harm the network and is no longer subject to slashing for actions during the respective views. Therefore,
	//   voting or continuing to signing block proposals is of no concern.
	// • In case this node is the leader, `block.ProposerID` and `r.committee.Self()` are identical. In other words,
	//   check (i) also verifies that this node itself is not ejected -- the same as check (ii). Hence, also check
	//   (i) can be skipped with the same reasoning.
	if currentLeader != r.committee.Self() {
		// (i): we need to make sure that proposer is not ejected to vote
		_, err = r.committee.IdentityByBlock(block.BlockID, block.ProposerID)
		if model.IsInvalidSignerError(err) {
			// the proposer must be ejected since the proposal has already been validated,
			// which ensures that the proposer was a valid committee member at the start of the epoch
			return nil, model.NewNoVoteErrorf("proposer ejected: %w", err)
		}
		if err != nil {
			return nil, fmt.Errorf("internal error retrieving Identity of proposer %x at block %x: %w", block.ProposerID, block.BlockID, err)
		}

		// (ii) Do not produce a vote for blocks where we are not an active committee member. The HotStuff
		// state machine may request to vote during grace periods outside the epochs, where the node is
		// authorized to actively participate. If we voted during those grace periods, we would needlessly
		// waste network bandwidth, as such votes can't be used to produce valid QCs.
		_, err = r.committee.IdentityByBlock(block.BlockID, r.committee.Self())
		if model.IsInvalidSignerError(err) {
			return nil, model.NewNoVoteErrorf("I am not authorized to vote for block %x: %w", block.BlockID, err)
		}
		if err != nil {
			return nil, fmt.Errorf("could not get self identity: %w", err)
		}
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

// ProduceTimeout takes current view, highest locally known QC and TC (optional, must be nil if and
// only if QC is for previous view) and decides whether to produce timeout for current view.
// Returns:
//   - (timeout, nil): It is safe to timeout for current view using newestQC and lastViewTC.
//   - (nil, model.NoTimeoutError): If replica is not part of the authorized consensus committee (anymore) and
//     therefore is not authorized to produce a valid timeout object. This sentinel error is _expected_ during
//     normal operation, e.g. during the grace-period after Epoch switchover or after the replica self-ejected.
//
// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (r *SafetyRules) ProduceTimeout(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*model.TimeoutObject, error) {
	lastTimeout := r.safetyData.LastTimeout
	if lastTimeout != nil && lastTimeout.View == curView {
		// model.TimeoutObject are conceptually immutable, hence we create a shallow copy here, which allows us to increment TimeoutTick
		updatedTimeout := *lastTimeout
		updatedTimeout.TimeoutTick += 1

		// persist updated TimeoutObject in `safetyData` and return it
		r.safetyData.LastTimeout = &updatedTimeout
		err := r.persist.PutSafetyData(r.safetyData)
		if err != nil {
			return nil, fmt.Errorf("could not persist safety data: %w", err)
		}
		return r.safetyData.LastTimeout, nil
	}

	err := r.IsSafeToTimeout(curView, newestQC, lastViewTC)
	if err != nil {
		return nil, fmt.Errorf("local, trusted inputs failed safety rules: %w", err)
	}

	// Do not produce a timeout for view where we are not a valid committee member.
	_, err = r.committee.IdentityByEpoch(curView, r.committee.Self())
	if err != nil {
		if model.IsInvalidSignerError(err) {
			return nil, model.NewNoTimeoutErrorf("I am not authorized to timeout for view %d: %w", curView, err)
		}
		return nil, fmt.Errorf("could not get self identity: %w", err)
	}

	timeout, err := r.signer.CreateTimeout(curView, newestQC, lastViewTC)
	if err != nil {
		return nil, fmt.Errorf("could not create timeout at view %d: %w", curView, err)
	}

	r.safetyData.HighestAcknowledgedView = curView
	r.safetyData.LastTimeout = timeout

	err = r.persist.PutSafetyData(r.safetyData)
	if err != nil {
		return nil, fmt.Errorf("could not persist safety data: %w", err)
	}

	return timeout, nil
}

// SignOwnProposal takes an unsigned block proposal and produces a vote for it. Vote is a cryptographic commitment
// to the proposal. By adding the vote to an unsigned proposal, the caller constructs a signed block proposal. This
// method has to be used only by the leader, which must be the proposer of the block (or an exception is returned).
// Implementors must guarantee that:
// - vote on the proposal satisfies safety rules
// - maximum one proposal is signed per view
// Returns:
//   - (vote, nil): the passed unsigned proposal is a valid one, and it's safe to make a proposal.
//     Subsequently, leader does _not_ produce any _other_ proposal with the same (or lower) view.
//   - (nil, model.NoVoteError): according to HotStuff's Safety Rules, it is not safe to sign the given proposal.
//     This could happen because we have already proposed or timed out for the given view.
//     This is a sentinel error and _expected_ during normal operation.
//
// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
func (r *SafetyRules) SignOwnProposal(unsignedProposal *model.Proposal) (*model.Vote, error) {
	// check that the block is created by us
	if unsignedProposal.Block.ProposerID != r.committee.Self() {
		return nil, fmt.Errorf("can't sign proposal for someone else's block")
	}

	return r.produceVote(unsignedProposal, unsignedProposal.Block.View)
}

// isSafeToVote checks if this proposal is valid in terms of voting rules, if voting for this proposal won't break safety rules.
// Expected errors during normal operations:
//   - NoVoteError if replica already acted during this view (either voted or generated timeout)
func (r *SafetyRules) isSafeToVote(proposal *model.Proposal) error {
	blockView := proposal.Block.View

	err := r.validateEvidenceForEnteringView(blockView, proposal.Block.QC, proposal.LastViewTC)
	if err != nil {
		// As we are expecting the blocks to be pre-validated, any failure here is a symptom of an internal bug.
		return fmt.Errorf("proposal failed consensus validity check: %w", err)
	}

	// This check satisfies voting rule 1
	// 1. Replicas vote strictly in increasing rounds,
	// block's view must be greater than the view that we have voted for
	acView := r.safetyData.HighestAcknowledgedView
	if blockView == acView {
		return model.NewNoVoteErrorf("already voted or generated timeout in view %d", blockView)
	}
	if blockView < acView {
		return fmt.Errorf("already acted during view %d but got proposal for lower view %d", acView, blockView)
	}

	return nil
}

// IsSafeToTimeout checks if it's safe to timeout with proposed data, i.e. timing out won't break safety.
// newestQC is the valid QC with the greatest view that we have observed.
// lastViewTC is the TC for the previous view (might be nil).
//
// When generating a timeout, the inputs are provided by node-internal components. Failure to comply with
// the protocol is a symptom of an internal bug. We don't expect any errors during normal operations.
func (r *SafetyRules) IsSafeToTimeout(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) error {
	err := r.validateEvidenceForEnteringView(curView, newestQC, lastViewTC)
	if err != nil {
		return fmt.Errorf("not safe to timeout: %w", err)
	}

	if newestQC.View < r.safetyData.LockedOneChainView {
		return fmt.Errorf("have already seen QC for view %d, but newest QC is reported to be for view %d", r.safetyData.LockedOneChainView, newestQC.View)
	}
	if curView+1 <= r.safetyData.HighestAcknowledgedView {
		return fmt.Errorf("cannot generate timeout for past view %d", curView)
	}
	// the logic for rejecting inputs with `curView <= newestQC.View` is already contained
	// in `validateEvidenceForEnteringView(..)`, because it only passes if
	// * either `curView == newestQC.View + 1` (condition 2)
	// * or `curView > newestQC.View` (condition 4)

	return nil
}

// validateEvidenceForEnteringView performs the following check that is fundamental for consensus safety:
// Whenever a replica acts within a view, it must prove that is has sufficient evidence to enter this view
// Specifically:
//  1. The replica must always provide a QC and optionally a TC.
//  2. [Happy Path] If the previous round (i.e. `view -1`) resulted in a QC, the replica is allowed to transition to `view`.
//     The QC from the previous round provides sufficient evidence. Furthermore, to prevent resource-exhaustion attacks,
//     we require that no TC is included as part of the proof.
//  3. Following the Happy Path has priority over following the Recovery Path (specified below).
//  4. [Recovery Path] If the previous round (i.e. `view -1`) did *not* result in a QC, a TC from the previous round
//     is required to transition to `view`. The following additional consistency requirements have to be satisfied:
//     (a) newestQC.View + 1 < view
//     Otherwise, the replica has violated condition 3 (in case newestQC.View + 1 = view); or the replica
//     failed to apply condition 2 (in case newestQC.View + 1 > view).
//     (b) newestQC.View ≥ lastViewTC.NewestQC.View
//     Otherwise, the replica has violated condition 3.
//
// SafetyRules has the sole signing authority and enforces adherence to these conditions. In order to generate valid
// consensus signatures, the replica must provide the respective evidence (required QC + optional TC) to its
// internal SafetyRules component for each consensus action that the replica wants to take:
//   - primary signing its own proposal
//   - replica voting for a block
//   - replica generating a timeout message
//
// During normal operations, no errors are expected:
//   - As we are expecting the blocks to be pre-validated, any failure here is a symptom of an internal bug.
//   - When generating a timeout, the inputs are provided by node-internal components. Failure to comply with
//     the protocol is a symptom of an internal bug.
func (r *SafetyRules) validateEvidenceForEnteringView(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) error {
	// Condition 1:
	if newestQC == nil {
		return fmt.Errorf("missing the mandatory QC")
	}

	// Condition 2:
	if newestQC.View+1 == view {
		if lastViewTC != nil {
			return fmt.Errorf("when QC is for prior round, no TC should be provided")
		}
		return nil
	}
	// Condition 3: if we reach the following lines, the happy path is not satisfied.

	// Condition 4:
	if lastViewTC == nil {
		return fmt.Errorf("expecting TC because QC is not for prior view; but didn't get any TC")
	}
	if lastViewTC.View+1 != view {
		return fmt.Errorf("neither QC (view %d) nor TC (view %d) allows to transition to view %d", newestQC.View, lastViewTC.View, view)
	}
	if newestQC.View >= view {
		// Note: we need to enforce here that `newestQC.View + 1 < view`, i.e. we error for `newestQC.View+1 >= view`
		// However, `newestQC.View+1 == view` is impossible, because otherwise we would have walked into condition 2.
		// Hence, it suffices to error if `newestQC.View+1 > view`, which is identical to `newestQC.View >= view`
		return fmt.Errorf("still at view %d, despite knowing a QC for view %d", view, newestQC.View)
	}
	if newestQC.View < lastViewTC.NewestQC.View {
		return fmt.Errorf("failed to update newest QC (still at view %d) despite a newer QC (view %d) being included in TC", newestQC.View, lastViewTC.NewestQC.View)
	}

	return nil
}
