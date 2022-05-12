package migrations

import (
	"bytes"
	"fmt"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/engine/execution/state"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func KeyToRegisterID(key ledger.Key) (flow.RegisterID, error) {
	if len(key.KeyParts) != 3 ||
		key.KeyParts[0].Type != state.KeyPartOwner ||
		key.KeyParts[1].Type != state.KeyPartController ||
		key.KeyParts[2].Type != state.KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("key not in expected format %s", key.String())
	}

	return flow.NewRegisterID(
		string(key.KeyParts[0].Value),
		string(key.KeyParts[1].Value),
		string(key.KeyParts[2].Value),
	), nil
}

func registerIDToKey(registerID flow.RegisterID) ledger.Key {
	newKey := ledger.Key{}
	newKey.KeyParts = []ledger.KeyPart{
		{
			Type:  state.KeyPartOwner,
			Value: []byte(registerID.Owner),
		},
		{
			Type:  state.KeyPartController,
			Value: []byte(registerID.Controller),
		},
		{
			Type:  state.KeyPartKey,
			Value: []byte(registerID.Key),
		},
	}
	return newKey
}

type AccountsAtreeLedger struct {
	Accounts fvmState.Accounts
}

func NewAccountsAtreeLedger(accounts fvmState.Accounts) *AccountsAtreeLedger {
	return &AccountsAtreeLedger{Accounts: accounts}
}

var _ atree.Ledger = &AccountsAtreeLedger{}

func (a *AccountsAtreeLedger) GetValue(owner, key []byte) ([]byte, error) {
	v, err := a.Accounts.GetValue(
		flow.BytesToAddress(owner),
		string(key),
	)
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (a *AccountsAtreeLedger) SetValue(owner, key, value []byte) error {
	err := a.Accounts.SetValue(
		flow.BytesToAddress(owner),
		string(key),
		value,
	)
	if err != nil {
		return fmt.Errorf("setting value failed: %w", err)
	}
	return nil
}

func (a *AccountsAtreeLedger) ValueExists(owner, key []byte) (exists bool, err error) {
	v, err := a.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("checking value existence failed: %w", err)
	}

	return len(v) > 0, nil
}

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (a *AccountsAtreeLedger) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	v, err := a.Accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}

func splitPayloads(inp []ledger.Payload) (fvmPayloads []ledger.Payload, storagePayloads []ledger.Payload, slabPayloads []ledger.Payload) {
	for _, p := range inp {
		if fvmState.IsFVMStateKey(
			string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[1].Value),
			string(p.Key.KeyParts[2].Value),
		) {
			fvmPayloads = append(fvmPayloads, p)
			continue
		}
		if bytes.HasPrefix(p.Key.KeyParts[2].Value, []byte(atree.LedgerBaseStorageSlabPrefix)) {
			slabPayloads = append(slabPayloads, p)
			continue
		}
		// otherwise this is a storage payload
		storagePayloads = append(storagePayloads, p)
	}
	return
}
