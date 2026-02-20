package registers

import (
	"fmt"
	"sync"

	"github.com/onflow/atree"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/environment"
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
	Count() int
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
	accountRegisters.uncheckedSet(key, value)
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

func (b *ByAccount) DestructIntoPayloads(nWorker int) []*ledger.Payload {
	payloads := make([]*ledger.Payload, b.Count())

	type job struct {
		registers *AccountRegisters
		payloads  []*ledger.Payload
	}

	var wg sync.WaitGroup

	jobs := make(chan job, b.AccountCount())

	worker := func() {
		defer wg.Done()
		for job := range jobs {
			job.registers.insertIntoPayloads(job.payloads)
		}
	}

	for range nWorker {
		wg.Add(1)
		go worker()
	}

	startOffset := 0
	for owner, accountRegisters := range b.registers {

		endOffset := startOffset + accountRegisters.Count()
		accountPayloads := payloads[startOffset:endOffset]

		jobs <- job{
			registers: accountRegisters,
			payloads:  accountPayloads,
		}

		// Remove the account from the map to reduce memory usage.
		// The account registers are now stored in the payloads,
		// so we don't need to keep them in the by-account registers.
		// This allows GC to collect converted account registers during the loop.
		delete(b.registers, owner)

		startOffset = endOffset
	}
	close(jobs)

	wg.Wait()

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

func (b *ByAccount) HasAccountOwner(owner string) bool {
	_, ok := b.registers[owner]
	return ok
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
		return nil, fmt.Errorf("owner mismatch: expected %x, got %x", a.owner, owner)
	}
	return a.registers[key], nil
}

func (a *AccountRegisters) Set(owner string, key string, value []byte) error {
	if owner != a.owner {
		return fmt.Errorf("owner mismatch: expected %x, got %x", a.owner, owner)
	}
	a.uncheckedSet(key, value)
	return nil
}

func (a *AccountRegisters) uncheckedSet(key string, value []byte) {
	if len(value) == 0 {
		delete(a.registers, key)
	} else {
		a.registers[key] = value
	}
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
	payloads := make([]*ledger.Payload, a.Count())
	a.insertIntoPayloads(payloads)
	return payloads
}

// insertIntoPayloads inserts the registers into the given payloads slice.
// The payloads slice must have the same size as the number of registers.
func (a *AccountRegisters) insertIntoPayloads(payloads []*ledger.Payload) {
	payloadCount := len(payloads)
	registerCount := len(a.registers)
	if payloadCount != registerCount {
		panic(fmt.Errorf(
			"given payloads slice has wrong size: got %d, expected %d",
			payloadCount,
			registerCount,
		))
	}

	index := 0
	for key, value := range a.registers {
		if len(value) == 0 {
			panic(fmt.Errorf("unexpected empty register value: %x, %x", a.owner, key))
		}

		ledgerKey := convert.RegisterIDToLedgerKey(flow.RegisterID{
			Owner: a.owner,
			Key:   key,
		})
		payload := ledger.NewPayload(ledgerKey, value)
		payloads[index] = payload
		index++
	}
}

// Merge merges the registers from the other AccountRegisters into this AccountRegisters.
func (a *AccountRegisters) Merge(other *AccountRegisters) error {
	for key, value := range other.registers {
		_, ok := a.registers[key]
		if ok {
			return fmt.Errorf("key already exists: %s", key)
		}
		a.registers[key] = value
	}
	return nil
}

func (a *AccountRegisters) ForEachKey(f func(key string) error) error {
	for key := range a.registers {
		err := f(key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *AccountRegisters) PayloadSize() int {
	size := 0
	for key, value := range a.registers {
		registerKey := flow.RegisterID{
			Owner: a.owner,
			Key:   key,
		}
		size += environment.RegisterSize(registerKey, value)
	}
	return size
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

func (l ReadOnlyLedger) GetValue(address, key []byte) (value []byte, err error) {
	owner := flow.AddressToRegisterOwner(flow.BytesToAddress(address))
	return l.Registers.Get(owner, string(key))
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

func (l ReadOnlyLedger) AllocateSlabIndex(_ []byte) (atree.SlabIndex, error) {
	panic("unexpected call of AllocateSlabIndex")
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

				expectedChangeAddressesArray := zerolog.Arr()
				for expectedChangeAddress := range expectedChangeAddresses {
					expectedChangeAddressesArray =
						expectedChangeAddressesArray.Str(expectedChangeAddress.Hex())
				}

				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", registerID.String()).
					Str("actual_address", ownerAddress.Hex()).
					Array("expected_addresses", expectedChangeAddressesArray).
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
