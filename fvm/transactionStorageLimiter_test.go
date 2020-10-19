package fvm_test

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter_Process(t *testing.T) {
	t.Run("capacity > storage == OK", func(t *testing.T) {
		owner := "1"
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 99),
				storageCapacityOKV(owner, 100),
			})
		d := &fvm.TransactionStorageLimiter{}
		if err := d.Process(nil, fvm.Context{}, nil, ledger); err != nil {
			t.Errorf("Transaction with higher capacity than storage used should work")
		}
	})
	t.Run("capacity = storage == OK", func(t *testing.T) {
		owner := "1"
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 100),
				storageCapacityOKV(owner, 100),
			})
		d := &fvm.TransactionStorageLimiter{}
		if err := d.Process(nil, fvm.Context{}, nil, ledger); err != nil {
			t.Errorf("Transaction with equal capacity than storage used should work")
		}
	})
	t.Run("capacity < storage == Not OK", func(t *testing.T) {
		owner := "1"
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 101),
				storageCapacityOKV(owner, 100),
			})
		d := &fvm.TransactionStorageLimiter{}
		if err := d.Process(nil, fvm.Context{}, nil, ledger); err == nil {
			t.Errorf("Transaction with lower capacity than storage used should fail")
		}
	})

	t.Run("two registers change with the same account: call get only once", func(t *testing.T) {
		owner := "1"
		ledger := newMockLedger(
			[]string{owner, owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 99),
				storageCapacityOKV(owner, 100),
			})
		d := &fvm.TransactionStorageLimiter{}
		if err := d.Process(nil, fvm.Context{}, nil, ledger); err != nil {
			t.Errorf("Transaction should no fail")
		}
		require.Equal(t, 2, ledger.GetCalls[owner])
	})

	t.Run("two registers change with different same accounts: call get once per account", func(t *testing.T) {
		owner1 := "1"
		owner2 := "2"
		ledger := newMockLedger(
			[]string{owner1, owner2},
			[]OwnerKeyValue{
				storageUsedOKV(owner1, 99),
				storageCapacityOKV(owner1, 100),
				storageUsedOKV(owner2, 999),
				storageCapacityOKV(owner2, 1000),
			})
		d := &fvm.TransactionStorageLimiter{}
		if err := d.Process(nil, fvm.Context{}, nil, ledger); err != nil {
			t.Errorf("Transaction should no fail")
		}
		require.Equal(t, 2, ledger.GetCalls[owner1])
		require.Equal(t, 2, ledger.GetCalls[owner2])
	})
}

type MockLedger struct {
	UpdatedRegisterKeys []flow.RegisterID
	StorageValues       map[string]map[string]flow.RegisterValue
	GetCalls            map[string]int
}

type OwnerKeyValue struct {
	Owner string
	Key   string
	Value uint64
}

func storageUsedOKV(owner string, value uint64) OwnerKeyValue {
	return OwnerKeyValue{
		Owner: owner,
		Key:   state.StorageUsedRegisterName,
		Value: value,
	}
}

func storageCapacityOKV(owner string, value uint64) OwnerKeyValue {
	return OwnerKeyValue{
		Owner: owner,
		Key:   state.StorageCapacityRegisterName,
		Value: value,
	}
}

func newMockLedger(updatedKeys []string, ownerKeyStorageValue []OwnerKeyValue) MockLedger {
	storageValues := make(map[string]map[string]flow.RegisterValue)
	for _, okv := range ownerKeyStorageValue {
		_, exists := storageValues[okv.Owner]
		if !exists {
			storageValues[okv.Owner] = make(map[string]flow.RegisterValue)
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, okv.Value)
		storageValues[okv.Owner][okv.Key] = buf
	}
	updatedRegisters := make([]flow.RegisterID, len(updatedKeys))
	for i, key := range updatedKeys {
		updatedRegisters[i] = flow.RegisterID{
			Owner:      key,
			Controller: "",
			Key:        "",
		}
	}

	return MockLedger{
		UpdatedRegisterKeys: updatedRegisters,
		StorageValues:       storageValues,
		GetCalls:            make(map[string]int),
	}
}

func (l MockLedger) Set(_, _, _ string, _ flow.RegisterValue) {}
func (l MockLedger) Get(owner, _, key string) (flow.RegisterValue, error) {
	l.GetCalls[owner] = l.GetCalls[owner] + 1
	return l.StorageValues[owner][key], nil
}
func (l MockLedger) Touch(_, _, _ string)  {}
func (l MockLedger) Delete(_, _, _ string) {}
func (l MockLedger) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	return l.UpdatedRegisterKeys, []flow.RegisterValue{}
}
