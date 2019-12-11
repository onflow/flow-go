package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateTokenScript creates a script that instantiates
// a new Vault instance and stores it in memory.
// balance is an argument to the Vault constructor.
// The Vault must have been deployed already.
func GenerateCreateTokenScript(tokenAddr flow.Address, initialBalance int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {

		  prepare(acct: Account) {
			let oldVault <- acct.storage[FungibleToken.Vault] <- FungibleToken.createVault(initialBalance: %d)
			destroy oldVault

			acct.published[&FungibleToken.Receiver] = &acct.storage[FungibleToken.Vault] as FungibleToken.Receiver
			acct.published[&FungibleToken.Provider] = &acct.storage[FungibleToken.Vault] as FungibleToken.Provider
		  }
		}
	`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance))
}

// GenerateCreateThreeTokensArrayScript creates a script
// that creates three new vault instances, stores them
// in an array of vaults, and then stores the array
// to the storage of the signer's account
func GenerateCreateThreeTokensArrayScript(tokenAddr flow.Address, initialBalance int, bal2 int, bal3 int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {

		  prepare(acct: Account) {
			let vaultA <- FungibleToken.createVault(initialBalance: %d)
    		let vaultB <- FungibleToken.createVault(initialBalance: %d)
			let vaultC <- FungibleToken.createVault(initialBalance: %d)
			
			var vaultArray <- [<-vaultA, <-vaultB]

			vaultArray.append(<-vaultC)

			let storedVaults <- acct.storage[[FungibleToken.Vault]] <- vaultArray
			destroy storedVaults

            acct.published[&[FungibleToken.Vault]] = &acct.storage[[FungibleToken.Vault]] as [FungibleToken.Vault]
		  }
		}
	`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance, bal2, bal3))
}

// GenerateWithdrawScript creates a script that withdraws
// tokens from a vault and destroys the tokens
func GenerateWithdrawScript(tokenCodeAddr flow.Address, vaultNumber int, withdrawAmount int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {
		  prepare(acct: Account) {
			var vaultArray <- acct.storage[[FungibleToken.Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[%d].withdraw(amount: %d)

			var storedVaults: @[FungibleToken.Vault]? <- vaultArray
			acct.storage[[FungibleToken.Vault]] <-> storedVaults

			destroy withdrawVault
			destroy storedVaults
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, vaultNumber, withdrawAmount))
}

// GenerateWithdrawDepositScript creates a script
// that withdraws tokens from a vault and deposits
// them to another vault
func GenerateWithdrawDepositScript(tokenCodeAddr flow.Address, withdrawVaultNumber int, depositVaultNumber int, withdrawAmount int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {
		  prepare(acct: Account) {
			var vaultArray <- acct.storage[[FungibleToken.Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[%d].withdraw(amount: %d)

			vaultArray[%d].deposit(from: <-withdrawVault)

			var storedVaults: @[FungibleToken.Vault]? <- vaultArray
			acct.storage[[FungibleToken.Vault]] <-> storedVaults

			destroy storedVaults
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, withdrawVaultNumber, withdrawAmount, depositVaultNumber))
}

// GenerateDepositVaultScript creates a script that withdraws an tokens from an account
// and deposits it to another account's vault
func GenerateDepositVaultScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {
		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&FungibleToken.Provider] ?? panic("missing Provider reference")
			let receiverRef = recipient.published[&FungibleToken.Receiver] ?? panic("missing Receiver reference")

			let tokens <- providerRef.withdraw(amount: %d)

			receiverRef.deposit(from: <-tokens)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateTransferVaultScript creates a script that withdraws an tokens from an account
// and deposits it to another account's vault
func GenerateTransferVaultScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {

		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&FungibleToken.Provider] ?? panic("missing Provider reference")
			let receiverRef = recipient.published[&FungibleToken.Receiver] ?? panic("missing Receiver reference")

			providerRef.transfer(to: receiverRef, amount: %d)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateInvalidTransferSenderScript creates a script that trys to do a transfer from a receiver reference, which is invalid
func GenerateInvalidTransferSenderScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {
		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&FungibleToken.Provider] ?? panic("missing Provider reference")
			let receiverRef = recipient.published[&FungibleToken.Receiver] ?? panic("missing Receiver reference")

			receiverRef.transfer(to: receiverRef, amount: %d)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateInvalidTransferReceiverScript creates a script that trys to do a transfer from a receiver reference, which is invalid
func GenerateInvalidTransferReceiverScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import FungibleToken from 0x%s

		transaction {

		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&FungibleToken.Provider] ?? panic("missing Provider reference")
			let receiverRef = recipient.published[&FungibleToken.Receiver] ?? panic("missing Receiver reference")

			providerRef.transfer(to: providerRef, amount: %d)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateInspectVaultScript creates a script that retrieves a
// Vault from the array in storage and makes assertions about
// its balance. If these assertions fail, the script panics.
func GenerateInspectVaultScript(tokenCodeAddr, userAddr flow.Address, expectedBalance int) []byte {
	template := `
		import FungibleToken from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaultRef = acct.published[&FungibleToken.Receiver] ?? panic("missing Receiver reference")
			assert(
                vaultRef.balance == %d,
                message: "incorrect Balance!"
            )
		}
    `

	return []byte(fmt.Sprintf(template, tokenCodeAddr, userAddr, expectedBalance))
}

// GenerateInspectVaultArrayScript creates a script that retrieves a
// Vault from the array in storage and makes assertions about
// its balance. If these assertions fail, the script panics.
func GenerateInspectVaultArrayScript(tokenCodeAddr, userAddr flow.Address, vaultNumber int, expectedBalance int) []byte {
	template := `
		import FungibleToken from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaultArray = acct.published[&[FungibleToken.Vault]] ?? panic("missing vault")
			assert(
                vaultArray[%d].balance == %d,
                message: "incorrect Balance!"
            )
        }
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, userAddr, vaultNumber, expectedBalance))
}
