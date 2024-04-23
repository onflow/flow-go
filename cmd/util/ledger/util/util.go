package util

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func newRegisterID(owner []byte, key []byte) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
}

type AccountsAtreeLedger struct {
	Accounts environment.Accounts
}

func NewAccountsAtreeLedger(accounts environment.Accounts) *AccountsAtreeLedger {
	return &AccountsAtreeLedger{Accounts: accounts}
}

var _ atree.Ledger = &AccountsAtreeLedger{}

func (a *AccountsAtreeLedger) GetValue(owner, key []byte) ([]byte, error) {
	registerID := newRegisterID(owner, key)
	v, err := a.Accounts.GetValue(
		registerID)
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (a *AccountsAtreeLedger) SetValue(owner, key, value []byte) error {
	registerID := newRegisterID(owner, key)
	err := a.Accounts.SetValue(registerID, value)
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

// AllocateSlabIndex allocates new storage index under the owner accounts to store a new register
func (a *AccountsAtreeLedger) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	v, err := a.Accounts.AllocateSlabIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.SlabIndex{}, fmt.Errorf("storage index allocation failed: %w", err)
	}
	return v, nil
}

type PayloadSnapshot struct {
	Payloads map[flow.RegisterID]*ledger.Payload
}

var _ snapshot.StorageSnapshot = (*PayloadSnapshot)(nil)

func NewPayloadSnapshot(payloads []*ledger.Payload) (*PayloadSnapshot, error) {
	l := &PayloadSnapshot{
		Payloads: make(map[flow.RegisterID]*ledger.Payload, len(payloads)),
	}
	for _, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		id, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, err
		}
		l.Payloads[id] = payload
	}
	return l, nil
}

func (p PayloadSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	value, exists := p.Payloads[id]
	if !exists {
		return nil, nil
	}
	return value.Value(), nil
}

type PayloadsReadonlyLedger struct {
	Snapshot *PayloadSnapshot

	AllocateSlabIndexFunc func(owner []byte) (atree.SlabIndex, error)
	SetValueFunc          func(owner, key, value []byte) (err error)
}

var _ atree.Ledger = &PayloadsReadonlyLedger{}

func (p *PayloadsReadonlyLedger) GetValue(owner, key []byte) (value []byte, err error) {
	registerID := newRegisterID(owner, key)
	v, err := p.Snapshot.Get(registerID)
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (p *PayloadsReadonlyLedger) SetValue(owner, key, value []byte) (err error) {
	if p.SetValueFunc != nil {
		return p.SetValueFunc(owner, key, value)
	}

	panic("SetValue not expected to be called")
}

func (p *PayloadsReadonlyLedger) ValueExists(owner, key []byte) (exists bool, err error) {
	registerID := newRegisterID(owner, key)
	_, ok := p.Snapshot.Payloads[registerID]
	return ok, nil
}

func (p *PayloadsReadonlyLedger) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	if p.AllocateSlabIndexFunc != nil {
		return p.AllocateSlabIndexFunc(owner)
	}

	panic("AllocateSlabIndex not expected to be called")
}

func NewPayloadsReadonlyLedger(snapshot *PayloadSnapshot) *PayloadsReadonlyLedger {
	return &PayloadsReadonlyLedger{Snapshot: snapshot}
}

// IsServiceLevelAddress returns true if the given address is the service level address.
// Which means it's not an actual account but instead holds service lever registers.
func IsServiceLevelAddress(address common.Address) bool {
	return address == common.ZeroAddress
}

var _ atree.Ledger = &PayloadsReadonlyLedger{}

// PayloadsLedger is a simple read/write in-memory atree.Ledger implementation
type PayloadsLedger struct {
	Payloads map[flow.RegisterID]*ledger.Payload

	AllocateStorageIndexFunc func(owner []byte) (atree.SlabIndex, error)
}

var _ atree.Ledger = &PayloadsLedger{}

func NewPayloadsLedger(payloads map[flow.RegisterID]*ledger.Payload) *PayloadsLedger {
	return &PayloadsLedger{
		Payloads: payloads,
	}
}

func (p *PayloadsLedger) GetValue(owner, key []byte) (value []byte, err error) {
	registerID := newRegisterID(owner, key)
	v, ok := p.Payloads[registerID]
	if !ok {
		return nil, nil
	}
	return v.Value(), nil
}

func (p *PayloadsLedger) SetValue(owner, key, value []byte) (err error) {
	registerID := newRegisterID(owner, key)
	ledgerKey := convert.RegisterIDToLedgerKey(registerID)
	p.Payloads[registerID] = ledger.NewPayload(ledgerKey, value)
	return nil
}

func (p *PayloadsLedger) ValueExists(owner, key []byte) (exists bool, err error) {
	registerID := newRegisterID(owner, key)
	_, ok := p.Payloads[registerID]
	return ok, nil
}

func (p *PayloadsLedger) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	if p.AllocateStorageIndexFunc != nil {
		return p.AllocateStorageIndexFunc(owner)
	}

	panic("AllocateSlabIndex not expected to be called")
}
