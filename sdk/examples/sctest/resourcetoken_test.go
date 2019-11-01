// Package sctest implements a sample BPL contract and example testing
// code using the emulator test blockchain.
package sctest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	resourceTokenContractFile = "./contracts/session-token.bpl"
)

// Creates a script that instantiates a new Vault instance
// and stores it in memory.
// balance is an argument to the Vault constructor.
// The Vault must have been deployed already.
func generateCreateTokenScript(tokenAddr flow.Address, initialBalance int) []byte {
	template := `
		import Vault, createVault from 0x%s

		fun main(acct: Account) {
			var vaultA: <-Vault? <- createVault(initialBalance: %d)
			
			acct.storage[Vault] <-> vaultA

			destroy vaultA
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance))
}

func generateCreateThreeTokensArrayScript(tokenAddr flow.Address, initialBalance int, bal2 int, bal3 int) []byte {
	template := `
		import Vault, createVault from 0x%s

		fun main(acct: Account) {
			var vaultA: <-Vault <- createVault(initialBalance: %d)
    		var vaultB: <-Vault <- createVault(initialBalance: %d)
			var vaultC: <-Vault <- createVault(initialBalance: %d)
			
			var vaultArray: <-[Vault] <- [<-vaultA, <-vaultB]

			vaultArray.append(<-vaultC)
			
			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

			destroy storedVaults
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance, bal2, bal3))
}

// Creates a script that withdraws tokens from a vault
func generateWithdrawScript(tokenCodeAddr flow.Address, vaultNumber int, withdrawAmount int) []byte {
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
func generateWithdrawDepositScript(tokenCodeAddr flow.Address, withdrawVaultNumber int, depositVaultNumber int, withdrawAmount int) []byte {
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
func generateInspectVaultScript(nftCodeAddr, userAddr flow.Address, vaultNumber int, expectedBalance int) []byte {
	template := `
		import Vault from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let vaultArray <- acct.storage[[Vault]] ?? panic("missing vault")
			if vaultArray[%d].balance != %d {
				log(vaultArray[2].balance)
				panic("incorrect Balance!")
			}

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults
			
			destroy storedVaults
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, vaultNumber, expectedBalance))
}

func TestTokenDeployment(t *testing.T) {
	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := readFile(resourceTokenContractFile)
	_, err := b.CreateAccount(nil, tokenCode, getNonce())
	assert.Nil(t, err)
	b.CommitBlock()
}

func TestCreateToken(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := readFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, getNonce())
	assert.Nil(t, err)

	// Vault must be instantiated with a positive balance
	t.Run("Cannot create token with negative initial balance", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateTokenScript(contractAddr, -7),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, true)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateTokenScript(contractAddr, 10),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, false)
	})

	t.Run("Should be able to create multiple tokens and store them in an array", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, false)
	})
}

func TestTransfers(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := readFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, getNonce())
	assert.Nil(t, err)

	// then deploy the three tokens to an account
	tx := flow.Transaction{
		Script:         generateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5),
		Nonce:          getNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	signAndSubmit(tx, b, t, false)

	t.Run("Should be able to withdraw tokens from a vault", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateWithdrawScript(contractAddr, 0, 3),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, false)

		// Assert that the vaults balance is correct
		_, err = b.ExecuteScript(generateInspectVaultScript(contractAddr, b.RootAccountAddress(), 0, 7))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	t.Run("Should be able to transfer tokens from one vault to another", func(t *testing.T) {

		tx = flow.Transaction{
			Script:         generateWithdrawDepositScript(contractAddr, 1, 2, 8),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, false)

		// Assert that the vault's balance is correct
		_, err = b.ExecuteScript(generateInspectVaultScript(contractAddr, b.RootAccountAddress(), 1, 12))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the vault's balance is correct
		_, err = b.ExecuteScript(generateInspectVaultScript(contractAddr, b.RootAccountAddress(), 2, 13))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})
}
