package migrations

import (
	"fmt"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func AddMissingKeysMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {
	view := NewView(payloads)
	txState := state.NewTransactionState(view, state.DefaultParameters())
	accounts := environment.NewAccounts(txState)

	// get the key from the canary service account
	serviceAddressHex := "f8d6e0586b0a20c7"
	serviceAddress := flow.HexToAddress(serviceAddressHex)
	ok, err := accounts.Exists(serviceAddress)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("service account does not exist: %s", serviceAddress)
	}

	keys, err := accounts.GetPublicKeys(serviceAddress)
	if err != nil {
		return nil, err
	}
	if len(keys) != 1 {
		return nil, fmt.Errorf("expected 1 key for service account, got: %d", len(keys))
	}
	key := keys[0]

	// copy the key to other core contract accounts

	// Canary FlowToken
	err = appendKeyForAccount(accounts, "e5a8b7f23e8b548f", key)
	if err != nil {
		return nil, err
	}

	// Canary FlowFees
	err = appendKeyForAccount(accounts, "0ae53cb6e3f42a79", key)
	if err != nil {
		return nil, err
	}

	// Canary FungibleToken
	err = appendKeyForAccount(accounts, "ee82856bf20e2aa6", key)
	if err != nil {
		return nil, err
	}

	return view.Payloads(), nil
}

func appendKeyForAccount(accounts *environment.StatefulAccounts, addressInHex string, accountKey flow.AccountPublicKey) error {
	address := flow.HexToAddress(addressInHex)
	ok, err := accounts.Exists(address)
	if err != nil {
		return err
	}
	if ok {
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
