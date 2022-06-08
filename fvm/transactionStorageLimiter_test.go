package fvm_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	fvmmock "github.com/onflow/flow-go/fvm/mock"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter_Process(t *testing.T) {
	owner := flow.HexToAddress("1")
	t.Run("capacity > storage -> OK", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("Context").Return(&fvm.Context{LimitAccountStorage: true})
		env.On("GetStorageCapacity", mock.Anything).Return(uint64(100), nil)
		env.On("GetStorageUsed", mock.Anything).Return(uint64(99), nil)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckLimits(env, []flow.Address{owner})
		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("capacity = storage -> OK", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("Context").Return(&fvm.Context{LimitAccountStorage: true})
		env.On("GetStorageCapacity", mock.Anything).Return(uint64(100), nil)
		env.On("GetStorageUsed", mock.Anything).Return(uint64(100), nil)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckLimits(env, []flow.Address{owner})
		require.NoError(t, err, "Transaction with equal capacity than storage used should work")
	})
	t.Run("capacity < storage -> Not OK", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("Context").Return(&fvm.Context{LimitAccountStorage: true})
		env.On("GetStorageCapacity", mock.Anything).Return(uint64(100), nil)
		env.On("GetStorageUsed", mock.Anything).Return(uint64(101), nil)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckLimits(env, []flow.Address{owner})
		require.Error(t, err, "Transaction with lower capacity than storage used should fail")
	})
	t.Run("if ctx LimitAccountStorage false-> OK", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("Context").Return(&fvm.Context{LimitAccountStorage: false})
		env.On("GetStorageCapacity", mock.Anything).Return(uint64(100), nil)
		env.On("GetStorageUsed", mock.Anything).Return(uint64(101), nil)

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckLimits(env, []flow.Address{owner})
		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("non existing accounts or any other errors on fetching storage used -> Not OK", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("Context").Return(&fvm.Context{LimitAccountStorage: true})
		env.On("GetStorageCapacity", mock.Anything).Return(uint64(100), nil)
		env.On("GetStorageUsed", mock.Anything).Return(uint64(0), errors.NewAccountNotFoundError(owner))

		d := &fvm.TransactionStorageLimiter{}
		err := d.CheckLimits(env, []flow.Address{owner})
		require.Error(t, err, "check storage used on non existing account (not general registers) should fail")
	})
}
