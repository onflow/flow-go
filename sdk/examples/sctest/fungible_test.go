package sctest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"
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

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateTokenScript(contractAddr, 10),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 10))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	t.Run("Should be able to create multiple tokens and store them in an array", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateThreeTokensArrayScript(contractAddr, 10, 20, 5),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)
	})
}

func TestInAccountTransfers(t *testing.T) {
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

	SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	t.Run("Should be able to withdraw tokens from a vault", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateWithdrawScript(contractAddr, 0, 3),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

		// Assert that the vaults balance is correct
		_, err = b.ExecuteScript(GenerateInspectVaultArrayScript(contractAddr, b.RootAccountAddress(), 0, 7))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	t.Run("Should be able to withdraw and deposit tokens from one vault to another in an account", func(t *testing.T) {

		tx = flow.Transaction{
			Script:         GenerateWithdrawDepositScript(contractAddr, 1, 2, 8),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

		// Assert that the vault's balance is correct
		_, err = b.ExecuteScript(GenerateInspectVaultArrayScript(contractAddr, b.RootAccountAddress(), 1, 12))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the vault's balance is correct
		_, err = b.ExecuteScript(GenerateInspectVaultArrayScript(contractAddr, b.RootAccountAddress(), 2, 13))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})
}

func TestExternalTransfers(t *testing.T) {
	b := newEmulator()

	// First, deploy the token contract
	tokenCode := ReadFile(resourceTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)

	// then deploy the tokens to an account
	tx := flow.Transaction{
		Script:         GenerateCreateTokenScript(contractAddr, 10),
		Nonce:          GetNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	// create a new account
	bastianPrivateKey := randomKey()
	bastianPublicKey := bastianPrivateKey.PublicKey(keys.PublicKeyWeightThreshold)
	bastianAddress, err := b.CreateAccount([]flow.AccountPublicKey{bastianPublicKey}, nil, GetNonce())

	// then deploy the tokens to the new account
	tx = flow.Transaction{
		Script:         GenerateCreateTokenScript(contractAddr, 10),
		Nonce:          GetNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{bastianAddress},
	}

	SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)

	t.Run("Should be able to withdraw and deposit tokens from a vault", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateDepositVaultScript(contractAddr, bastianAddress, 3),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

		// Assert that the vaults' balances are correct
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 7))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, bastianAddress, 13))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	t.Run("Should fail when trying to call functions that are not exposed with the interface", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateInvalidTransferSenderScript(contractAddr, bastianAddress, 3),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)

		tx = flow.Transaction{
			Script:         GenerateInvalidTransferReceiverScript(contractAddr, bastianAddress, 3),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)

	})

	t.Run("Should fail when trying to transfer a negative amount", func(t *testing.T) {

		tx = flow.Transaction{
			Script:         GenerateTransferVaultScript(contractAddr, b.RootAccountAddress(), -7),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, true)

	})

	t.Run("Should fail when trying to transfer an amount that is greater than the account's balance", func(t *testing.T) {

		tx = flow.Transaction{
			Script:         GenerateTransferVaultScript(contractAddr, b.RootAccountAddress(), 30),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, true)

		// Assert that the vaults' balances have not changed after all the fails
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 7))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, bastianAddress, 13))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

	})

	t.Run("Should be able to transfer tokens from one vault to another", func(t *testing.T) {

		tx = flow.Transaction{
			Script:         GenerateTransferVaultScript(contractAddr, b.RootAccountAddress(), 7),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)

		// Assert that the vaults' balances are correct
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, b.RootAccountAddress(), 14))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, err = b.ExecuteScript(GenerateInspectVaultScript(contractAddr, bastianAddress, 6))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})
}
