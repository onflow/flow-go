package fvm_test

import (
	"encoding/binary"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter_Process(t *testing.T) {
	t.Run("capacity > storage -> OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		sm := newMockStateHolder(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 99),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, &fvm.Context{
			LimitAccountStorage: true,
		}, nil, sm, programs.NewEmptyPrograms())
		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("capacity = storage -> OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		sm := newMockStateHolder(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 100),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, &fvm.Context{
			LimitAccountStorage: true,
		}, nil, sm, programs.NewEmptyPrograms())
		require.NoError(t, err, "Transaction with equal capacity than storage used should work")
	})
	t.Run("capacity < storage -> Not OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		sm := newMockStateHolder(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 101),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, &fvm.Context{
			LimitAccountStorage: true,
		}, nil, sm, programs.NewEmptyPrograms())
		require.Error(t, err, "Transaction with lower capacity than storage used should fail")
	})
	t.Run("non account registers are ignored", func(t *testing.T) {
		owner := ""
		sm := newMockStateHolder(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 101),
				accountExists(owner), // it has exists value, but it cannot be parsed as an address
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, &fvm.Context{
			LimitAccountStorage: true,
		}, nil, sm, programs.NewEmptyPrograms())

		require.NoError(t, err)
	})
	t.Run("account registers without exists are ignored", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		sm := newMockStateHolder(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsed(owner, 101),
			})
		d := &fvm.TransactionStorageLimiter{
			GetStorageCapacityFuncFactory: mockGetStorageCapacityFuncFactory,
		}

		err := d.Process(nil, &fvm.Context{
			LimitAccountStorage: true,
		}, nil, sm, programs.NewEmptyPrograms())

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

func newMockStateHolder(updatedKeys []string, ownerKeyStorageValue []OwnerKeyValue) *state.StateHolder {

	view := utils.NewSimpleView()
	s := state.NewState(view)
	sm := state.NewStateHolder(s)

	for _, okv := range ownerKeyStorageValue {
		_ = s.Set(okv.Owner, "", okv.Key, uint64ToBinary(okv.Value))
	}

	for _, key := range updatedKeys {
		_ = s.Set(key, "", "", []byte("1"))
	}

	return sm
}

func mockGetStorageCapacityFuncFactory(_ *fvm.VirtualMachine, _ fvm.Context, _ *fvm.TransactionProcedure, _ *state.StateHolder, _ *programs.Programs) (func(address common.Address) (value uint64, err error), error) {
	return func(address common.Address) (value uint64, err error) {
		return 100, nil
	}, nil
}

// uint64ToBinary converst a uint64 to a byte slice (big endian)
func uint64ToBinary(integer uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, integer)
	return b
}
