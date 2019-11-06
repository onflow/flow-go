package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Creates a script that instantiates a new Vault instance
// and stores it in memory.
// balance is an argument to the Vault constructor.
// The Vault must have been deployed already.
func GenerateCreateTokenScript(tokenAddr flow.Address, initialBalance int) []byte {
	template := `
		import Vault, createVault from 0x%s

		fun main(acct: Account) {
			var vaultA: <-Vault? <- createVault(initialBalance: %d)
			
			acct.storage[Vault] <-> vaultA

			destroy vaultA
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance))
}

func GenerateCreateThreeTokensArrayScript(tokenAddr flow.Address, initialBalance int, bal2 int, bal3 int) []byte {
	template := `
		import Vault, createVault from 0x%s

		fun main(acct: Account) {
			let vaultA: <-Vault <- createVault(initialBalance: %d)
    		let vaultB: <-Vault <- createVault(initialBalance: %d)
			let vaultC: <-Vault <- createVault(initialBalance: %d)
			
			var vaultArray: <-[Vault] <- [<-vaultA, <-vaultB]

			vaultArray.append(<-vaultC)
			
			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

			destroy storedVaults
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance, bal2, bal3))
}

// Creates a script that withdraws tokens from a vault
func GenerateWithdrawScript(tokenCodeAddr flow.Address, vaultNumber int, withdrawAmount int) []byte {
	template := `
		import Vault from 0x%s

		fun main(acct: Account) {
			var vaultArray <- acct.storage[[Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[%d].withdraw(amount: %d)

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

			destroy withdrawVault
			destroy storedVaults
		}`

	return []byte(fmt.Sprintf(template, tokenCodeAddr.String(), vaultNumber, withdrawAmount))
}

// Creates a script that withdraws tokens from a vault
// and deposits them to another vault
func GenerateWithdrawDepositScript(tokenCodeAddr flow.Address, withdrawVaultNumber int, depositVaultNumber int, withdrawAmount int) []byte {
	template := `
		import Vault from 0x%s

		fun main(acct: Account) {
			var vaultArray <- acct.storage[[Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[%d].withdraw(amount: %d)

			vaultArray[%d].deposit(from: <-withdrawVault)

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

			destroy storedVaults
		}`

	return []byte(fmt.Sprintf(template, tokenCodeAddr.String(), withdrawVaultNumber, withdrawAmount, depositVaultNumber))
}

// Creates a script that retrieves an Vault from storage and makes assertions
// about its properties. If these assertions fail, the script panics.
func GenerateInspectVaultScript(nftCodeAddr, userAddr flow.Address, vaultNumber int, expectedBalance int) []byte {
	template := `
		import Vault from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let vaultArray <- acct.storage[[Vault]] ?? panic("missing vault")
			if vaultArray[%d].balance != %d {
				panic("incorrect Balance!")
			}

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults
			
			destroy storedVaults
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, vaultNumber, expectedBalance))
}
