package votecollector

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/module/irrecoverable"
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

// VerifyingVoteProcessorFactory generates hotstuff.VerifyingVoteCollector instances
type VerifyingVoteProcessorFactory = func(log zerolog.Logger, proposal *model.SignedProposal) (hotstuff.VerifyingVoteProcessor, error)

// VoteCollector implements a state machine for transition between different states of vote collector
type VoteCollector struct {
	sync.Mutex
	log                      zerolog.Logger
	workers                  hotstuff.Workers
	notifier                 hotstuff.VoteAggregationConsumer
	createVerifyingProcessor VerifyingVoteProcessorFactory

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

// AddVote adds a vote to current vote collector
// All expected errors are handled via callbacks to notifier.
// No errors are expected during normal operation.
func (m *VoteCollector) AddVote(vote *model.Vote) error {
	// Cache vote
	unique, err := m.ensureVoteUnique(vote)
	if err != nil {
		return err
	}
	if !unique {
		return nil
	}

	err = m.processVote(vote)
	if err != nil {
		return fmt.Errorf("internal error processing vote %v for block %v: %w",
			vote.ID(), vote.BlockID, err)
	}
	return nil
}

// ensureVoteUnique caches the vote in the votesCache. Additionally, it's responsible for reporting byzantine behavior when
// byzantine leader or replica has provided an equivocating vote. All votes that are different from the original one(by same signer)
// are reported as equivocation attempt.
// ATTENTION: In order to guarantee that all equivocation attempts will be caught this function needs to be called before
// processing individual votes and block proposals.
// Possible return values:
//   - (true, nil) - provided vote was first from given signer ID
//   - (false, nil) - there is another vote in the cache that was previously added by given signer ID
//   - (false, error) - exception during processing.
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
					Err(err).
					Msg("received vote for incompatible block")

				return nil
			}
			return irrecoverable.NewException(err)
		}

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

// ProcessBlock performs validation of block signature and processes block with respected collector.
// In case we have received double proposal, we will stop attempting to build a QC for this view,
// because we don't want to build on any proposal from an equivocating primary. Note: slashing challenges
// for proposal equivocation are triggered by hotstuff.Forks, so we don't have to do anything else here.
//
// The internal state change is implemented as an atomic compare-and-swap, i.e.
// the state transition is only executed if VoteCollector's internal state is
// equal to `expectedValue`. The implementation only allows the transitions
//
//	CachingVotes   -> VerifyingVotes
//	CachingVotes   -> Invalid
//	VerifyingVotes -> Invalid
func (m *VoteCollector) ProcessBlock(proposal *model.SignedProposal) error {
	proposerVote, err := proposal.ProposerVote()
	if err != nil {
		return model.NewInvalidProposalErrorf(proposal, "invalid proposer vote")
	}
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

// caching2Verifying ensures that the VoteProcessor is currently in state `VoteCollectorStatusCaching`
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
