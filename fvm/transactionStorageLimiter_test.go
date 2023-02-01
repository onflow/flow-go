package fvm_test

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	fvmmock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter(t *testing.T) {
	txnState := state.NewTransactionState(
		utils.NewSimpleView(),
		state.DefaultParameters())

	owner := flow.HexToAddress("1")

	err := txnState.Set(
		flow.RegisterID{
			Owner: string(owner[:]),
			Key:   "a",
		},
		flow.RegisterValue("foo"))
	require.NoError(t, err)
	err = txnState.Set(
		flow.RegisterID{
			Owner: string(owner[:]),
			Key:   "b",
		},
		flow.RegisterValue("bar"))
	require.NoError(t, err)

	t.Run("capacity > storage -> OK", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(99), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, flow.EmptyAddress, 0)
		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("capacity = storage -> OK", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(100), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, flow.EmptyAddress, 0)
		require.NoError(t, err, "Transaction with equal capacity than storage used should work")
	})
	t.Run("capacity = storage -> OK (dedup payer)", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(100), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, owner, 0)
		require.NoError(t, err, "Transaction with equal capacity than storage used should work")
	})
	t.Run("capacity < storage -> Not OK", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(101), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, flow.EmptyAddress, 0)
		require.Error(t, err, "Transaction with lower capacity than storage used should fail")
	})
	t.Run("capacity > storage -> OK (payer not updated)", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(99), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		txnState := state.NewTransactionState(
			utils.NewSimpleView(),
			state.DefaultParameters())
		// sanity check
		require.Empty(t, txnState.UpdatedRegisterIDs())

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, owner, 1)
		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("capacity < storage -> Not OK (payer not updated)", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(101), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		txnState := state.NewTransactionState(
			utils.NewSimpleView(),
			state.DefaultParameters())
		// sanity check
		require.Empty(t, txnState.UpdatedRegisterIDs())

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, owner, 1000)
		require.Error(t, err, "Transaction with lower capacity than storage used should fail")
	})
	t.Run("if ctx LimitAccountStorage false-> OK", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(false)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageCapacity", mock.Anything).Return(uint64(100), nil)
		env.On("GetStorageUsed", mock.Anything).Return(uint64(101), nil)
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, flow.EmptyAddress, 0)
		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("non existing accounts or any other errors on fetching storage used -> Not OK", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		env := &fvmmock.Environment{}
		env.On("Chain").Return(chain)
		env.On("LimitAccountStorage").Return(true)
		env.On("StartChildSpan", mock.Anything).Return(
			tracing.NewMockTracerSpan())
		env.On("GetStorageUsed", mock.Anything).Return(uint64(0), errors.NewAccountNotFoundError(owner))
		env.On("AccountsStorageCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.NewArray([]cadence.Value{
				bytesToUFix64(100),
			}),
			nil,
		)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, txnState, flow.EmptyAddress, 0)
		require.Error(t, err, "check storage used on non existing account (not general registers) should fail")
	})
}

func bytesToUFix64(b uint64) cadence.Value {
	return cadence.UFix64(b * 100)
}
