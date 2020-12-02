package migrations

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const (
	maxKeySize         = 10_000_000
	maxValueSize       = 10_000_000
	maxInteractionSize = 100_000_000
)

func AddMissingKeysMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {
	l := newLed(payloads)

	st := state.NewState(l, maxKeySize, maxValueSize, maxInteractionSize)
	a := state.NewAccounts(st)

	coreContractEncodedKey := "f847b8402b0bf247520770a4bad19e07f6d6b1e8f0542da564154087e2681b175b4432ec2c7b09a52d34dabe0a887ea0f96b067e52c6a0792dcff730fe78a6c5fbbf0a9c02038203e8"

	// Testnet FungibleToken
	err := appendKeyForAccount(a, "9a0766d93b6608b7", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	// Testnet NonFungibleToken
	err = appendKeyForAccount(a, "631e88ae7f1d7c20", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	// Testnet FlowToken
	err = appendKeyForAccount(a, "7e60df042a9c0868", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	// Testnet FlowFees
	err = appendKeyForAccount(a, "912d5440f7e3769e", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	nonCoreContractEncodedKey := "f847b840a272d78cfa14eb248d95c12da8c6a24db9fda5ceddc07444080b49ef6cd15a06e88223af8acd235e2ff7a627adb81cf37d0a1384d9985de4bc7a7fc7eb86848402038203e8"

	// Testnet StakingProxy
	err = appendKeyForAccount(a, "7aad92e5a0715d21", nonCoreContractEncodedKey)

	if err != nil {
		return nil, err
	}

	// Testnet LockedTokens
	err = appendKeyForAccount(a, "95e019a17d0e23d7", nonCoreContractEncodedKey)

	if err != nil {
		return nil, err
	}
	return l.Payloads(), nil
}

func appendKeyForAccount(accounts *state.Accounts, addressInHex string, encodedKeyInHex string) error {
	address := flow.HexToAddress(addressInHex)
	ok, err := accounts.Exists(address)
	if err != nil {
		return err
	}
	if ok {
		accountKeyBytes, err := hex.DecodeString(encodedKeyInHex)
		if err != nil {
			return err
		}
		accountKey, err := flow.DecodeRuntimeAccountPublicKey(accountKeyBytes, 0)
		if err != nil {
			return err
		}
		err = accounts.AppendPublicKey(address, accountKey)
		if err != nil {
			return err
		}
	} else {
		// if not exist log and return gracefully
		fmt.Println("warning account does not exist: ", addressInHex)
	}
	return nil
}

type led struct {
	payloads map[string]ledger.Payload
}

func (l *led) Set(owner, controller, key string, value flow.RegisterValue) error {
	keyparts := []ledger.KeyPart{ledger.NewKeyPart(0, []byte(owner)),
		ledger.NewKeyPart(1, []byte(controller)),
		ledger.NewKeyPart(2, []byte(key))}
	fk := fullKey(owner, controller, key)
	l.payloads[fk] = ledger.Payload{Key: ledger.NewKey(keyparts), Value: ledger.Value(value)}
	return nil
}

func (l *led) Get(owner, controller, key string) (flow.RegisterValue, error) {
	fk := fullKey(owner, controller, key)
	return flow.RegisterValue(l.payloads[fk].Value), nil
}

func (l *led) Delete(owner, controller, key string) {
	fk := fullKey(owner, controller, key)
	delete(l.payloads, fk)
}

func (l *led) Touch(owner, controller, key string) error {
	return nil
}

func (l *led) Payloads() []ledger.Payload {
	ret := make([]ledger.Payload, 0, len(l.payloads))
	for _, v := range l.payloads {
		ret = append(ret, v)
	}
	return ret
}

func newLed(payloads []ledger.Payload) *led {
	mapping := make(map[string]ledger.Payload)
	for _, p := range payloads {
		fk := fullKey(string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[1].Value),
			string(p.Key.KeyParts[2].Value))
		mapping[fk] = p
	}

	return &led{
		payloads: mapping,
	}
}

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
