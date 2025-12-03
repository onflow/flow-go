package votecollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/logging"
)

var (
	ErrDifferentCollectorState = errors.New("different state")
)

// VerifyingVoteProcessorFactory generates [hotstuff.VerifyingVoteProcessor] instances
type VerifyingVoteProcessorFactory = func(log zerolog.Logger, proposal *model.SignedProposal) (hotstuff.VerifyingVoteProcessor, error)

// VoteCollector implements a state machine for transition between different states of vote processor.
// It ingests *all* votes for a specified view and consolidates the handling of all byzantine edge cases
// related to voting (including byzantine leaders publishing conflicting proposals and/or votes).
// On the happy path, the VoteCollector generates a QC when enough votes have been ingested.
// We internally delegate the vote-format specific processing to the VoteProcessor.
type VoteCollector struct {
	log                      zerolog.Logger
	workers                  hotstuff.Workers
	notifier                 hotstuff.VoteAggregationConsumer
	createVerifyingProcessor VerifyingVoteProcessorFactory

	// Byzantine nodes might mount the following attacks on the vote-processing logic:
	//  1. The leader might send block proposal and equivocate by sending a different conflicting vote as an independent message.
	//  2. The leader might send a block proposal and (repeatedly) send the same vote again as an independent message.
	//  3. Any byzantine replica might send multiple individual vote messages (repeated identical votes, or equivocating with different votes).
	// These are resource exhaustion attacks (if frequent), but can also be attempts by byzantine nodes to have their votes repeatedly
	// counted (producing an invalid QC if repeatedly counted and thereby disrupting the certification and finalization process).
	//
	// Replicas that are not the leader might also attempt to submit proposals, but these are already caught earlier by the compliance layer.
	// Hence, attacks 1. and 2. are only available to the leader, because these attacks specifically utilize the fact that the leader signs
	// their proposal, with the signature authenticating the proposal as well as serving as the leader's vote. Attack 3. can be mounted by
	// replicas and the leader alike.
	//
	// ATTENTION: Detecting vote equivocation is a collaborative effort of the VoteCollector's `votesCache` and the `VoteProcessor`.
	//  * Stand-alone votes always hit the `votesCache`, which checks the vote against all previously cached votes for
	//    equivocation (protocol violation) or exact duplicates (non-slashable, potential spamming). This mitigates attack 3.
	//  * In the current implementation, the `votesCache` does not catch leader attacks 1. and 2., which exploit the fact that stand-alone
	//    votes and votes embedded in proposals are processed concurrently through different code paths. We emphasize that attack vectors
	//    1. and 2. are only available to a byzantine leader as long as the leader has not (yet) equivocated for the current view. Once
	//    the VoteCollector notices that the _leader_ equivocates, it immediately stops accepting proposals for the view, thereby closing
	//    attack vectors 1. and 2. Hence, it is sufficient for the [VerifyingVoteProcessor] to catch _all_ equivocation attacks,
	//    including attacks 1. and 2.
	//
	// The VoteProcessor provides the final defense against any double-counting attacks, because this attack vector is only open as long
	// as we are ingesting votes with the goal of producing a QC, in other words as along as we are still following the happy path.
	votesCache     VotesCache
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
	notifier hotstuff.VoteAggregationConsumer,
	verifyingVoteProcessorFactory VerifyingVoteProcessorFactory,
) voteaggregator.NewCollectorFactoryMethod {
	return func(view uint64, workers hotstuff.Workers) (hotstuff.VoteCollector, error) {
		return NewStateMachine(view, log, workers, notifier, verifyingVoteProcessorFactory), nil
	}
}

func NewStateMachine(
	view uint64,
	log zerolog.Logger,
	workers hotstuff.Workers,
	notifier hotstuff.VoteAggregationConsumer,
	verifyingVoteProcessorFactory VerifyingVoteProcessorFactory,
) *VoteCollector {
	log = log.With().
		Str("component", "hotstuff.vote_collector").
		Uint64("view", view).
		Logger()
	sm := &VoteCollector{
		log:                      log,
		workers:                  workers,
		notifier:                 notifier,
		createVerifyingProcessor: verifyingVoteProcessorFactory,
		votesCache:               *NewVotesCache(view),
	}

	// without a block, we don't process votes (only cache them)
	sm.votesProcessor.Store(&atomicValueWrapper{
		processor: NewNoopCollector(hotstuff.VoteCollectorStatusCaching),
	})
	return sm
}

// AddVote adds a vote to the vote collector. The vote must be for the `VoteCollector`'s view (otherwise,
// an exception is returned). When enough votes have been added to produce a QC, the QC will be created
// asynchronously, and passed to EventLoop through a callback.
// All byzantine edge cases are handled internally via callbacks to notifier.
// Under normal execution, only exceptions are propagated to caller.
func (m *VoteCollector) AddVote(vote *model.Vote) error {
	// Cache vote; only the first vote from any specific signer will pass this step
	unique, err := m.ensureVoteUnique(vote)
	if err != nil {
		return err
	}
	if !unique {
		return nil
	}

	err = m.processVote(vote) // handles all byzantine edge cases internally
	if err != nil {
		return fmt.Errorf("internal error processing vote %v for block %v: %w",
			vote.ID(), vote.BlockID, err)
	}
	return nil
}

// ensureVoteUnique caches the vote in the votesCache (or rejects it) - implemented as a concurrency safe, atomic operation.
// This function is responsible for reporting byzantine behavior when byzantine leader or replica has sent an equivocating vote.
// All votes that are different from the original one(by same signer) are reported as equivocation attempts.
//
// ATTENTION: In order to guarantee that all equivocation attempts will be caught, this function needs to be
// called consistently before processing individual votes _and_ block proposals.
//
// Possible return values:
//   - (true, nil) if vote is first from given signer ID
//   - (false, nil) if there is another vote in the cache that was previously added by given signer ID
//   - (false, error) if exception during processing.
//
// No errors are expected during normal operations.
func (m *VoteCollector) ensureVoteUnique(vote *model.Vote) (bool, error) {
	err := m.votesCache.AddVote(vote)
	if err != nil {
		if errors.Is(err, RepeatedVoteErr) {
			return false, nil
		}
		if doubleVoteErr, isDoubleVoteErr := model.AsDoubleVoteError(err); isDoubleVoteErr {
			m.notifier.OnDoubleVotingDetected(doubleVoteErr.FirstVote, doubleVoteErr.ConflictingVote)
			return false, nil
		}
		return false, fmt.Errorf("internal error adding vote %v to cache for block %v: %w",
			vote.ID(), vote.BlockID, err)
	}
	return true, nil
}

// processVote uses compare-and-repeat pattern to process vote with underlying vote processor.
// This compare-and-repeat pattern is crucial to ensure all valid votes make it to the
// [hotstuff.VerifyingVoteProcessor], i.e. liveness, despite the vote processor possibly being
// swapped concurrently when the block is arriving (see implementation for detailed reasoning).
//
// PREREQUISITE: This method should only be called for votes that were successfully added to
// `votesCache` (identical duplicates are ok). Therefore, we don't have to deal here with
// equivocation (same replica voting for different blocks) or inconsistent votes (replica
// emitting votes with inconsistent signatures for the same block), because such votes were
// already filtered out by the cache.
//
// All byzantine edge cases are handled internally via callbacks to notifier. Under normal
// execution, only exceptions are propagated to caller.
func (m *VoteCollector) processVote(vote *model.Vote) error {
	for {
		processor := m.atomicLoadProcessor()
		currentState := processor.Status()
		err := processor.Process(vote)
		if err != nil {
			if invalidVoteErr, ok := model.AsInvalidVoteError(err); ok {
				// vote is invalid, which we only notice once we try to add it to the [VerifyingVoteProcessor]
				m.notifier.OnInvalidVoteDetected(*invalidVoteErr)
				return nil
			}
			if model.IsDuplicatedSignerError(err) {
				// This error is returned for repetitions of exactly the same vote. Such repetitions can occur (as race
				// condition) in our concurrent implementation. When the block proposal for the view arrives:
				//  (A1) `votesProcessor` transitions from [VoteCollectorStatusCaching] to [VoteCollectorStatusVerifying]
				//  (A2) the cached votes are fed into the [VerifyingVoteProcessor]
				// However, to increase concurrency, step (A1) and (A2) are _not_ atomically happening together.
				// Therefore, the following event (B) might happen _in between_:
				//  (B) A newly-arriving vote V is first cached and then processed by the [VerifyingVoteProcessor].
				// In this scenario, vote V is already included in the [VerifyingVoteProcessor]. Nevertheless, step (A2)
				// will attempt to add V again to the [VerifyingVoteProcessor], because the vote is in the cache.
				m.log.Debug().Msgf("duplicated signer %x", vote.SignerID)
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
				// of malicious consensus participants.  Hence, we log it here as a warning:
				m.log.Warn().
					Str(logging.KeySuspicious, "true").
					Err(err).
					Msg("received vote for incompatible block")

				return nil
			}
			return irrecoverable.NewException(err)
		}

		// ATTENTION: repeating the processing if the processor changed concurrently is REQUIRED for LIVENESS on the
		// happy path (honest proposer and a supermajority of votes arriving in time).
		// Liveness Proof for the happy path (utilizing the Go Memory Model https://go.dev/ref/mem, 'happens before' relation):
		//  * We only care about the Vote Processor's state transition `CachingVotes` → `VerifyingVotes`, because all other state transitions
		//    are leaving the happy path. The state transition is effectively an atomic write to the variable `votesProcessor` (see method
		//    `VoteCollector.ProcessBlock` below for details).
		//  * Case (a): `currentState` = [VoteCollectorStatusVerifying] = `m.Status()`
		//    Then, the `vote` is added directly to the [VerifyingVoteProcessor]. Informally, the state transition has happened before we retrieved
		//    the Vote Processor via `m.atomicLoadProcessor()` above.
		//  * Case (b): `currentState` = [VoteCollectorStatusCaching] = `m.Status()`
		//    We note the following 'happens before' relations (i) and (ii), which are guaranteed by sequential execution _within_ a goroutine:
		//    In goroutine A1 performing the state transition, (i) we write to `votesProcessor` before we read all cached votes from `votesCache`
		//    (acquiring `votesCache`s lock). In goroutine A2, (ii) we have added the vote to the `votesCache` (also acquiring `votesCache`s lock)
		//    before we read `votesProcessor`s status below and confirm its status still being [VoteCollectorStatusCaching].
		//       We prove by contradiction that it is impossible for thread A1 to read the cached votes before thread A2 adds its vote to `votesCache`
		//    (if that was possible, the verifying vote processor would not see the vote). By the time A1 reads the `votesCache`, it has already
		//    completed updating the status of the `votesProcessor` to [VoteCollectorStatusVerifying] (per (i)). We assumed that A1 reading the
		//    `votesCache` happens before A2 writing to it (`votesCache` uses locks, which establish a happens before relation). With (ii), we infer
		//    that the `votesProcessor` reaching status [VoteCollectorStatusVerifying] happens before A2 reading the `votesProcessor` status below.
		//    Therefore, A2 would read the status [VoteCollectorStatusVerifying], which contradicts our assumption.
		//       Hence, we conclude that thread A2 always caches its vote before A1 reads the `votesCache`. As A1 is the thread that performs the
		//    state transition, it will include A1's vote when feeding the cached votes into the [VerifyingVoteProcessor].
		//    Informally, the state transition will happen _after_ we cached the vote.
		//  * Case (c): `currentState` = [VoteCollectorStatusCaching] while `m.Status()` = [VoteCollectorStatusVerifying].
		//    In this scenario, the vote is being fed into the [NoopProcessor] first, before we realize that the state has changed. However,
		//    since the status has changed, the check below will trigger a repeat of the processing, which will then enter case (a)
		//    (or leave the happy path by transitioning to [VoteCollectorStatusInvalid], implying we are dealing with a byzantine proposer, in which
		//    case we may drop all votes anyway).
		// We have shown that all votes will reach the [VerifyingVoteProcessor] on the happy path.
		//
		// CAUTION: In the proof, we utilized that reading the `votesCache` happens before writing to it (case b). It is important to emphasize that
		// only locks are agnostic to the performed operation being a read or a write. In contrast, atomic variables only establish a 'happens before'
		// relation when a preceding write is observed by a subsequent read (consult go memory model https://go.dev/ref/mem, specifically the
		//'synchronized before', and its generalized "happens before" relation). However, in our case, we first read and then write - an order of
		// operations which does not induce any synchronization guarantees according to Go Memory Model. Hence, the `votesCache` utilizing locks is
		// critical for the correctness of the `VoteCollector`.
		if currentState != m.Status() {
			continue
		}

		m.notifier.OnVoteProcessed(vote)
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

// ProcessBlock validates the block signature, and adds it as the proposer's vote.
// In case we have received double proposal, we will stop attempting to build a QC for this view,
// because we don't want to build on any proposal from an equivocating primary. Note: slashing challenges
// for proposal equivocation are triggered by hotstuff.Forks, so we don't have to do anything else here.
//
// The internal state change is implemented as an atomic compare-and-swap, i.e.
// the state transition is only executed if VoteCollector's internal state is
// equal to `expectedValue`. The implementation only allows the transitions
//
//	CachingVotes   → VerifyingVotes
//	CachingVotes   → Invalid
//	VerifyingVotes → Invalid
//
// No errors are expected during normal operation (Byzantine edge cases handled via notifications internally).
func (m *VoteCollector) ProcessBlock(proposal *model.SignedProposal) error {
	proposerVote, err := proposal.ProposerVote()
	if err != nil {
		return model.NewInvalidProposalErrorf(proposal, "invalid proposer vote")
	}
	// We only abort here in case of an exception. No matter whether the vote is the first from the voter, an exact
	// duplicate or an equivocation, we still proceed as long as the vote is individually valid. This is fine, because
	// the VoteProcessor is robust against all byzantine edge cases. The VoteCollector's responsibility is to detect
	// equivocation attempts and report them via the notifier, which is done inside `ensureVoteUnique`.
	// We could additionally abort the vote collection here in case of equivocation, but this is not necessary for
	// safety, as long as the offending proposer is eventually slashed, which only requires notifying about the equivocation.
	_, err = m.ensureVoteUnique(proposerVote)
	if err != nil {
		return err
	}

	if proposal.Block.View != m.View() {
		return fmt.Errorf("this VoteCollector requires a proposal for view %d but received block %v with view %d",
			m.votesCache.View(), proposal.Block.BlockID, proposal.Block.View)
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
//   - NON-BLOCKING and consume the votes without noteworthy delay, and
//   - CONCURRENCY SAFE
func (m *VoteCollector) RegisterVoteConsumer(consumer hotstuff.VoteConsumer) {
	m.votesCache.RegisterVoteConsumer(consumer)
}

// caching2Verifying ensures that the VoteProcessor is currently in state [VoteCollectorStatusCaching]
// and replaces it by a newly-created VerifyingVoteProcessor.
// Error returns:
// * ErrDifferentCollectorState if the VoteCollector's state is _not_ `CachingVotes`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (m *VoteCollector) caching2Verifying(proposal *model.SignedProposal) error {
	blockID := proposal.Block.BlockID
	newProc, err := m.createVerifyingProcessor(m.log, proposal)
	if err != nil {
		return fmt.Errorf("failed to create VerifyingVoteProcessor for block %v: %w", blockID, err)
	}
	newProcWrapper := &atomicValueWrapper{processor: newProc}

	// We now have an optimistically-constructed `newProcWrapper` that represents the desired new state (happy path). We must ensure
	// that writing the `newProcWrapper` to `m.votesProcessor` happens ATOMICALLY if and only if the current state is `CachingVotes`.
	// The "Compare-And-Swap Loop" (CAS LOOP) is an efficient pattern to implement this logic:
	//    (i) We first retrieve the current state and check whether it is `CachingVotes`.
	//   (ii) If so, we attempt to compare-and-swap the current with the new state.
	// Note that (i) and (ii) are separate operations. However, the CAS in (ii) ensures that the write only happens if the current state
	// is still the same as what we observed in (i). If another thread changed the state in between (i) and (ii), we have worked with
	// an outdated view of the current state, and should repeat the attempt to update the state (hence the "loop" in CAS LOOP).
	//
	// On our specific application here, putting (i) and (ii) in a loop is not necessary for the following reason: The state transition
	// to `VoteCollectorStatusVerifying` is possible only if the current state is `VoteCollectorStatusCaching`. Once the state changed
	// away from `VoteCollectorStatusCaching` it can never return to this state. I.e. if condition (i) failed once, it can never be
	// satisfied later. Step (ii) failing implies that condition (i) was previously true, but no longer holds.
	currentProcWrapper := m.votesProcessor.Load().(*atomicValueWrapper)
	currentState := currentProcWrapper.processor.Status() // must use same object here as in CAS below (_not_ a fresh load from `m.Status()`)
	if currentState != hotstuff.VoteCollectorStatusCaching {
		return fmt.Errorf("processors's current state is %s: %w", currentState.String(), ErrDifferentCollectorState)
	}
	stateUpdateSuccessful := m.votesProcessor.CompareAndSwap(currentProcWrapper, newProcWrapper)
	if !stateUpdateSuccessful {
		return fmt.Errorf("CAS failed in between, processors's current state is %s: %w", m.Status(), ErrDifferentCollectorState)
	}

	return nil
}

// terminateVoteProcessing terminates vote processing by moving the processor into VoteCollectorStatusInvalid state.
// It utilizes atomic CAS(Compare-And-Swap) operation in a loop to ensure that eventually we enter invalid state.
func (m *VoteCollector) terminateVoteProcessing() {
	newProcWrapper := &atomicValueWrapper{
		processor: NewNoopCollector(hotstuff.VoteCollectorStatusInvalid),
	}

	for {
		currentState := m.Status()
		if currentState == hotstuff.VoteCollectorStatusInvalid {
			return
		}
		m.votesProcessor.Store(newProcWrapper)
	}
}

// processCachedVotes feeds all cached votes into the VoteProcessor
func (m *VoteCollector) processCachedVotes(block *model.Block) {
	cachedVotes := m.votesCache.All()
	m.log.Info().Msgf("processing %d cached votes", len(cachedVotes))
	for _, vote := range cachedVotes {
		if vote.BlockID != block.BlockID {
			continue
		}

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
