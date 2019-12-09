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
		import Vault, createVault, Receiver, Provider from 0x%s

		transaction {

		  prepare(acct: Account) {
			var vaultA: <-Vault? <- createVault(initialBalance: %d)
			
			acct.storage[Vault] <-> vaultA

			acct.published[&Receiver] = &acct.storage[Vault] as Receiver
			acct.published[&Provider] = &acct.storage[Vault] as Provider

			destroy vaultA
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
		import Vault, createVault from 0x%s

		transaction {

		  prepare(acct: Account) {
			let vaultA: <-Vault <- createVault(initialBalance: %d)
    		let vaultB: <-Vault <- createVault(initialBalance: %d)
			let vaultC: <-Vault <- createVault(initialBalance: %d)
			
			var vaultArray: <-[Vault] <- [<-vaultA, <-vaultB]

			vaultArray.append(<-vaultC)
			
			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults
            acct.published[&[Vault]] = &acct.storage[[Vault]] as [Vault] 

			destroy storedVaults
		  }
		}
	`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance, bal2, bal3))
}

// GenerateWithdrawScript creates a script that withdraws
// tokens from a vault and destroys the tokens
func GenerateWithdrawScript(tokenCodeAddr flow.Address, vaultNumber int, withdrawAmount int) []byte {
	template := `
		import Vault from 0x%s

		transaction {
		  prepare(acct: Account) {
			var vaultArray <- acct.storage[[Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[%d].withdraw(amount: %d)

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

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
		import Vault from 0x%s

		transaction {
		  prepare(acct: Account) {
			var vaultArray <- acct.storage[[Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[%d].withdraw(amount: %d)

			vaultArray[%d].deposit(from: <-withdrawVault)

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

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
		import Vault, Provider, Receiver from 0x%s

		transaction {
		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&Provider] ?? panic("missing Vault Provider reference")
			let receiverRef = recipient.published[&Receiver] ?? panic("missing Vault receiver reference")

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
		import Vault, Provider, Receiver from 0x%s

		transaction {

		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&Provider] ?? panic("missing Vault Provider reference")
			let receiverRef = recipient.published[&Receiver] ?? panic("missing Vault receiver reference")

			providerRef.transfer(to: receiverRef, amount: %d)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateInvalidTransferSenderScript creates a script that trys to do a transfer from a receiver reference, which is invalid
func GenerateInvalidTransferSenderScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import Vault, Provider, Receiver from 0x%s

		transaction {
		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&Provider] ?? panic("missing Vault Provider reference")
			let receiverRef = recipient.published[&Receiver] ?? panic("missing Vault receiver reference")

			receiverRef.transfer(to: receiverRef, amount: %d)
		  }
		}
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, receiverAddr, amount))
}

// GenerateInvalidTransferReceiverScript creates a script that trys to do a transfer from a receiver reference, which is invalid
func GenerateInvalidTransferReceiverScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, amount int) []byte {
	template := `
		import Vault, Provider, Receiver from 0x%s

		transaction {

		  prepare(acct: Account) {
			let recipient = getAccount(0x%s)

			let providerRef = acct.published[&Provider] ?? panic("missing Vault Provider reference")
			let receiverRef = recipient.published[&Receiver] ?? panic("missing Vault receiver reference")

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
		import Vault, Receiver from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaultRef = acct.published[&Receiver] ?? panic("missing Receiver reference")
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
		import Vault from 0x%s

		pub fun main() {
			let acct = getAccount(0x%s)
			let vaultArray = acct.published[&[Vault]] ?? panic("missing vault")
			assert(
                vaultArray[%d].balance == %d,
                message: "incorrect Balance!"
            )
        }
	`

	return []byte(fmt.Sprintf(template, tokenCodeAddr, userAddr, vaultNumber, expectedBalance))
}
