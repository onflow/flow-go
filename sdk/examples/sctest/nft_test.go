package sctest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

const (
	NFTContractFile = "./contracts/nft.cdc"
)

func TestNFTDeployment(t *testing.T) {
	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(NFTContractFile)
	_, err := b.CreateAccount(nil, tokenCode, GetNonce())
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	b.CommitBlock()
}

func TestCreateNFT(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(NFTContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)

	// Vault must be instantiated with a positive ID
	t.Run("Cannot create token with negative ID", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateNFTScript(contractAddr, -7, -7),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateNFTScript(contractAddr, 1, 1),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)
	})

	// Assert that the account's collection is correct
	_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, "true"))
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}

	// Assert that the account's collection doesn't contain ID 3
	_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 3, "true"))
	assert.Error(t, err)

}

func TestTransferNFT(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	tokenCode := ReadFile(NFTContractFile)
	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)

	// then deploy a NFT to the root account
	tx := flow.Transaction{
		Script:         GenerateCreateNFTScript(contractAddr, 1, 1),
		Nonce:          GetNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	// Assert that the account's collection is correct
	_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, "true"))
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}

	// create a new account
	bastianPrivateKey := randomKey()
	bastianPublicKey := bastianPrivateKey.PublicKey(keys.PublicKeyWeightThreshold)
	bastianAddress, err := b.CreateAccount([]flow.AccountPublicKey{bastianPublicKey}, nil, GetNonce())

	// then deploy an NFT to another account
	tx = flow.Transaction{
		Script:         GenerateCreateNFTScript(contractAddr, 2, 2),
		Nonce:          GetNonce(),
		ComputeLimit:   20,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{bastianAddress},
	}

	SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)

	// transfer an NFT
	t.Run("Should be able to withdraw an NFT and deposit to another accounts collection", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateDepositScript(contractAddr, bastianAddress, 1),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, bastianAddress, 1, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, bastianAddress, 1, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, bastianAddress, 2, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, bastianAddress, 2, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, b.RootAccountAddress(), 1, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, b.RootAccountAddress(), 2, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 2, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		_, err = b.ExecuteScript(GenerateInspectCollectionArrayScript(contractAddr, bastianAddress))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	// transfer an NFT
	t.Run("Should be able to transfer an NFT to another accounts collection", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateTransferScript(contractAddr, b.RootAccountAddress(), 1),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, bastianAddress, 2, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, bastianAddress, 1, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, b.RootAccountAddress(), 2, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})

	// transfer an NFT
	t.Run("Should fail when trying to transfer a token that doesn't exist", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateTransferScript(contractAddr, b.RootAccountAddress(), 5),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, true)

		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, bastianAddress, 2, "true"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, bastianAddress, 1, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		// Assert that the account's collection is correct
		_, err = b.ExecuteScript(GenerateInspectCollectionDictionaryScript(contractAddr, b.RootAccountAddress(), 2, "false"))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
	})
}
