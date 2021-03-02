package fvm_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter_Process(t *testing.T) {
	t.Run("capacity > storage -> OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		st := newMockState(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 99),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, fvm.Context{
			LimitAccountStorage: true,
		}, nil, st, fvm.NewEmptyPrograms())

		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("capacity = storage -> OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		st := newMockState(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 100),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, fvm.Context{
			LimitAccountStorage: true,
		}, nil, st, fvm.NewEmptyPrograms())

		require.NoError(t, err, "Transaction with equal capacity than storage used should work")
	})
	t.Run("capacity < storage -> Not OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		st := newMockState(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 101),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, fvm.Context{
			LimitAccountStorage: true,
		}, nil, st, fvm.NewEmptyPrograms())

		require.Error(t, err, "Transaction with lower capacity than storage used should fail")
	})
	t.Run("non account registers are ignored", func(t *testing.T) {
		owner := ""
		st := newMockState(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 101),
				accountExists(owner), // it has exists value, but it cannot be parsed as an address
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, fvm.Context{
			LimitAccountStorage: true,
		}, nil, st, fvm.NewEmptyPrograms())

		require.NoError(t, err)
	})
	t.Run("account registers without exists are ignored", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		st := newMockState(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 101),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, fvm.Context{
			LimitAccountStorage: true,
		}, nil, st, fvm.NewEmptyPrograms())

		require.NoError(t, err)
	})
}

type OwnerKeyValue struct {
	Owner string
	Key   string
	Value uint64
}

func storageUsed(owner string, value uint64) OwnerKeyValue {
	return OwnerKeyValue{
		Owner: owner,
		Key:   state.KeyStorageUsed,
		Value: value,
	}
}

func accountExists(owner string) OwnerKeyValue {
	return OwnerKeyValue{
		Owner: owner,
		Key:   state.KeyExists,
		Value: 1,
	}
}

func newMockState(updatedKeys []string, ownerKeyStorageValue []OwnerKeyValue) *state.State {

	ledger := state.NewMapLedger()
	s := state.NewState(ledger)

	for _, okv := range ownerKeyStorageValue {
		_ = s.Set(okv.Owner, "", okv.Key, utils.Uint64ToBinary(okv.Value))
	}
	_ = s.Commit()

	for _, key := range updatedKeys {
		_ = s.Set(key, "", "", []byte("1"))
	}

	return s
}

func mockGetStorageCapacityFuncFactory(_ *fvm.VirtualMachine, _ fvm.Context, _ *fvm.TransactionProcedure, _ *state.State, _ *fvm.Programs) (func(address common.Address) (value uint64, err error), error) {
	return func(address common.Address) (value uint64, err error) {
		return 100, nil
	}, nil
}
