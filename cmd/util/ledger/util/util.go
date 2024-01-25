package util

import (
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"

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

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (a *AccountsAtreeLedger) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	v, err := a.Accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage index allocation failed: %w", err)
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

// NopMemoryGauge is a no-op implementation of the MemoryGauge interface
type NopMemoryGauge struct{}

func (n NopMemoryGauge) MeterMemory(common.MemoryUsage) error {
	return nil
}

var _ common.MemoryGauge = (*NopMemoryGauge)(nil)

type PayloadsReadonlyLedger struct {
	Snapshot *PayloadSnapshot

	AllocateStorageIndexFunc func(owner []byte) (atree.StorageIndex, error)
	SetValueFunc             func(owner, key, value []byte) (err error)
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

func (p *PayloadsReadonlyLedger) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	if p.AllocateStorageIndexFunc != nil {
		return p.AllocateStorageIndexFunc(owner)
	}

	panic("AllocateStorageIndex not expected to be called")
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

func PayloadsFromEmulatorSnapshot(snapshotPath string) ([]*ledger.Payload, error) {
	db, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT key, value FROM ledger")
	if err != nil {
		return nil, err
	}

	var payloads []*ledger.Payload

	for rows.Next() {
		var hexKey, hexValue string

		err := rows.Scan(&hexKey, &hexValue)
		if err != nil {
			return nil, err
		}

		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, err
		}

		value, err := hex.DecodeString(hexValue)
		if err != nil {
			return nil, err
		}

		registerId := registerIDKeyFromString(string(key))

		ledgerKey := convert.RegisterIDToLedgerKey(registerId)

		payloads = append(
			payloads,
			ledger.NewPayload(
				ledgerKey,
				value,
			),
		)
	}

	return payloads, nil
}

// registerIDKeyFromString is the inverse of `flow.RegisterID.String()` method.
func registerIDKeyFromString(s string) flow.RegisterID {
	parts := strings.SplitN(s, "/", 2)

	owner := parts[0]
	key := parts[1]

	address, err := common.HexToAddress(owner)
	if err != nil {
		panic(err)
	}

	var decodedKey string

	switch key[0] {
	case '$':
		b := make([]byte, 9)
		b[0] = '$'

		int64Value, err := strconv.ParseInt(key[1:], 10, 64)
		if err != nil {
			panic(err)
		}

		binary.BigEndian.PutUint64(b[1:], uint64(int64Value))

		decodedKey = string(b)
	case '#':
		decoded, err := hex.DecodeString(key[1:])
		if err != nil {
			panic(err)
		}
		decodedKey = string(decoded)
	default:
		panic("Invalid register key")
	}

	return flow.RegisterID{
		Owner: string(address.Bytes()),
		Key:   decodedKey,
	}
}
