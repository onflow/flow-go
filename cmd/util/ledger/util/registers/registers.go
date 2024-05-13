package registers

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type ForEachCallback func(owner string, key string, value []byte) error

type Registers interface {
	Get(owner string, key string) ([]byte, error)
	Set(owner string, key string, value []byte) error
	ForEach(f ForEachCallback) error
	Payloads() []*ledger.Payload
}

// ByAccount represents the registers of all accounts
type ByAccount struct {
	registers map[string]*AccountRegisters
}

var _ Registers = &ByAccount{}

func NewByAccount() *ByAccount {
	return &ByAccount{
		registers: make(map[string]*AccountRegisters),
	}
}

const payloadsPerAccountEstimate = 200

func NewByAccountFromPayloads(payloads []*ledger.Payload) (*ByAccount, error) {
	byAccount := &ByAccount{
		registers: make(map[string]*AccountRegisters, len(payloads)/payloadsPerAccountEstimate),
	}

	for _, payload := range payloads {
		registerID, registerValue, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		err = byAccount.Set(
			registerID.Owner,
			registerID.Key,
			registerValue,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to set register: %w", err)
		}
	}

	return byAccount, nil
}

func (b *ByAccount) Get(owner string, key string) ([]byte, error) {
	accountRegisters, ok := b.registers[owner]
	if !ok {
		return nil, nil
	}
	return accountRegisters.registers[key], nil
}

func (b *ByAccount) Set(owner string, key string, value []byte) error {
	accountRegisters := b.AccountRegisters(owner)
	accountRegisters.registers[key] = value
	return nil
}

func (b *ByAccount) ForEach(f ForEachCallback) error {
	for _, accountRegisters := range b.registers {
		err := accountRegisters.ForEach(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *ByAccount) Payloads() []*ledger.Payload {
	payloads := make([]*ledger.Payload, 0, len(b.registers)*payloadsPerAccountEstimate)
	for _, accountRegisters := range b.registers {
		payloads = accountRegisters.appendPayloads(payloads)
	}
	return payloads
}

func (b *ByAccount) ForEachAccount(f func(accountRegisters *AccountRegisters) error) error {
	for _, accountRegisters := range b.registers {
		err := f(accountRegisters)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *ByAccount) AccountCount() int {
	return len(b.registers)
}

func (b *ByAccount) AccountRegisters(owner string) *AccountRegisters {
	accountRegisters, ok := b.registers[owner]
	if !ok {
		accountRegisters = NewAccountRegisters(owner)
		b.registers[owner] = accountRegisters
	}
	return accountRegisters
}

func (b *ByAccount) SetAccountRegisters(newAccountRegisters *AccountRegisters) *AccountRegisters {
	owner := newAccountRegisters.Owner()
	oldAccountRegisters := b.registers[owner]
	b.registers[owner] = newAccountRegisters
	return oldAccountRegisters
}

func (b *ByAccount) Count() int {
	// TODO: parallelize
	count := 0
	for _, accountRegisters := range b.registers {
		count += accountRegisters.Count()
	}
	return count
}

// AccountRegisters represents the registers of an account
type AccountRegisters struct {
	owner     string
	registers map[string][]byte
}

func NewAccountRegisters(owner string) *AccountRegisters {
	return &AccountRegisters{
		owner:     owner,
		registers: make(map[string][]byte),
	}
}

var _ Registers = &AccountRegisters{}

func (a *AccountRegisters) Get(owner string, key string) ([]byte, error) {
	if owner != a.owner {
		return nil, fmt.Errorf("owner mismatch: expected %s, got %s", a.owner, owner)
	}
	return a.registers[key], nil
}

func (a *AccountRegisters) Set(owner string, key string, value []byte) error {
	if owner != a.owner {
		return fmt.Errorf("owner mismatch: expected %s, got %s", a.owner, owner)
	}
	if len(value) == 0 {
		delete(a.registers, key)
	} else {
		a.registers[key] = value
	}
	return nil
}

func (a *AccountRegisters) ForEach(f ForEachCallback) error {
	for key, value := range a.registers {
		err := f(a.owner, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *AccountRegisters) Count() int {
	return len(a.registers)
}

func (a *AccountRegisters) Owner() string {
	return a.owner
}

func (a *AccountRegisters) Payloads() []*ledger.Payload {
	payloads := make([]*ledger.Payload, 0, len(a.registers))
	return a.appendPayloads(payloads)
}

func (a *AccountRegisters) appendPayloads(payloads []*ledger.Payload) []*ledger.Payload {
	for key, value := range a.registers {
		if len(value) == 0 {
			continue
		}

		ledgerKey := convert.RegisterIDToLedgerKey(flow.RegisterID{
			Owner: a.owner,
			Key:   key,
		})
		payload := ledger.NewPayload(ledgerKey, value)
		payloads = append(payloads, payload)
	}
	return payloads
}

func NewAccountRegistersFromPayloads(owner string, payloads []*ledger.Payload) (*AccountRegisters, error) {
	accountRegisters := NewAccountRegisters(owner)

	for _, payload := range payloads {
		registerID, registerValue, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		err = accountRegisters.Set(
			registerID.Owner,
			registerID.Key,
			registerValue,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to set register: %w", err)
		}
	}

	return accountRegisters, nil
}

// StorageSnapshot adapts Registers to the snapshot.StorageSnapshot interface
type StorageSnapshot struct {
	Registers
}

var _ snapshot.StorageSnapshot = StorageSnapshot{}

func (s StorageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return s.Registers.Get(id.Owner, id.Key)
}

// ReadOnlyLedger adapts Registers to the atree.Ledger interface
type ReadOnlyLedger struct {
	Registers
}

var _ atree.Ledger = ReadOnlyLedger{}

func (l ReadOnlyLedger) GetValue(owner, key []byte) (value []byte, err error) {
	return l.Registers.Get(string(owner), string(key))
}

func (l ReadOnlyLedger) ValueExists(owner, key []byte) (exists bool, err error) {
	value, err := l.Registers.Get(string(owner), string(key))
	if err != nil {
		return false, err
	}
	return value != nil, nil
}

func (l ReadOnlyLedger) SetValue(_, _, _ []byte) error {
	panic("unexpected call of SetValue")
}

func (l ReadOnlyLedger) AllocateStorageIndex(_ []byte) (atree.StorageIndex, error) {
	panic("unexpected call of AllocateStorageIndex")
}

// ApplyChanges applies the given changes to the given registers,
// and verifies that the changes are only for the expected addresses.
func ApplyChanges(
	registers Registers,
	changes map[flow.RegisterID]flow.RegisterValue,
	expectedChangeAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) error {

	for registerID, newValue := range changes {

		if expectedChangeAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(registerID.Owner))

			if _, ok := expectedChangeAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", registerID.String()).
					Str("actual_address", ownerAddress.Hex()).
					Interface("expected_addresses", expectedChangeAddresses).
					Hex("value", newValue).
					Msg("key is part of the change set, but is for a different account")
			}
		}

		err := registers.Set(registerID.Owner, registerID.Key, newValue)
		if err != nil {
			return fmt.Errorf("failed to set register: %w", err)
		}
	}

	return nil
}
