package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// StateReplayCache caches value read from state together with all register touches triggered
// when reading that value from state. These register touches will be re/played on the target
// view in every GetOrRetrieveValue value call.
type StateReplayCache[T any] struct {
	cachedValue            T
	cachedState            *State
	cachedUpdatedRegisters map[string]bool
}

func NewStateReplayCache[T any]() *StateReplayCache[T] {
	return &StateReplayCache[T]{
		cachedUpdatedRegisters: make(map[string]bool),
	}
}

// GetOrRetrieveValue returns a state value. When value is not cached, `retriever` closure will be
// called to retrieve the value and the state delta will be captured and cached too; when value is
// cached the cached one will be returned, and the cached state delta will be replayed on `view`.
func (stateReplayCache *StateReplayCache[T]) GetOrRetrieveValue(
	view View,
	retriever func(*TransactionState) (T, error),
	stateParams StateParameters,
) (T, error) {
	if view == nil {
		return stateReplayCache.cachedValue, fmt.Errorf("view parameter should not be nil")
	}
	if retriever == nil {
		return stateReplayCache.cachedValue, fmt.Errorf("retriever closure should not be nil")
	}

	transactionState := NewTransactionState(view, stateParams)

	if stateReplayCache.cachedState == nil {
		nestedTxId, err := transactionState.BeginNestedTransaction()
		if err != nil {
			return stateReplayCache.cachedValue, fmt.Errorf("failed to start a nested tx: %w", err)
		}

		// retrieve state value by calling retriever closure
		stateReplayCache.cachedValue, err = retriever(transactionState)
		if err != nil {
			return stateReplayCache.cachedValue, fmt.Errorf("failed to retrieve value: %w", err)
		}

		// apply reg touches on the target view and cache the delta state for replay purposes
		stateReplayCache.cachedState, err = transactionState.Commit(nestedTxId)
		if err != nil {
			stateReplayCache.cachedState = nil
			return stateReplayCache.cachedValue, fmt.Errorf("failed to commit state: %w", err)
		}

		stateReplayCache.refreshUpdatedRegisters()
	} else {
		// if value/state is cached already, simply replay it on the target view.
		err := transactionState.AttachAndCommit(stateReplayCache.cachedState)
		if err != nil {
			return stateReplayCache.cachedValue, fmt.Errorf("failed to replay cached state: %w", err)
		}
	}

	return stateReplayCache.cachedValue, nil
}

// Invalidate clears the cached state so next GetOrRetrieveValue() call will do
// actual state read again to retrieve latest value in state.
func (stateReplayCache *StateReplayCache[T]) Invalidate(
	updatedRegiestersInTx []flow.RegisterID,
) bool {
	// detect register collision
	for _, reg := range updatedRegiestersInTx {
		_, found := stateReplayCache.cachedUpdatedRegisters[reg.String()]
		if found {
			stateReplayCache.cachedState = nil
			stateReplayCache.cachedUpdatedRegisters = make(map[string]bool)
			return true
		}
	}

	return false
}

func (stateReplayCache *StateReplayCache[T]) refreshUpdatedRegisters() {
	stateReplayCache.cachedUpdatedRegisters = make(map[string]bool)
	for _, reg := range stateReplayCache.cachedState.UpdatedRegisters() {
		stateReplayCache.cachedUpdatedRegisters[reg.String()] = true
	}
}
