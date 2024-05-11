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
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func newRegisterID(owner []byte, key []byte) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
}

type AccountsAtreeLedger struct {
	Accounts environment.Accounts
	temp     map[string][]byte
}

func NewAccountsAtreeLedger(accounts environment.Accounts) *AccountsAtreeLedger {
	return &AccountsAtreeLedger{
		Accounts: accounts,
		temp:     make(map[string][]byte),
	}
}

var _ atree.Ledger = &AccountsAtreeLedger{}

func (a *AccountsAtreeLedger) GetValue(owner, key []byte) ([]byte, error) {
	if common.Address(owner) == common.ZeroAddress {
		return a.temp[string(key)], nil
	}

	registerID := newRegisterID(owner, key)
	v, err := a.Accounts.GetValue(registerID)
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (a *AccountsAtreeLedger) SetValue(owner, key, value []byte) error {
	if common.Address(owner) == common.ZeroAddress {
		a.temp[string(key)] = value
		return nil
	}

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

// AllocateStorageIndex allocates new storage index under the owner accounts to store a new register
func (a *AccountsAtreeLedger) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	v, err := a.Accounts.AllocateStorageIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("storage index allocation failed: %w", err)
	}
	return v, nil
}

// IsServiceLevelAddress returns true if the given address is the service level address.
// Which means it's not an actual account but instead holds service lever registers.
func IsServiceLevelAddress(address common.Address) bool {
	return address == common.ZeroAddress
}

func PayloadsFromEmulatorSnapshot(snapshotPath string) ([]*ledger.Payload, error) {
	db, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		return nil, err
	}

	payloads, _, _, err := PayloadsAndAccountsFromEmulatorSnapshot(db)
	return payloads, err
}

func PayloadsAndAccountsFromEmulatorSnapshot(db *sql.DB) (
	[]*ledger.Payload,
	map[flow.RegisterID]PayloadMetaInfo,
	[]common.Address,
	error,
) {
	rows, err := db.Query("SELECT key, value, version, height FROM ledger ORDER BY height DESC")
	if err != nil {
		return nil, nil, nil, err
	}

	var payloads []*ledger.Payload
	var accounts []common.Address
	accountsSet := make(map[common.Address]struct{})

	payloadSet := make(map[flow.RegisterID]PayloadMetaInfo)

	for rows.Next() {
		var hexKey, hexValue string
		var height, version uint64

		err := rows.Scan(&hexKey, &hexValue, &height, &version)
		if err != nil {
			return nil, nil, nil, err
		}

		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, nil, nil, err
		}

		value, err := hex.DecodeString(hexValue)
		if err != nil {
			return nil, nil, nil, err
		}

		registerId, address := registerIDKeyFromString(string(key))

		if _, contains := accountsSet[address]; !contains {
			accountsSet[address] = struct{}{}
			accounts = append(accounts, address)
		}

		ledgerKey := convert.RegisterIDToLedgerKey(registerId)

		payload := ledger.NewPayload(
			ledgerKey,
			value,
		)

		if _, ok := payloadSet[registerId]; ok {
			continue
		}

		payloads = append(payloads, payload)
		payloadSet[registerId] = PayloadMetaInfo{
			Height:  height,
			Version: version,
		}
	}

	return payloads, payloadSet, accounts, nil
}

// registerIDKeyFromString is the inverse of `flow.RegisterID.String()` method.
func registerIDKeyFromString(s string) (flow.RegisterID, common.Address) {
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
		},
		address
}

type PayloadMetaInfo struct {
	Height, Version uint64
}

// PayloadsLedger is a simple read/write in-memory atree.Ledger implementation
type PayloadsLedger struct {
	Payloads map[flow.RegisterID]*ledger.Payload

	AllocateStorageIndexFunc func(owner []byte) (atree.StorageIndex, error)
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

func (p *PayloadsLedger) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	if p.AllocateStorageIndexFunc != nil {
		return p.AllocateStorageIndexFunc(owner)
	}

	panic("AllocateStorageIndex not expected to be called")
}
