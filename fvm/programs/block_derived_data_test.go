package programs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
)

func TestDerivedTransactionDataGetOrCompute(t *testing.T) {
	blockDerivedData := NewEmptyBlockDerivedData[string, int]()

	derivedKey := "derivedKey"
	derivedValue := 12345
	addr := "addr"
	key := "key"

	t.Run("compute value", func(t *testing.T) {
		view := utils.NewSimpleView()
		txnState := state.NewTransactionState(view, state.DefaultParameters())

		calledValFunc := false
		valFunc := func(
			txnState *state.TransactionState,
			_ string,
		) (
			int,
			error,
		) {
			calledValFunc = true
			_, err := txnState.Get(addr, key, true)
			if err != nil {
				return 0, err
			}

			return derivedValue, nil
		}

		txnDerivedData, err := blockDerivedData.NewTransactionDerivedData(0, 0)
		assert.Nil(t, err)

		val, err := txnDerivedData.GetOrCompute(
			txnState,
			derivedKey,
			valFunc)
		assert.Nil(t, err)
		assert.Equal(t, derivedValue, val)
		assert.True(t, calledValFunc)

		assert.True(
			t,
			view.Ledger.RegisterTouches[utils.FullKey(addr, key)])

		// Commit to setup the next test.
		err = txnDerivedData.Commit()
		assert.Nil(t, err)
	})

	t.Run("get value", func(t *testing.T) {
		view := utils.NewSimpleView()
		txnState := state.NewTransactionState(view, state.DefaultParameters())

		calledValFunc := false
		valFunc := func(
			txnState *state.TransactionState,
			_ string,
		) (
			int,
			error,
		) {
			calledValFunc = true
			_, err := txnState.Get(addr, key, true)
			if err != nil {
				return 0, err
			}

			return derivedValue, nil
		}

		txnDerivedData, err := blockDerivedData.NewTransactionDerivedData(1, 1)
		assert.Nil(t, err)

		val, err := txnDerivedData.GetOrCompute(
			txnState,
			derivedKey,
			valFunc)
		assert.Nil(t, err)
		assert.Equal(t, derivedValue, val)
		assert.False(t, calledValFunc)

		assert.True(
			t,
			view.Ledger.RegisterTouches[utils.FullKey(addr, key)])
	})
}
