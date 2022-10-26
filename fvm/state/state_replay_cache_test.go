package state_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestStateReplayCache(t *testing.T) {

	t.Run("retriever closure called at first read", func(t *testing.T) {
		view := utils.NewSimpleView()
		cache1 := state.NewStateReplayCache[int]()

		testValue := int(123)
		testRegsToBeTouched := []flow.RegisterID{
			{
				Owner: "owner1",
				Key:   "key1",
			},
			{
				Owner: "owner2",
				Key:   "key2",
			},
		}
		retrievedValue, err := cache1.GetOrRetrieveValue(
			view,
			func(ts *state.TransactionState) (int, error) {
				for _, reg := range testRegsToBeTouched {
					_ = view.Set(reg.Owner, reg.Key, flow.RegisterValue{})
				}
				return testValue, nil
			},
			state.DefaultParameters(),
		)
		require.NoError(t, err)
		require.Equal(t, testValue, retrievedValue)
		updatedRegs, _ := view.RegisterUpdates()
		require.True(t, registerIdSliceDeepEqual(testRegsToBeTouched, updatedRegs))
	})

	t.Run("retriever closure not called with cached value", func(t *testing.T) {
		view := utils.NewSimpleView()
		cache1 := state.NewStateReplayCache[int]()

		testValue := int(123)
		closureCalledCount := 0
		retriever := func(ts *state.TransactionState) (int, error) {
			closureCalledCount++
			return testValue, nil
		}

		// 1st read
		retrievedValue, err := cache1.GetOrRetrieveValue(
			view,
			retriever,
			state.DefaultParameters(),
		)
		require.NoError(t, err)
		require.Equal(t, testValue, retrievedValue)
		require.Equal(t, 1, closureCalledCount)

		// 2nd read
		retrievedValue, err = cache1.GetOrRetrieveValue(
			view,
			retriever,
			state.DefaultParameters(),
		)
		require.NoError(t, err)
		require.Equal(t, testValue, retrievedValue)
		require.Equal(t, 1, closureCalledCount) // counter should not increase
	})

	t.Run("retriever closure returns error", func(t *testing.T) {
		view := utils.NewSimpleView()
		cache1 := state.NewStateReplayCache[int]()

		closureCalledCount := 0
		retrieverError := func(ts *state.TransactionState) (int, error) {
			closureCalledCount++
			return 0, errors.New("closure failure")
		}

		// 1st call
		_, err := cache1.GetOrRetrieveValue(
			view,
			retrieverError,
			state.DefaultParameters(),
		)
		require.Error(t, err)
		require.Equal(t, 1, closureCalledCount)

		// 2nd call
		_, err = cache1.GetOrRetrieveValue(
			view,
			retrieverError,
			state.DefaultParameters(),
		)
		require.Error(t, err)
		require.Equal(t, 2, closureCalledCount)

		// 3rd call with working retriever
		retrieverOk := func(ts *state.TransactionState) (int, error) {
			closureCalledCount++
			return 0, nil
		}
		_, err = cache1.GetOrRetrieveValue(
			view,
			retrieverOk,
			state.DefaultParameters(),
		)
		require.NoError(t, err)
		require.Equal(t, 3, closureCalledCount)
	})

	t.Run("cache invalidation", func(t *testing.T) {
		cache1 := state.NewStateReplayCache[int]()

		testValue := int(123)
		testRegsToBeTouched := []flow.RegisterID{
			{
				Owner: "owner1",
				Key:   "key1",
			},
			{
				Owner: "owner2",
				Key:   "key2",
			},
		}
		closureCalledCount := 0
		retriever := func(ts *state.TransactionState) (int, error) {
			closureCalledCount++
			for _, reg := range testRegsToBeTouched {
				_ = ts.Set(reg.Owner, reg.Key, flow.RegisterValue{}, false)
			}
			return testValue, nil
		}

		// 1st read
		viewToReplayOn := utils.NewSimpleView()
		retrievedValue, err := cache1.GetOrRetrieveValue(
			viewToReplayOn,
			retriever,
			state.DefaultParameters(),
		)
		require.NoError(t, err)
		require.Equal(t, testValue, retrievedValue)
		require.Equal(t, 1, closureCalledCount)
		regsReplayed, _ := viewToReplayOn.RegisterUpdates()
		require.True(t, registerIdSliceDeepEqual(testRegsToBeTouched, regsReplayed))

		// Invalidate - no detection found
		testRegs1 := []flow.RegisterID{
			{
				Owner: "owner3",
				Key:   "key3",
			},
		}
		invalidated := cache1.Invalidate(testRegs1)
		require.False(t, invalidated)

		// Invalidate - detection found
		invalidated = cache1.Invalidate(testRegsToBeTouched)
		require.True(t, invalidated)

		retrievedValue, err = cache1.GetOrRetrieveValue(
			viewToReplayOn,
			retriever,
			state.DefaultParameters(),
		)
		require.NoError(t, err)
		require.Equal(t, testValue, retrievedValue)
		require.Equal(t, 2, closureCalledCount)
		regsReplayed, _ = viewToReplayOn.RegisterUpdates()
		require.True(t, registerIdSliceDeepEqual(testRegsToBeTouched, regsReplayed))
	})

	t.Run("nil parameter checks", func(t *testing.T) {
		view := utils.NewSimpleView()
		cache1 := state.NewStateReplayCache[int]()

		testValue := int(123)
		retriever := func(ts *state.TransactionState) (int, error) {
			return testValue, nil
		}

		_, err := cache1.GetOrRetrieveValue(
			nil,
			retriever,
			state.DefaultParameters(),
		)
		require.Error(t, err)

		_, err = cache1.GetOrRetrieveValue(
			view,
			nil,
			state.DefaultParameters(),
		)
		require.Error(t, err)
	})
}

func registerIdSliceDeepEqual(regs1 []flow.RegisterID, regs2 []flow.RegisterID) bool {
	regMap1 := make(map[string]bool)
	for _, reg := range regs1 {
		regMap1[reg.String()] = true
	}
	regMap2 := make(map[string]bool)
	for _, reg := range regs2 {
		regMap2[reg.String()] = true
	}
	return reflect.DeepEqual(regMap1, regMap2)
}
