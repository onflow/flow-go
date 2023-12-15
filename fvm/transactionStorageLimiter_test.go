package fvm_test

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	fvmmock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter(t *testing.T) {
	owner := flow.HexToAddress("1")
	executionSnapshot := &snapshot.ExecutionSnapshot{
		WriteSet: map[flow.RegisterID]flow.RegisterValue{
			flow.NewRegisterID(owner, "a"): flow.RegisterValue("foo"),
			flow.NewRegisterID(owner, "b"): flow.RegisterValue("bar"),
		},
	}

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
		err := d.CheckStorageLimits(env, executionSnapshot, flow.EmptyAddress, 0)
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
		err := d.CheckStorageLimits(env, executionSnapshot, flow.EmptyAddress, 0)
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
		err := d.CheckStorageLimits(env, executionSnapshot, owner, 0)
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
		err := d.CheckStorageLimits(env, executionSnapshot, flow.EmptyAddress, 0)
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

		executionSnapshot = &snapshot.ExecutionSnapshot{}

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, executionSnapshot, owner, 1)
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

		executionSnapshot = &snapshot.ExecutionSnapshot{}

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckStorageLimits(env, executionSnapshot, owner, 1000)
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
		err := d.CheckStorageLimits(env, executionSnapshot, flow.EmptyAddress, 0)
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
		err := d.CheckStorageLimits(env, executionSnapshot, flow.EmptyAddress, 0)
		require.Error(t, err, "check storage used on non existing account (not general registers) should fail")
	})
}

func bytesToUFix64(b uint64) cadence.Value {
	return cadence.UFix64(b * 100)
}
