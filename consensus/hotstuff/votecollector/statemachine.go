package votecollector

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

var (
	ErrInvalidCollectorStateTransition = errors.New("invalid state transition")
	ErrDifferentCollectorState         = errors.New("different state")
)

// VerifyingVoteProcessorFactory generates hotstuff.VerifyingVoteCollector instances
type VerifyingVoteProcessorFactory = func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error)

// StateMachine implements a state machine for transition between different states of vote collector
type StateMachine struct {
	sync.Mutex
	log                      zerolog.Logger
	workerPool               *workerpool.WorkerPool
	notifier                 hotstuff.Consumer
	createVerifyingProcessor VerifyingVoteProcessorFactory

	votesCache     VotesCache
	votesProcessor atomic.Value
}

func (m *StateMachine) atomicLoadProcessor() hotstuff.VoteProcessor {
	return m.votesProcessor.Load().(*atomicValueWrapper).processor
}

// atomic.Value doesn't allow storing interfaces as atomic values,
// it requires that stored type is always the same so we need a wrapper that will mitigate this restriction
// https://github.com/golang/go/issues/22550
type atomicValueWrapper struct {
	processor hotstuff.VoteProcessor
}

func NewStateMachine(
	view uint64,
	log zerolog.Logger,
	workerPool *workerpool.WorkerPool,
	notifier hotstuff.Consumer,
	verifyingCollectorFactory VerifyingVoteProcessorFactory,
) *StateMachine {
	log = log.With().
		Str("hotstuff", "VoteCollector").
		Uint64("view", view).
		Logger()
	sm := &StateMachine{
		log:                      log,
		workerPool:               workerPool,
		notifier:                 notifier,
		createVerifyingProcessor: verifyingCollectorFactory,
		votesCache:               *NewVotesCache(view),
	}

	// without a block, we don't process votes (only cache them)
	sm.votesProcessor.Store(&atomicValueWrapper{
		processor: NewNoopCollector(hotstuff.VoteCollectorStatusCaching),
	})
	return sm
}

// CreateVote implements BlockSigner interface, if underlying collector implements BlockSigner interface then we will
// delegate function call, otherwise we will return an error indicating wrong collector state.
// ATTENTION: this might be changed if CreateVote and state transitions will be called in parallel
// something like compare-and-repeat might need to be implemented.
func (m *StateMachine) CreateVote(block *model.Block) (*model.Vote, error) {
	processor := m.atomicLoadProcessor()
	blockSigner, ok := processor.(hotstuff.BlockSigner)
	if ok {
		return blockSigner.CreateVote(block)
	}
	return nil, ErrDifferentCollectorState
}

func (m *StateMachine) AddVote(vote *model.Vote) error {
	// Cache vote
	err := m.votesCache.AddVote(vote)
	if err != nil {
		if errors.Is(err, RepeatedVoteErr) {
			return nil
		}
		if model.IsDoubleVoteError(err) {
			panic("call m.notifier.OnDoubleVotingDetected( .. )")
		}
		return fmt.Errorf("internal error adding vote to cache")
	}

	err = m.processVote(vote)
	if err != nil {
		if errors.Is(err, VoteForIncompatibleBlockError) {
			return nil
		}
		fmt.Errorf("internal error processing vote: %w", err)
	}
	return nil
}

func (m *StateMachine) processVote(vote *model.Vote) error {
	for {
		processor := m.atomicLoadProcessor()
		currentState := processor.Status()
		err := processor.Process(vote)
		if err != nil {
			if model.IsInvalidVoteError(err) {
				panic("call m.notifier.OnInvalidVoteDetected( .. )")
			}
			return err
		}

		if currentState != m.Status() {
			continue
		}
		return nil
	}
}

func (m *StateMachine) Status() hotstuff.VoteCollectorStatus {
	return m.atomicLoadProcessor().Status()
}

// ProcessBlock performs validation of block signature and processes block with respected collector.
// In case we have received double proposal, we will stop attempting to build a QC for this view,
// because we don't want to build on any proposal from an equivocating primary. Note: slashing challenges
// for proposal equivocation are triggered by hotstuff.Forks, so we don't have to do anything else here.
//
// The internal state change is implemented as an atomic compare-and-swap, i.e.
// the state transition is only executed if VoteCollector's internal state is
// equal to `expectedValue`. The implementation only allows the transitions
//         CachingVotes   -> VerifyingVotes
//         CachingVotes   -> Invalid
//         VerifyingVotes -> Invalid
func (m *StateMachine) ProcessBlock(proposal *model.Proposal) error {
	for {
		proc := m.atomicLoadProcessor()

		switch proc.Status() {
		// first valid block for this view: commence state transition from caching to verifying
		case hotstuff.VoteCollectorStatusCaching:
			// TODO: implement logic for validating block proposal, converting it to vote and further processing

			err := m.caching2Verifying(proposal.Block)
			if errors.Is(err, ErrDifferentCollectorState) {
				continue // concurrent state update by other thread => restart our logic
			}
			if err != nil {
				return fmt.Errorf("internal error updating VoteProcessor's status from %s to %s",
					proc.Status().String(), hotstuff.VoteCollectorStatusVerifying.String())
			}
			m.processCachedVotes(proposal.Block)

		// We already received a valid block for this view. Check whether the proposer is
		// equivocating and terminate vote processing in this case. Note: proposal equivocation
		// is handled by hotstuff.Forks, so we don't have to do anything else here.
		case hotstuff.VoteCollectorStatusVerifying:
			verifyingProc, ok := proc.(hotstuff.VerifyingVoteProcessor)
			if !ok {
				return fmt.Errorf("VoteProcessor has status %s but it has an incompatible implementation type %T",
					proc.Status(), verifyingProc)
			}
			if verifyingProc.Block().BlockID != proposal.Block.BlockID {
				m.terminateVoteProcessing()
			}

		// Vote processing for this view has already been terminated. Note: proposal equivocation
		// is handled by hotstuff.Forks, so we don't have anything to do here.
		case hotstuff.VoteCollectorStatusInvalid: /* no op */

		default:
			return fmt.Errorf("VoteProcessor reported unknown status %s", proc.Status())
		}

		return nil
	}
}

// caching2Verifying ensures that the VoteProcessor is currently in state `VoteCollectorStatusCaching`
// and replaces it by a newly-created VerifyingVoteProcessor.
// Error returns:
// * ErrDifferentCollectorState if the VoteCollector's state is _not_ `CachingVotes`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (m *StateMachine) caching2Verifying(block *model.Block) error {
	log := m.log.With().Hex("BlockID", block.BlockID[:]).Logger()
	newProc, err := m.createVerifyingProcessor(log, block)
	if err != nil {
		return fmt.Errorf("failed to create VerifyingVoteProcessor for block %v", block.BlockID)
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

func (m *StateMachine) terminateVoteProcessing() {
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
func (m *StateMachine) processCachedVotes(block *model.Block) {
	for _, vote := range m.votesCache.All() {
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
		m.workerPool.Submit(voteProcessingTask)
	}
}
