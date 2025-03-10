package mocks

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// EpochQuery implements [protocol.EpochQuery] for testing purposes.
// Safe for concurrent use by multiple goroutines.
type EpochQuery struct {
	t         *testing.T
	mu        sync.RWMutex
	counter   uint64                             // represents the current epoch
	committed map[uint64]protocol.CommittedEpoch // all committed epochs, by their respective epoch counter
	tentative map[uint64]protocol.TentativeEpoch // only for the next epoch (counter+1) if uncommitted
}

var _ protocol.EpochQuery = (*EpochQuery)(nil)

func NewEpochQuery(t *testing.T, counter uint64, epochs ...protocol.CommittedEpoch) *EpochQuery {
	mock := &EpochQuery{
		t:         t,
		counter:   counter,
		committed: make(map[uint64]protocol.CommittedEpoch),
		tentative: make(map[uint64]protocol.TentativeEpoch),
	}

	for _, epoch := range epochs {
		mock.AddCommitted(epoch)
	}

	return mock
}

func (mock *EpochQuery) Current() (protocol.CommittedEpoch, error) {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	epoch, exists := mock.committed[mock.counter]
	if !exists {
		return nil, fmt.Errorf("EpochQuery mock has no entry for current epoch - likely a test is not properly set up")
	}
	return epoch, nil
}

func (mock *EpochQuery) NextUnsafe() (protocol.TentativeEpoch, error) {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	// NextUnsafe should only return a tentative epoch when we have no committed epoch for the next counter.
	// If we have a committed epoch (are implicitly in EpochPhaseCommitted) or no tentative epoch, return an error.
	// Note that in tests we do not require that a committed epoch be added as a tentative epoch first.
	_, exists := mock.committed[mock.counter+1]
	if exists {
		return nil, protocol.ErrNextEpochAlreadyCommitted
	}
	epoch, exists := mock.tentative[mock.counter+1]
	if !exists {
		return nil, protocol.ErrNextEpochNotSetup
	}
	return epoch, nil
}

func (mock *EpochQuery) NextCommitted() (protocol.CommittedEpoch, error) {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	epoch, exists := mock.committed[mock.counter+1]
	if !exists {
		return nil, protocol.ErrNextEpochNotCommitted
	}
	return epoch, nil
}

func (mock *EpochQuery) Previous() (protocol.CommittedEpoch, error) {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	epoch, exists := mock.committed[mock.counter-1]
	if !exists {
		return nil, protocol.ErrNoPreviousEpoch
	}
	return epoch, nil
}

// Phase returns a phase consistent with the current epoch state.
func (mock *EpochQuery) Phase() flow.EpochPhase {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	_, exists := mock.committed[mock.counter+1]
	if exists {
		return flow.EpochPhaseCommitted
	}
	_, exists = mock.tentative[mock.counter+1]
	if exists {
		return flow.EpochPhaseSetup
	}
	return flow.EpochPhaseStaking
}

func (mock *EpochQuery) ByCounter(counter uint64) protocol.CommittedEpoch {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	return mock.committed[counter]
}

// Transition increments the counter indicating which epoch is the "current epoch".
// It is assumed that an epoch corresponding to the current epoch counter exists;
// otherwise this mock is in a state that is illegal according to protocol rules.
func (mock *EpochQuery) Transition() {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	mock.counter++
}

// AddCommitted adds the given Committed Epoch to this EpochQuery implementation, so its
// information can be retrieved by the business logic via the [protocol.EpochQuery] API.
func (mock *EpochQuery) AddCommitted(epoch protocol.CommittedEpoch) {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	mock.committed[epoch.Counter()] = epoch
}

// AddTentative adds the given Tentative Epoch to this EpochQuery implementation, so its
// information can be retrieved by the business logic via the [protocol.EpochQuery] API.
func (mock *EpochQuery) AddTentative(epoch protocol.TentativeEpoch) {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	counter := epoch.Counter()
	require.Equal(mock.t, mock.counter+1, counter, "may only add tentative next epoch with current counter + 1")
	mock.tentative[counter] = epoch
}
