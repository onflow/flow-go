package migrations

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func AddMissingKeysMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {
	l := newLed(payloads)
	a := state.NewAccounts(l)

	//// TestNet
	// coreContractEncodedKey := "f847b8402b0bf247520770a4bad19e07f6d6b1e8f0542da564154087e2681b175b4432ec2c7b09a52d34dabe0a887ea0f96b067e52c6a0792dcff730fe78a6c5fbbf0a9c02038203e8"

	// // Testnet FungibleToken
	// err := appendKeyForAccount(a, "9a0766d93b6608b7", coreContractEncodedKey)
	// if err != nil {
	// 	return nil, err
	// }

	// // Testnet NonFungibleToken
	// err = appendKeyForAccount(a, "631e88ae7f1d7c20", coreContractEncodedKey)
	// if err != nil {
	// 	return nil, err
	// }

	// // Testnet FlowToken
	// err = appendKeyForAccount(a, "7e60df042a9c0868", coreContractEncodedKey)
	// if err != nil {
	// 	return nil, err
	// }

	// // Testnet FlowFees
	// err = appendKeyForAccount(a, "912d5440f7e3769e", coreContractEncodedKey)
	// if err != nil {
	// 	return nil, err
	// }

	coreContractEncodedKey := "f847b8403588eb28b60e28d24c1e8b03f9a00f73ebd3f6707ee813e27d58ecb6439b8dde1413d7a74a7cc7e8939cbef2e0aa6acc51d5c7010afdb4c6dba55d4cc2ca8bed02018203e8"

	// Mainnet FungibleToken
	err := appendKeyForAccount(a, "f233dcee88fe0abe", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	// Mainnet NonFungibleToken
	err = appendKeyForAccount(a, "1d7e57aa55817448", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	// Mainnet FlowToken
	err = appendKeyForAccount(a, "1654653399040a61", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	// Mainnet FlowFees
	err = appendKeyForAccount(a, "f919ee77447b7497", coreContractEncodedKey)
	if err != nil {
		return nil, err
	}

	//// TestNet
	// nonCoreContractEncodedKey := "f847b840a272d78cfa14eb248d95c12da8c6a24db9fda5ceddc07444080b49ef6cd15a06e88223af8acd235e2ff7a627adb81cf37d0a1384d9985de4bc7a7fc7eb86848402038203e8"

	// // Testnet StakingProxy
	// err = appendKeyForAccount(a, "7aad92e5a0715d21", nonCoreContractEncodedKey)

	// if err != nil {
	// 	return nil, err
	// }

	// // Testnet LockedTokens
	// err = appendKeyForAccount(a, "95e019a17d0e23d7", nonCoreContractEncodedKey)

	// if err != nil {
	// 	return nil, err
	// }

	// Mainnet
	nonCoreContractEncodedKey := "f847b8406e4f43f79d3c1d8cacb3d5f3e7aeedb29feaeb4559fdb71a97e2fd0438565310e87670035d83bc10fe67fe314dba5363c81654595d64884b1ecad1512a64e65e02018203e8"

	// Mainnet StakingProxy
	err = appendKeyForAccount(a, "62430cf28c26d095", nonCoreContractEncodedKey)

	if err != nil {
		return nil, err
	}

	// Mainnet LockedTokens
	err = appendKeyForAccount(a, "8d0e87b65159ae63", nonCoreContractEncodedKey)

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

func (l *led) Set(owner, controller, key string, value flow.RegisterValue) {
	keyparts := []ledger.KeyPart{ledger.NewKeyPart(0, []byte(owner)),
		ledger.NewKeyPart(1, []byte(controller)),
		ledger.NewKeyPart(2, []byte(key))}
	fk := fullKey(owner, controller, key)
	l.payloads[fk] = ledger.Payload{Key: ledger.NewKey(keyparts), Value: ledger.Value(value)}
}

func (l *led) Get(owner, controller, key string) (flow.RegisterValue, error) {
	fk := fullKey(owner, controller, key)
	return flow.RegisterValue(l.payloads[fk].Value), nil
}

func (l *led) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	panic("this method shouldn't be used here")
}

func (l *led) Delete(owner, controller, key string) {
	fk := fullKey(owner, controller, key)
	delete(l.payloads, fk)
}

func (l *led) Touch(owner, controller, key string) {}

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
