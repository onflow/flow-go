package migrations

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// BrokenContractMigration fixes some of the early contracts that have been broken due to Cadence upgrades.
func BrokenContractMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {
	l := NewView(payloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	a := state.NewAccounts(sth)

	err := migrateContractForAccount(a, "937cbdee135c656c")
	if err != nil {
		return nil, err
	}

	err = migrateContractForAccount(a, "7f560d7e3ff04a31")
	if err != nil {
		return nil, err
	}

	return l.Payloads(), nil
}

func migrateContractForAccount(accounts *state.Accounts, addressInHex string) error {
	address := flow.HexToAddress(addressInHex)
	ok, err := accounts.Exists(address)
	if err != nil {
		return err
	}
	if ok {
		err = accounts.SetContract("TokenHolderKeyManager", address, GetKeyManagerContractContent())
		if err != nil {
			return err
		}
	} else {
		// if not exist log and return gracefully
		fmt.Println("warning account does not exist: ", addressInHex)
	}
	return nil
}

func GetKeyManagerContractContent() []byte {
	return []byte(`import KeyManager from 0x840b99b76051d886

	// The TokenHolderKeyManager contract is an implementation of
	// the KeyManager interface intended for use by FLOW token holders.
	//
	// One instance is deployed to each token holder account.
	// Deployment is executed a with signature from the administrator,
	// allowing them to take possession of a KeyAdder resource
	// upon initialization.
	pub contract TokenHolderKeyManager: KeyManager {

	access(contract) fun addPublicKey(_ publicKey: [UInt8]) {
		self.account.addPublicKey(publicKey)
	}
	
	pub resource KeyAdder: KeyManager.KeyAdder {

		pub let address: Address

		pub fun addPublicKey(_ publicKey: [UInt8]) {
		TokenHolderKeyManager.addPublicKey(publicKey)
		}

		init(address: Address) {
		self.address = address
		}
	}

	init(admin: AuthAccount, path: StoragePath) {
		let keyAdder <- create KeyAdder(address: self.account.address)

		admin.save(<- keyAdder, to: path)
	}
	}
	`)
}
