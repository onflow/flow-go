package votecollector

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
)

var (
	ErrDifferentCollectorState = errors.New("different state")
)

// VoteCollector implements a state machine for transition between different states of vote collector
type VoteCollector struct {
	sync.Mutex
	log                      zerolog.Logger
	workers                  hotstuff.Workers
	notifier                 hotstuff.Consumer
	createVerifyingProcessor hotstuff.VoteProcessorFactory

	votesCache     ViewSpecificVotesCache
	votesProcessor atomic.Value
}

var _ hotstuff.VoteCollector = (*VoteCollector)(nil)

func (m *VoteCollector) atomicLoadProcessor() hotstuff.VoteProcessor {
	return m.votesProcessor.Load().(*atomicValueWrapper).processor
}

// atomic.Value doesn't allow storing interfaces as atomic values,
// it requires that stored type is always the same, so we need a wrapper that will mitigate this restriction
// https://github.com/golang/go/issues/22550
type atomicValueWrapper struct {
	processor hotstuff.VoteProcessor
}

func NewStateMachineFactory(
	log zerolog.Logger,
	notifier hotstuff.Consumer,
	verifyingVoteProcessorFactory hotstuff.VoteProcessorFactory,
) voteaggregator.NewCollectorFactoryMethod {
	return func(view uint64, workers hotstuff.Workers) (hotstuff.VoteCollector, error) {
		return NewStateMachine(view, log, workers, notifier, verifyingVoteProcessorFactory), nil
	}
}

func NewStateMachine(
	view uint64,
	log zerolog.Logger,
	workers hotstuff.Workers,
	notifier hotstuff.Consumer,
	verifyingVoteProcessorFactory hotstuff.VoteProcessorFactory,
) *VoteCollector {
	log = log.With().
		Str("hotstuff", "VoteCollector").
		Uint64("view", view).
		Logger()
	sm := &VoteCollector{
		log:                      log,
		workers:                  workers,
		notifier:                 notifier,
		createVerifyingProcessor: verifyingVoteProcessorFactory,
		votesCache:               *NewViewSpecificVotesCache(view),
	}

	// without a block, we don't process votes (only cache them here in `votesCache`)
	sm.votesProcessor.Store(&atomicValueWrapper{
		processor: NewNoopCollector(hotstuff.VoteCollectorStatusCaching),
	})
	return sm
}

// AddVote adds a vote to the vote collector. When enough votes have been
// added to produce a QC, the QC will be created asynchronously, and passed
// to EventLoop through a callback.
// All expected errors are handled internally. Under normal execution only
// exceptions are propagated to caller.
//
// PREREQUISITE: The origin of the vote is cryptographically verified (via the
// sender's networking key). Otherwise, VoteCollector is vulnerable to
// impersonation attacks.
func (m *VoteCollector) AddVote(vote *model.Vote) error {
	// Cache vote
	err := m.votesCache.AddVote(vote) // only accepts votes for pre-configured view; errors otherwise
	if err != nil {
		if errors.Is(err, DuplicatedVoteErr) {
			return nil
		}
		if doubleVoteErr, isDoubleVoteErr := model.AsDoubleVoteError(err); isDoubleVoteErr {
			m.notifier.OnDoubleVotingDetected(doubleVoteErr.FirstVote, doubleVoteErr.ConflictingVote)
			return nil
		}
		if inconsistentVoteErr, isInconsistentVoteErr := model.AsInconsistentVoteError(err); isInconsistentVoteErr {
			m.notifier.OnInconsistentVotingDetected(inconsistentVoteErr.FirstVote, inconsistentVoteErr.InconsistentVote)
			return nil
		}
		return fmt.Errorf("internal error adding vote %v to cache for block %v: %w", vote.ID(), vote.BlockID, err)
	}

	// process vote
	err = m.processVote(vote)
	if err != nil {
		return fmt.Errorf("internal error processing vote %v for block %v: %w", vote.ID(), vote.BlockID, err)
	}
	return nil
}

// processVote uses compare-and-repeat pattern to process vote with underlying vote processor.
// All expected errors are handled internally. Under normal execution only exceptions are
// propagated to caller.
//
// PREREQUISITE: This method should only be called for votes that were successfully added to
// `votesCache` (identical duplicates are ok). Therefore, we don't have to deal here with
// equivocation (same replica voting for different blocks) or inconsistent votes (replica
// emitting votes with inconsistent signatures for the same block), because such votes were
// already filtered out by the cache.
func (m *VoteCollector) processVote(vote *model.Vote) error {
	for {
		processor := m.atomicLoadProcessor()
		currentState := processor.Status()
		err := processor.Process(vote)
		if err != nil {
			if errors.Is(err, DuplicatedVoteErr) {
				// This error is returned for repetitions of exactly the same vote. Such repetitions can occur (as race
				// condition) in our concurrent implementation. When the block proposal for the view arrives:
				//  (A1) `votesProcessor` transitions from `VoteCollectorStatusCaching` to `VoteCollectorStatusVerifying`
				//  (A2) the cached votes are fed into the VerifyingVoteProcessor
				// However, to increase concurrency, step (A1) and (A2) are _not_ atomically happening together.
				// Therefore, the following event (B) might happen _in between_:
				//  (B) A newly-arriving vote V is first cached and then processed by the VerifyingVoteProcessor.
				// In this scenario, vote V is already included in the VerifyingVoteProcessor. Nevertheless, step (A2)
				// will attempt to add V again to the VerifyingVoteProcessor, because the vote is in the cache.
				return nil
			}
			if model.IsInvalidVoteError(err) {
				// vote is invalid, which we only notice once we try to add it to the VerifyingVoteProcessor
				m.notifier.OnInvalidVoteDetected(vote)
				return nil
			}
			if errors.Is(err, VoteForIncompatibleBlockError) {
				// For honest nodes, there should be only a single proposal per view and all votes should
				// be for this proposal. However, byzantine nodes might deviate from this happy path:
				// * A malicious leader might create multiple (individually valid) conflicting proposals for the
				//   same view. Honest replicas will send correct votes for whatever proposal they see first.
				//   We only accept the first valid block and reject any other conflicting blocks that show up later.
				// * Alternatively, malicious replicas might send votes with the expected view, but for blocks that
				//   don't exist.
				// In either case, receiving votes for the same view but for different block IDs is a symptom
				// of malicious consensus participants. At this point, we can't attribute the failure to specific
				// nodes. Hence, we log a warning here:
				m.log.Warn().
					Err(err).
					Msg("encountered votes for different blocks within the same view, i.e. we detected byzantine consensus participant(s)")
				return nil
			}

			return err
		}

		if currentState != m.Status() {
			// Status of VoteProcessor was concurrently updated while we added the vote => repeat
			continue
		}
		return nil
	}
}

// Status returns the status of underlying vote processor
func (m *VoteCollector) Status() hotstuff.VoteCollectorStatus {
	return m.atomicLoadProcessor().Status()
}

// View returns view associated with this collector
func (m *VoteCollector) View() uint64 {
	return m.votesCache.View()
}

// ProcessBlock performs validation of block signature and processes block with respected collector.
// In case we have received double proposal, we will stop attempting to build a QC for this view,
// because we don't want to build on any proposal from an equivocating primary. Note: slashing challenges
// for proposal equivocation are triggered by hotstuff.Forks, so we don't have to do anything else here.
// Expected returns during normal operations:
//  * nil if the proposer's vote, included in the proposal, is valid.
//  * model.InvalidBlockError if the proposer's vote is invalid.
//  * model.DoubleVoteError if the proposer has voted for a block other than the given proposal.
//  * model.InconsistentVoteError if the proposer has emitted inconsistent votes for their proposal.
//    This sentinel error is only relevant for voting schemes, where the voting replica has different
//    options for signing (e.g. replica can sign with their staking key and/or their random beacon key).
//    For such voting schemes, byzantine replicas could try to submit different votes for the same block,
//    to exhaust the primary's resources or have multiple of their votes counted to undermine consensus
//    safety. Sending inconsistent votes belongs to the family of equivocation attacks.
// All other errors should be treated as exceptions.
//
// PREREQUISITES:
//  1. The origin of the proposal is cryptographically verified (via the sender's networking key).
//     Otherwise, VoteCollector is vulnerable to impersonation attacks.
//  2. Here, we only check the proposer's vote. All other aspects of the block proposal must already
//     be validated. Otherwise, VoteCollector is vulnerable to constructing a QC for an invalid block.
func (m *VoteCollector) ProcessBlock(proposal *model.Proposal) error {
	// IMPLEMENTATION: the internal state change is implemented as an atomic compare-and-swap,
	// i.e. the state transition is only executed if VoteCollector's internal state is
	// equal to `expectedValue`. The implementation only allows the transitions
	//         CachingVotes   -> VerifyingVotes
	//         CachingVotes   -> Invalid
	//         VerifyingVotes -> Invalid

	// Cache proposer's vote for their own block
	err := m.votesCache.AddVote(proposal.ProposerVote()) // only accepts votes for pre-configured view; errors otherwise
	if err != nil && !errors.Is(err, DuplicatedVoteErr) {
		if doubleVoteErr, isDoubleVoteErr := model.AsDoubleVoteError(err); isDoubleVoteErr {
			m.notifier.OnDoubleVotingDetected(doubleVoteErr.FirstVote, doubleVoteErr.ConflictingVote)
			return fmt.Errorf("rejecting proposal %v because proposer is equivocating: %w", proposal.Block.BlockID, err)
		}
		if inconsistentVoteErr, isInconsistentVoteErr := model.AsInconsistentVoteError(err); isInconsistentVoteErr {
			m.notifier.OnInconsistentVotingDetected(inconsistentVoteErr.FirstVote, inconsistentVoteErr.InconsistentVote)
			return fmt.Errorf("rejecting proposal %v because proposer published inconsistent voters: %w", proposal.Block.BlockID, err)
		}
		return fmt.Errorf("internal error caching proposer's vote for block %v: %w", proposal.Block.BlockID, err)
	}

	for {
		proc := m.atomicLoadProcessor()

		switch proc.Status() {
		// first valid block for this view: commence state transition from caching to verifying
		case hotstuff.VoteCollectorStatusCaching:
			err := m.caching2Verifying(proposal)
			if errors.Is(err, ErrDifferentCollectorState) {
				continue // concurrent state update by other thread => restart our logic
			}
			if model.IsInvalidBlockError(err) {
				return fmt.Errorf("cannot collect votes for invalid proposal: %w", err)
			}
			if err != nil {
				return fmt.Errorf("internal error updating VoteProcessor's status from %s to %s for block %v: %w",
					proc.Status().String(), hotstuff.VoteCollectorStatusVerifying.String(), proposal.Block.BlockID, err)
			}

			m.log.Info().
				Hex("block_id", proposal.Block.BlockID[:]).
				Msg("vote collector status changed from caching to verifying")

			m.processCachedVotes(proposal.Block)

		// We already received a valid block for this view. Check whether the proposer is
		// equivocating and terminate vote processing in this case. Note: proposal equivocation
		// is handled by hotstuff.Forks, so we don't have to do anything else here.
		case hotstuff.VoteCollectorStatusVerifying:
			verifyingProc, ok := proc.(hotstuff.VerifyingVoteProcessor)
			if !ok {
				return fmt.Errorf("while processing block %v, found that VoteProcessor reports status %s but has an incompatible implementation type %T",
					proposal.Block.BlockID, proc.Status(), verifyingProc)
			}
			if verifyingProc.Block().BlockID != proposal.Block.BlockID {
				m.terminateVoteProcessing()
			}

		// Vote processing for this view has already been terminated. Note: proposal equivocation
		// is handled by hotstuff.Forks, so we don't have anything to do here.
		case hotstuff.VoteCollectorStatusInvalid: /* no op */

		default:
			return fmt.Errorf("while processing block %v, found that VoteProcessor reported unknown status %s", proposal.Block.BlockID, proc.Status())
		}

		return nil
	}
}

// RegisterVoteConsumer registers a VoteConsumer. Upon registration, the collector
// feeds all cached votes into the consumer in the order they arrived.
// CAUTION, VoteConsumer implementations must be
//  * NON-BLOCKING and consume the votes without noteworthy delay, and
//  * CONCURRENCY SAFE
func (m *VoteCollector) RegisterVoteConsumer(consumer hotstuff.VoteConsumer) {
	m.votesCache.RegisterVoteConsumer(consumer)
}

// caching2Verifying ensures that the VoteProcessor is currently in state `VoteCollectorStatusCaching`
// and replaces it by a newly-created VerifyingVoteProcessor.
// Error returns:
//  * model.InvalidBlockError - proposal has invalid proposer vote
//  * ErrDifferentCollectorState if the VoteCollector's state is _not_ `CachingVotes`
//  * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (m *VoteCollector) caching2Verifying(proposal *model.Proposal) error {
	blockID := proposal.Block.BlockID
	newProc, err := m.createVerifyingProcessor.Create(m.log, proposal)
	if err != nil {
		return fmt.Errorf("failed to create VerifyingVoteProcessor for block %v: %w", blockID, err)
	}
	newProcWrapper := &atomicValueWrapper{processor: newProc}

	m.Lock()
	defer m.Unlock()
	proc := m.atomicLoadProcessor()
	if proc.Status() != hotstuff.VoteCollectorStatusCaching {
		return fmt.Errorf("processors's current state is %s: %w", proc.Status().String(), ErrDifferentCollectorState)
	}
	m.votesProcessor.Store(newProcWrapper)
	return nil
}

func (m *VoteCollector) terminateVoteProcessing() {
	if m.Status() == hotstuff.VoteCollectorStatusInvalid {
		return
	}
	newProcWrapper := &atomicValueWrapper{
		processor: NewNoopCollector(hotstuff.VoteCollectorStatusInvalid),
	}

	m.Lock()
	defer m.Unlock()
	m.votesProcessor.Store(newProcWrapper)
}

// processCachedVotes feeds all cached votes into the VoteProcessor
func (m *VoteCollector) processCachedVotes(block *model.Block) {
	for _, vote := range m.votesCache.All() {
		blockVote := vote
		voteProcessingTask := func() {
			err := m.processVote(blockVote)
			if err != nil {
				m.log.Fatal().Err(err).Msg("internal error processing cached vote")
			}
		}
		m.workers.Submit(voteProcessingTask)
	}
}
