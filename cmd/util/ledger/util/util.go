package util

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type AccountsAtreeLedger struct {
	Accounts environment.Accounts
}

func NewAccountsAtreeLedger(accounts environment.Accounts) *AccountsAtreeLedger {
	return &AccountsAtreeLedger{Accounts: accounts}
}

var _ atree.Ledger = &AccountsAtreeLedger{}

func (a *AccountsAtreeLedger) GetValue(owner, key []byte) ([]byte, error) {
	v, err := a.Accounts.GetValue(
		flow.NewRegisterID(
			flow.BytesToAddress(owner),
			string(key)))
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (a *AccountsAtreeLedger) SetValue(owner, key, value []byte) error {
	err := a.Accounts.SetValue(
		flow.NewRegisterID(
			flow.BytesToAddress(owner),
			string(key)),
		value)
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

// ApplyChangesAndGetNewPayloads applies the given changes to the snapshot and returns the new payloads.
// the snapshot is destroyed.
func (p *PayloadSnapshot) ApplyChangesAndGetNewPayloads(
	changes map[flow.RegisterID]flow.RegisterValue,
	expectedChangeAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) ([]*ledger.Payload, error) {
	originalPayloads := p.Payloads

	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	// Add all new payloads.
	for id, value := range changes {
		delete(originalPayloads, id)
		if len(value) == 0 {
			continue
		}

		if expectedChangeAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(id.Owner))

			if _, ok := expectedChangeAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", id.String()).
					Str("actual_address", ownerAddress.Hex()).
					Interface("expected_addresses", expectedChangeAddresses).
					Hex("value", value).
					Msg("key is part of the change set, but is for a different account")
			}
		}

		key := convert.RegisterIDToLedgerKey(id)
		newPayloads = append(newPayloads, ledger.NewPayload(key, value))
	}

	// Add any old payload that wasn't updated.
	for id, value := range originalPayloads {
		if len(value.Value()) == 0 {
			// This is strange, but we don't want to add empty values. Log it.
			logger.Warn().Msgf("empty value for key %s", id)
			continue
		}

		newPayloads = append(newPayloads, value)
	}

	return newPayloads, nil
}

type PayloadsReadonlyLedger struct {
	Snapshot *PayloadSnapshot

	AllocateSlabIndexFunc func(owner []byte) (atree.SlabIndex, error)
	SetValueFunc          func(owner, key, value []byte) (err error)
}

func (p *PayloadsReadonlyLedger) GetValue(owner, key []byte) (value []byte, err error) {
	v, err := p.Snapshot.Get(flow.NewRegisterID(flow.BytesToAddress(owner), string(key)))
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
	_, ok := p.Snapshot.Payloads[flow.NewRegisterID(flow.BytesToAddress(owner), string(key))]
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
