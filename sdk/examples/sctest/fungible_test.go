package sctest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	resourceTokenContractFile = "./contracts/fungible-token.cdc"
)

func TestTokenDeployment(t *testing.T) {
	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(resourceTokenContractFile)
	_, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)
	b.CommitBlock()
}

func TestCreateToken(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)

	// Vault must be instantiated with a positive balance
	t.Run("Cannot create token with negative initial balance", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateTokenScript(contractAddr, -7),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, true)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateTokenScript(contractAddr, 10),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, false)
	})

	t.Run("Should be able to create multiple tokens and store them in an array", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, false)
	})
}

func TestTransfers(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)

	// then deploy the three tokens to an account
	tx := flow.Transaction{
		Script:         GenerateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5),
		Nonce:          GetNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	SignAndSubmit(tx, b, t, false)

	t.Run("Should be able to withdraw tokens from a vault", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateWithdrawScript(contractAddr, 0, 3),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, false)

		// Assert that the vaults balance is correct
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 0, 7))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	t.Run("Should be able to transfer tokens from one vault to another", func(t *testing.T) {

		tx = flow.Transaction{
			Script:         GenerateWithdrawDepositScript(contractAddr, 1, 2, 8),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, false)

		// Assert that the vault's balance is correct
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 1, 12))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the vault's balance is correct
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 2, 13))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})
}
