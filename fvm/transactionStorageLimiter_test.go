package fvm_test

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionStorageLimiter_Process(t *testing.T) {
	t.Run("capacity > storage -> OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 99),
				storageCapacityOKV(owner, 100),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.NoError(t, err, "Transaction with higher capacity than storage used should work")
	})
	t.Run("capacity = storage -> OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 100),
				storageCapacityOKV(owner, 100),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.NoError(t, err, "Transaction with equal capacity than storage used should work")
	})
	t.Run("capacity < storage -> Not OK", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 101),
				storageCapacityOKV(owner, 100),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.Error(t, err, "Transaction with lower capacity than storage used should fail")
	})
	t.Run("if two registers change on the same account, only check capacity once", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		ledger := newMockLedger(
			[]string{owner, owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 99),
				storageCapacityOKV(owner, 100),
				accountExists(owner),
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.NoError(t, err)
		// three touches per account: get exists, get capacity, get used
		require.Equal(t, 3, ledger.GetCalls[owner])
	})
	t.Run("two registers change on different accounts, only check capacity once per account", func(t *testing.T) {
		owner1 := string(flow.HexToAddress("1").Bytes())
		owner2 := string(flow.HexToAddress("2").Bytes())
		ledger := newMockLedger(
			[]string{owner1, owner1, owner2, owner2},
			[]OwnerKeyValue{
				storageUsedOKV(owner1, 99),
				storageCapacityOKV(owner1, 100),
				accountExists(owner2),
				storageUsedOKV(owner2, 999),
				storageCapacityOKV(owner2, 1000),
				accountExists(owner2),
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.NoError(t, err)
		// three touches per account: get exists, get capacity, get used
		require.Equal(t, 3, ledger.GetCalls[owner1])
		require.Equal(t, 3, ledger.GetCalls[owner2])
	})
	t.Run("non account registers are ignored", func(t *testing.T) {
		owner := ""
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 101),
				storageCapacityOKV(owner, 100),
				accountExists(owner), // it has exists value, but it cannot be parsed as an address
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.NoError(t, err)
	})
	t.Run("account registers without exists are ignored", func(t *testing.T) {
		owner := string(flow.HexToAddress("1").Bytes())
		ledger := newMockLedger(
			[]string{owner},
			[]OwnerKeyValue{
				storageUsedOKV(owner, 101),
				storageCapacityOKV(owner, 100),
			})
		d := &fvm.TransactionStorageLimiter{}

		err := d.Process(nil, fvm.Context{}, nil, ledger)

		require.NoError(t, err)
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
		Key:   "storage_used",
		Value: value,
	}
}

func storageCapacityOKV(owner string, value uint64) OwnerKeyValue {
	return OwnerKeyValue{
		Owner: owner,
		Key:   "storage_capacity",
		Value: value,
	}
}

func accountExists(owner string) OwnerKeyValue {
	return OwnerKeyValue{
		Owner: owner,
		Key:   "exists",
		Value: 1,
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
