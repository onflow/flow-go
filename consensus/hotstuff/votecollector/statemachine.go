package votecollector

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

var (
	ErrInvalidCollectorStateTransition = errors.New("invalid state transition")
	ErrDifferentCollectorState         = errors.New("different state")
)

// NewVerifyingCollectorFactoryMethod is a factory method to generate a hotstuff.VoteCollectorState
type NewVerifyingCollectorFactoryMethod = func(base CollectionBase, block *model.Block) (hotstuff.VerifyingVoteCollector, error)

// StateMachine implements a state machine for transition between different states of vote collector
type StateMachine struct {
	CollectionBase

	sync.Mutex
	collector                atomic.Value
	createVerifyingCollector NewVerifyingCollectorFactoryMethod
}

func (m *StateMachine) atomicLoadCollector() hotstuff.VoteCollectorState {
	return m.collector.Load().(*atomicValueWrapper).collector
}

// atomic.Value doesn't allow storing interfaces as atomic values,
// it requires that stored type is always the same so we need a wrapper that will mitigate this restriction
// https://github.com/golang/go/issues/22550
type atomicValueWrapper struct {
	collector hotstuff.VoteCollectorState
}

func NewStateMachine(base CollectionBase) *StateMachine {
	sm := &StateMachine{
		CollectionBase: base,
	}

	// by default start with caching collector
	sm.collector.Store(&atomicValueWrapper{
		collector: NewCachingVoteCollector(base),
	})
	return sm
}

// CreateVote implements BlockSigner interface, if underlying collector implements BlockSigner interface then we will
// delegate function call, otherwise we will return an error indicating wrong collector state.
// ATTENTION: this might be changed if CreateVote and state transitions will be called in parallel
// something like compare-and-repeat might need to be implemented.
func (m *StateMachine) CreateVote(block *model.Block) (*model.Vote, error) {
	collector := m.atomicLoadCollector()
	blockSigner, ok := collector.(hotstuff.BlockSigner)
	if ok {
		return blockSigner.CreateVote(block)
	}
	return nil, ErrDifferentCollectorState
}

func (m *StateMachine) AddVote(vote *model.Vote) error {
	for {
		collector := m.atomicLoadCollector()
		currentState := collector.Status()
		err := collector.AddVote(vote)
		if err != nil {
			return fmt.Errorf("could not add vote %v: %w", vote.ID(), err)
		}
		if currentState != m.Status() {
			continue
		}

		return nil
	}
}

func (m *StateMachine) Status() hotstuff.VoteCollectorStatus {
	return m.atomicLoadCollector().Status()
}

// ChangeProcessingStatus changes the VoteCollector's internal processing
// status. The operation is implemented as an atomic compare-and-swap, i.e. the
// state transition is only executed if VoteCollector's internal state is
// equal to `expectedValue`. The return indicates whether the state was updated.
// The implementation only allows the transitions
//         CachingVotes   -> VerifyingVotes
//         CachingVotes   -> Invalid
//         VerifyingVotes -> Invalid
func (m *StateMachine) ChangeProcessingStatus(currentStatus, newStatus hotstuff.VoteCollectorStatus, payload interface{}) error {
	// don't transition between same states
	if currentStatus == newStatus {
		return nil
	}

	if (currentStatus == hotstuff.VoteCollectorStatusCaching) && (newStatus == hotstuff.VoteCollectorStatusVerifying) {
		proposal, ok := payload.(*model.Proposal)
		if !ok {
			return fmt.Errorf("invalid payload %v, expected *model.Block", payload)
		}

		cachingCollector, err := m.caching2Verifying(proposal.Block)
		if err != nil {
			return fmt.Errorf("failed to transistion VoteCollector from %s to %s: %w", currentStatus.String(), newStatus.String(), err)
		}

		blockID := proposal.Block.BlockID
		m.workerPool.Submit(func() {
			for _, vote := range cachingCollector.GetVotesByBlockID(blockID) {
				task := m.reIngestVoteTask(vote)
				m.workerPool.Submit(task)
			}
		})

		return nil
	}

	// TODO: handle state transition from caching to invalid

	return fmt.Errorf("cannot transition from %s to %s: %w", currentStatus.String(), newStatus.String(), ErrInvalidCollectorStateTransition)
}

// ProcessBlock performs validation of block signature and processes block with respected collector.
func (m *StateMachine) ProcessBlock(proposal *model.Proposal) error {
	// TODO: implement logic for validating block proposal, converting it to vote and further processing

	// TODO: Since we don't tolerate double votes we can't process double proposals.
	//  In case we have received double proposal and our collector is already in verifying state we will just
	//  turn collector into invalid state and stop processing of any votes until pacemaker kicks in, terminates current view with timeout
	//  and next leader produces block.

	block := proposal.Block
	err := m.ChangeProcessingStatus(m.Status(), hotstuff.VoteCollectorStatusVerifying, proposal)
	if err != nil {
		// if collector wasn't in caching state it means another goroutine has triggered state transition and
		// submitted proposal first. This means we have received double proposal, and we should stop collecting votes for
		// current view.
		if !errors.Is(err, ErrDifferentCollectorState) {
			return fmt.Errorf("could not change processing status for block %x: %w", block.BlockID, err)
		}
	}

	clr := m.atomicLoadCollector()

	// if we are dealing with verifying collector check if it's the same proposal
	if verifyingCollector, ok := clr.(hotstuff.VerifyingVoteCollector); ok {
		if verifyingCollector.Block().BlockID == block.BlockID {
			return nil
		}
	}

	// if our collector is in non verifying state, or we have received proposal for different block
	// we need to transition to invalid state and stop processing votes.
	err = m.ChangeProcessingStatus(m.Status(), hotstuff.VoteCollectorStatusInvalid, struct{}{})
	if err != nil {
		return fmt.Errorf("could not change processing status to invalid for block (%x): %w", block.BlockID, err)
	}

	return nil
}

// caching2Verifying ensures that the collector is currently in state `CachingVotes`
// and replaces it by a newly-created ConsensusClusterVoteCollector.
// Returns:
// * CachingVoteCollector as of before the update
// * ErrDifferentCollectorState if the VoteCollector's state is _not_ `CachingVotes`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (m *StateMachine) caching2Verifying(block *model.Block) (*CachingVoteCollector, error) {
	m.Lock()
	defer m.Unlock()
	clr := m.atomicLoadCollector()
	cachingCollector, ok := clr.(*CachingVoteCollector)
	if !ok {
		return nil, fmt.Errorf("collector's current state is %s: %w", clr.Status().String(), ErrDifferentCollectorState)
	}

	verifyingCollector, err := m.createVerifyingCollector(m.CollectionBase, block)
	if err != nil {
		return nil, fmt.Errorf("could not create verifying vote collector")
	}

	m.collector.Store(&atomicValueWrapper{collector: verifyingCollector})

	return cachingCollector, nil
}

// reIngestIncorporatedResultTask returns a functor for re-ingesting the specified
// IncorporatedResults; functor handles all potential business logic errors.
func (m *StateMachine) reIngestVoteTask(vote *model.Vote) func() {
	task := func() {
		panic("implement me")
	}
	return task
}
