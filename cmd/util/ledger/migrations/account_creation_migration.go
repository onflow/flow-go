package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// AccountCreationMigration moves account restriction logic from fvm code to the service account
func AccountCreationMigration(payloads []ledger.Payload) ([]ledger.Payload, error) {
	l := NewView(payloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	a := state.NewAccounts(sth)

	address := "e467b9dd11fa00df"
	err := migrateContractForAccount(a, address)
	if err != nil {
		return nil, err
	}

	key := "/storage/isAccountCreationRestricted"
	v := GetSetBooleanPayload(address, "", key)
	l.Set(address, "", key, v.Value)

	return l.Payloads(), nil
}

func migrateContractForAccount(accounts *state.Accounts, addressInHex string) error {
	address := flow.HexToAddress(addressInHex)
	ok, err := accounts.Exists(address)
	if err != nil {
		return err
	}
	if ok {
		err = accounts.SetContract("FlowServiceAccount", address, AccountCreationContractContent())
		if err != nil {
			return err
		}
	} else {
		// if not exist log and return gracefully
		fmt.Println("warning account does not exist: ", addressInHex)
	}
	return nil
}

func AccountCreationContractContent() []byte {
	return []byte(`import FungibleToken from 0xf233dcee88fe0abe
	import FlowToken from 0x1654653399040a61
	import FlowFees from 0xf919ee77447b7497
	import FlowStorageFees from 0xe467b9dd11fa00df
	
	pub contract FlowServiceAccount {
	
		pub event TransactionFeeUpdated(newFee: UFix64)
	
		pub event AccountCreationFeeUpdated(newFee: UFix64)
	
		pub event AccountCreatorAdded(accountCreator: Address)
	
		pub event AccountCreatorRemoved(accountCreator: Address)
	
		pub event IsAccountCreationRestrictedUpdated(isRestricted: Bool)
	
		/// A fixed-rate fee charged to execute a transaction
		pub var transactionFee: UFix64
	
		/// A fixed-rate fee charged to create a new account
		pub var accountCreationFee: UFix64
	
		/// The list of account addresses that have permission to create accounts
		access(contract) var accountCreators: {Address: Bool}
	
		/// Initialize an account with a FlowToken Vault and publish capabilities.
		pub fun initDefaultToken(_ acct: AuthAccount) {
			// Create a new FlowToken Vault and save it in storage
			acct.save(<-FlowToken.createEmptyVault(), to: /storage/flowTokenVault)
	
			// Create a public capability to the Vault that only exposes
			// the deposit function through the Receiver interface
			acct.link<&FlowToken.Vault{FungibleToken.Receiver}>(
				/public/flowTokenReceiver,
				target: /storage/flowTokenVault
			)
	
			// Create a public capability to the Vault that only exposes
			// the balance field through the Balance interface
			acct.link<&FlowToken.Vault{FungibleToken.Balance}>(
				/public/flowTokenBalance,
				target: /storage/flowTokenVault
			)
		}
	
		/// Get the default token balance on an account
		pub fun defaultTokenBalance(_ acct: PublicAccount): UFix64 {
			let balanceRef = acct
				.getCapability(/public/flowTokenBalance)
				.borrow<&FlowToken.Vault{FungibleToken.Balance}>()!
	
			return balanceRef.balance
		}
	
		/// Return a reference to the default token vault on an account
		pub fun defaultTokenVault(_ acct: AuthAccount): &FlowToken.Vault {
			return acct.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
				?? panic("Unable to borrow reference to the default token vault")
		}
	
		/// Called when a transaction is submitted to deduct the fee
		/// from the AuthAccount that submitted it
		pub fun deductTransactionFee(_ acct: AuthAccount) {
			if self.transactionFee == UFix64(0) {
				return
			}
	
			let tokenVault = self.defaultTokenVault(acct)
			let feeVault <- tokenVault.withdraw(amount: self.transactionFee)
	
			FlowFees.deposit(from: <-feeVault)
		}
	
		/// - Deducts the account creation fee from a payer account.
		/// - Inits the default token.
		/// - Inits account storage capacity.
		pub fun setupNewAccount(newAccount: AuthAccount, payer: AuthAccount) {
			if !FlowServiceAccount.isAccountCreator(payer.address) {
				panic("Account not authorized to create accounts")
			}
	
	
			if self.accountCreationFee < FlowStorageFees.minimumStorageReservation {
				panic("Account creation fees setup incorrectly")
			}
	
			let tokenVault = self.defaultTokenVault(payer)
			let feeVault <- tokenVault.withdraw(amount: self.accountCreationFee)
			let storageFeeVault <- (feeVault.withdraw(amount: FlowStorageFees.minimumStorageReservation) as! @FlowToken.Vault)
			FlowFees.deposit(from: <-feeVault)
	
			FlowServiceAccount.initDefaultToken(newAccount)
	
			let vaultRef = FlowServiceAccount.defaultTokenVault(newAccount)
	
			vaultRef.deposit(from: <-storageFeeVault)
		}
	
		/// Returns true if the given address is permitted to create accounts, false otherwise
		pub fun isAccountCreator(_ address: Address): Bool {
			// If account creation is not restricted, then anyone can create an account
			if !self.isAccountCreationRestricted() {
				return true
			}
			return self.accountCreators[address] ?? false
		}
	
		/// Is true if new acconts can only be created by approved accounts 'self.accountCreators'
		pub fun isAccountCreationRestricted(): Bool {
			return self.account.copy<Bool>(from: /storage/isAccountCreationRestricted) ?? false
		}
	
		// Authorization resource to change the fields of the contract
		/// Returns all addresses permitted to create accounts
		pub fun getAccountCreators(): [Address] {
			return self.accountCreators.keys
		}
	
		/// Authorization resource to change the fields of the contract
		pub resource Administrator {
	
			/// Sets the transaction fee
			pub fun setTransactionFee(_ newFee: UFix64) {
				FlowServiceAccount.transactionFee = newFee
				emit TransactionFeeUpdated(newFee: newFee)
			}
	
			/// Sets the account creation fee
			pub fun setAccountCreationFee(_ newFee: UFix64) {
				FlowServiceAccount.accountCreationFee = newFee
				emit AccountCreationFeeUpdated(newFee: newFee)
			}
	
			/// Adds an account address as an authorized account creator
			pub fun addAccountCreator(_ accountCreator: Address) {
				FlowServiceAccount.accountCreators[accountCreator] = true
				emit AccountCreatorAdded(accountCreator: accountCreator)
			}
	
			/// Removes an account address as an authorized account creator
			pub fun removeAccountCreator(_ accountCreator: Address) {
				FlowServiceAccount.accountCreators.remove(key: accountCreator)
				emit AccountCreatorRemoved(accountCreator: accountCreator)
			}
	
			 pub fun setIsAccountCreationRestricted(_ enabled: Bool) {
				let path = /storage/isAccountCreationRestricted
				let oldValue = FlowServiceAccount.account.load<Bool>(from: path)
				FlowServiceAccount.account.save<Bool>(enabled, to: path)
				if enabled != oldValue {
					emit IsAccountCreationRestrictedUpdated(isRestricted: enabled)
				}
			}
		}
	
		init() {
			self.transactionFee = 0.0
			self.accountCreationFee = 0.0
	
			self.accountCreators = {}
	
			let admin <- create Administrator()
			admin.addAccountCreator(self.account.address)
	
			self.account.save(<-admin, to: /storage/flowServiceAdmin)
		}
	}
	`)
}

func GetSetBooleanPayload(owner, controller, key string) *ledger.Payload {
	value := interpreter.BoolValue(true)

	v := interpreter.PrependMagic(
		value,
		interpreter.CurrentEncodingVersion,
	)
	k := registerIDToKey(flow.RegisterID{Owner: owner, Controller: controller, Key: key})
	return ledger.NewPayload(k, v)
}
