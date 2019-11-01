// Package sctest implements a sample BPL contract and example testing
// code using the emulator test blockchain.
package sctest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
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

func generateCreateThreeTokensArrayScript(tokenAddr flow.Address, initialBalance int) []byte {
	template := `
		import Vault, createVault from 0x%s

		fun main(acct: Account) {
			var vaultA: <-Vault <- createVault(initialBalance: %d)
    		var vaultB: <-Vault <- createVault(initialBalance: 0)
			var vaultC: <-Vault <- createVault(initialBalance: 5)
			
			var vaultArray: <-[Vault] <- [<-vaultA, <-vaultB]

			vaultArray.append(<-vaultC)
			
			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

			destroy storedVaults
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, initialBalance))
}

// Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func generateWithdrawScript(tokenCodeAddr flow.Address, withdrawAmount int) []byte {
	template := `
		import Vault from 0x%s

		fun main(acct: Account) {
			var vaultArray <- acct.storage[[Vault]] ?? panic("missing vault array!")
			
			let withdrawVault <- vaultArray[0].withdraw(amount: %d)

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults

			destroy withdrawVault
			destroy storedVaults
		}`

	return []byte(fmt.Sprintf(template, tokenCodeAddr.String(), withdrawAmount))
}

// Creates a script that retrieves an Vault from storage and makes assertions
// about its properties. If these assertions fail, the script panics.
func generateInspectVaultScript(nftCodeAddr, userAddr flow.Address, expectedBalance int) []byte {
	template := `
		import Vault from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let vaultArray <- acct.storage[[Vault]] ?? panic("missing vault")
			if vaultArray[0].balance != %d {
				panic("incorrect id")
			}

			var storedVaults: <-[Vault]? <- vaultArray
			acct.storage[[Vault]] <-> storedVaults
			
			destroy storedVaults
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, expectedBalance))
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
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		assert.Nil(t, err)
		err = b.SubmitTransaction(&tx)

		if assert.Error(t, err) {
			assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
		}
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateTokenScript(contractAddr, 10),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		assert.Nil(t, err)
		err = b.SubmitTransaction(&tx)

		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		b.CommitBlock()
	})

	t.Run("Should be able to create multiple tokens and store them in an array", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateThreeTokensArrayScript(contractAddr, 10),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		assert.Nil(t, err)
		err = b.SubmitTransaction(&tx)

		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		b.CommitBlock()
	})
}

func TestWithdraw(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := readFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, getNonce())
	assert.Nil(t, err)

	// then deploy the three tokens to an account
	tx := flow.Transaction{
		Script:         generateCreateThreeTokensArrayScript(contractAddr, 10),
		Nonce:          getNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
	assert.Nil(t, err)
	err = b.SubmitTransaction(&tx)

	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	b.CommitBlock()

	t.Run("Should be able to withdraw tokens from a vault", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateWithdrawScript(contractAddr, 3),
			Nonce:          getNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		assert.Nil(t, err)
		err = b.SubmitTransaction(&tx)

		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		b.CommitBlock()

		// Assert that ID/specialness are correct
		_, err = b.ExecuteScript(generateInspectVaultScript(contractAddr, b.RootAccountAddress(), 7))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})
}
