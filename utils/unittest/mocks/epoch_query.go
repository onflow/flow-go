package mocks

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
)

// EpochQuery implements [protocol.EpochQuery] for testing purposes.
// Safe for concurrent use by multiple goroutines.
type EpochQuery struct {
	t         *testing.T
	mu        sync.RWMutex
	counter   uint64                             // represents the current epoch
	byCounter map[uint64]protocol.CommittedEpoch // all committed epochs
	tentative map[uint64]protocol.TentativeEpoch // only for the next epoch (counter+1) if uncommitted
}

func NewEpochQuery(t *testing.T, counter uint64, epochs ...protocol.CommittedEpoch) *EpochQuery {
	mock := &EpochQuery{
		t:         t,
		counter:   counter,
		byCounter: make(map[uint64]protocol.CommittedEpoch),
		tentative: make(map[uint64]protocol.TentativeEpoch),
	}

	for _, epoch := range epochs {
		mock.Add(epoch)
	}

	return mock
}

func (mock *EpochQuery) Current() protocol.CommittedEpoch {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	return mock.byCounter[mock.counter]
}

func (mock *EpochQuery) NextUnsafe() protocol.TentativeEpoch {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	epoch, exists := mock.tentative[mock.counter+1]
	if !exists {
		return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
	}
	_, exists = mock.byCounter[mock.counter+1]
	if exists {
		return invalid.NewEpoch(protocol.ErrNextEpochAlreadyCommitted)
	}
	return epoch
}

func (mock *EpochQuery) NextCommitted() protocol.CommittedEpoch {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	epoch, exists := mock.byCounter[mock.counter+1]
	if !exists {
		return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
	}
	return epoch
}

func (mock *EpochQuery) Previous() protocol.CommittedEpoch {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	epoch, exists := mock.byCounter[mock.counter-1]
	if !exists {
		return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
	}
	return epoch
}

// Phase returns a phase consistent with the current epoch state.
func (mock *EpochQuery) Phase() flow.EpochPhase {
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	_, exists := mock.byCounter[mock.counter+1]
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
	return mock.byCounter[counter]
}

// Transition increments the counter indicating which epoch is the "current epoch".
// It is assumed that an epoch corresponding to the current epoch counter exists;
// otherwise this mock is in a state that is illegal according to protocol rules.
func (mock *EpochQuery) Transition() {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	mock.counter++
}

// Add adds the given Committed Epoch to this EpochQuery implementation, so its
// information can be retrieved by the business logic via the [protocol.EpochQuery] API.
func (mock *EpochQuery) Add(epoch protocol.CommittedEpoch) {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	counter, err := epoch.Counter()
	require.NoError(mock.t, err, "cannot add epoch with invalid counter")
	mock.byCounter[counter] = epoch
}

// AddTentative adds the given Tentative Epoch to this EpochQuery implementation, so its
// information can be retrieved by the business logic via the [protocol.EpochQuery] API.
func (mock *EpochQuery) AddTentative(epoch protocol.TentativeEpoch) {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	counter, err := epoch.Counter()
	require.NoError(mock.t, err, "cannot add epoch with invalid counter")
	require.Equal(mock.t, mock.counter+1, counter, "may only add tentative next epoch with current counter + 1")
	mock.tentative[counter] = epoch
}
