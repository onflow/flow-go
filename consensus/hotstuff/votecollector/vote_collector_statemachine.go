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
type NewVerifyingCollectorFactoryMethod = func(base BaseVoteCollector) (hotstuff.VoteCollectorState, error)

// VoteCollectorStateMachine implements a state machine for transition between different states of vote collector
type VoteCollectorStateMachine struct {
	BaseVoteCollector

	sync.Mutex
	collector                atomic.Value
	createVerifyingCollector NewVerifyingCollectorFactoryMethod
}

func (csm *VoteCollectorStateMachine) atomicLoadCollector() hotstuff.VoteCollectorState {
	return csm.collector.Load().(*atomicValueWrapper).collector
}

// atomic.Value doesn't allow storing interfaces as atomic values,
// it requires that stored type is always the same so we need a wrapper that will mitigate this restriction
// https://github.com/golang/go/issues/22550
type atomicValueWrapper struct {
	collector hotstuff.VoteCollectorState
}

func NewVoteCollectorStateMachine(base BaseVoteCollector) *VoteCollectorStateMachine {
	sm := &VoteCollectorStateMachine{
		BaseVoteCollector: base,
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
func (csm *VoteCollectorStateMachine) CreateVote(block *model.Block) (*model.Vote, error) {
	collector := csm.atomicLoadCollector()
	blockSigner, ok := collector.(hotstuff.BlockSigner)
	if ok {
		return blockSigner.CreateVote(block)
	}
	return nil, ErrDifferentCollectorState
}

func (csm *VoteCollectorStateMachine) AddVote(vote *model.Vote) error {
	for {
		collector := csm.atomicLoadCollector()
		currentState := collector.Status()
		err := collector.AddVote(vote)
		if err != nil {
			return fmt.Errorf("could not add vote %v: %w", vote.ID(), err)
		}
		if currentState != csm.Status() {
			continue
		}

		return nil
	}
}

func (csm *VoteCollectorStateMachine) Status() hotstuff.VoteCollectorStatus {
	return csm.atomicLoadCollector().Status()
}

func (csm *VoteCollectorStateMachine) ChangeProcessingStatus(expectedCurrentStatus, newStatus hotstuff.VoteCollectorStatus) error {
	// don't transition between same states
	if expectedCurrentStatus == newStatus {
		return nil
	}

	if (expectedCurrentStatus == hotstuff.VoteCollectorStatusCaching) && (newStatus == hotstuff.VoteCollectorStatusVerifying) {
		cachingCollector, err := csm.caching2Verifying()
		if err != nil {
			return fmt.Errorf("failed to transistion VoteCollector from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), err)
		}

		for _, vote := range cachingCollector.GetVotes() {
			task := csm.reIngestVoteTask(vote)
			csm.workerPool.Submit(task)
		}
		return nil
	}

	return fmt.Errorf("cannot transition from %s to %s: %w", expectedCurrentStatus.String(), newStatus.String(), ErrInvalidCollectorStateTransition)
}

// caching2Verifying ensures that the collector is currently in state `CachingVotes`
// and replaces it by a newly-created ConsensusClusterVoteCollector.
// Returns:
// * CachingVoteCollector as of before the update
// * ErrDifferentCollectorState if the VoteCollector's state is _not_ `CachingVotes`
// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
func (csm *VoteCollectorStateMachine) caching2Verifying() (*CachingVoteCollector, error) {
	csm.Lock()
	defer csm.Unlock()
	clr := csm.atomicLoadCollector()
	cachingCollector, ok := clr.(*CachingVoteCollector)
	if !ok {
		return nil, fmt.Errorf("collector's current state is %s: %w", clr.Status().String(), ErrDifferentCollectorState)
	}

	verifyingCollector, err := csm.createVerifyingCollector(csm.BaseVoteCollector)
	if err != nil {
		return nil, fmt.Errorf("could not create verifying vote collector")
	}

	csm.collector.Store(&atomicValueWrapper{collector: verifyingCollector})

	return cachingCollector, nil
}

// reIngestIncorporatedResultTask returns a functor for re-ingesting the specified
// IncorporatedResults; functor handles all potential business logic errors.
func (csm *VoteCollectorStateMachine) reIngestVoteTask(vote *model.Vote) func() {
	panic("implement me")
	task := func() {
	}
	return task
}
