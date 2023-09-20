package storage

import (
	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

type ledger struct {
	accounts environment.Accounts
}

var _ atree.Ledger = &ledger{}

func NewLedger(accounts environment.Accounts) *ledger {
	l := &ledger{
		accounts: accounts,
	}
	l.Setup()
	return l
}

// TODO deal with this setup better
func (l *ledger) Setup() {
	exists, err := l.accounts.Exists(FlexAddress)
	if err != nil {
		panic(err)
	}
	if !exists {
		err = l.accounts.Create(nil, FlexAddress)
		if err != nil {
			panic(err)
		}
	}
}

// GetValue gets a value for the given key in the storage, owned by the given account.
// TODO - question - do I need common.CopyBytes(data)
func (l *ledger) GetValue(_, key []byte) (value []byte, err error) {
	return l.accounts.GetValue(flow.RegisterID{
		Owner: string(FlexAddress.Bytes()),
		Key:   string(key),
	})
}

// SetValue sets a value for the given key in the storage, owned by the given account.
// owner is discarded as input
func (l *ledger) SetValue(_, key, value []byte) (err error) {
	return l.accounts.SetValue(flow.RegisterID{
		Owner: string(FlexAddress.Bytes()),
		Key:   string(key),
	}, value)
}

// ValueExists returns true if the given key exists in the storage, owned by the given account.
func (l *ledger) ValueExists(_, key []byte) (exists bool, err error) {
	data, err := l.GetValue(nil, key)
	return len(data) > 0, err
}

// AllocateStorageIndex allocates a new storage index under the given account.
func (l *ledger) AllocateStorageIndex(_ []byte) (atree.StorageIndex, error) {
	return l.accounts.AllocateStorageIndex(FlexAddress)
}
