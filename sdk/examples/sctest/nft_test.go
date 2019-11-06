package sctest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
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
			Script:         GenerateCreateNFTScript(contractAddr, -7),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, true)
	})

	t.Run("Should be able to create token", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateNFTScript(contractAddr, 1),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, false)
	})

	// Assert that the account's collection is correct
	// _, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 0))
	// if !assert.Nil(t, err) {
	// 	t.Log(err.Error())
	// }

}

// func TestTransferNFT(t *testing.T) {
// 	b := newEmulator()

// 	// First, deploy the contract
// 	tokenCode := ReadFile(NFTContractFile)
// 	contractAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
// 	assert.Nil(t, err)

// 	// then deploy a NFT to the root account
// 	tx := flow.Transaction{
// 		Script:         GenerateCreateNFTScript(contractAddr, 1),
// 		Nonce:          GetNonce(),
// 		ComputeLimit:   20,
// 		PayerAccount:   b.RootAccountAddress(),
// 		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
// 	}

// 	SignAndSubmit(tx, b, t, false)

// 	// Assert that the account's collection is correct
// 	_, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, b.RootAccountAddress(), 1))
// 	if !assert.Nil(t, err) {
// 		t.Log(err.Error())
// 	}

// 	// create a new account
// 	bastianPrivateKey := randomKey()
// 	bastianPublicKey := bastianPrivateKey.PublicKey(keys.PublicKeyWeightThreshold)
// 	bastianAddress, err := b.CreateAccount([]flow.AccountPublicKey{bastianPublicKey}, nil, GetNonce())

// 	// then deploy an NFT to another account
// 	tx = flow.Transaction{
// 		Script:         GenerateCreateNFTScript(contractAddr, 2),
// 		Nonce:          GetNonce(),
// 		ComputeLimit:   20,
// 		PayerAccount:   b.RootAccountAddress(),
// 		ScriptAccounts: []flow.Address{bastianAddress}, //[]flow.Address{b.RootAccountAddress()},
// 	}

// 	sig1, err := keys.SignTransaction(tx, b.RootKey())
// 	assert.Nil(t, err)

// 	sig2, err := keys.SignTransaction(tx, bastianPrivateKey)
// 	assert.Nil(t, err)

// 	tx.AddSignature(b.RootAccountAddress(), sig1)
// 	tx.AddSignature(bastianAddress, sig2)

// 	err = b.SubmitTransaction(tx)

// transfer an NFT
// t.Run("Should be able to transfer an NFT to another accounts collection", func(t *testing.T) {
// 	tx := flow.Transaction{
// 		Script:         GenerateTransferScript(contractAddr, bastianAddress, 1),
// 		Nonce:          GetNonce(),
// 		ComputeLimit:   20,
// 		PayerAccount:   b.RootAccountAddress(),
// 		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
// 	}

// 	SignAndSubmit(tx, b, t, false)
// })

// transfer an NFT
// t.Run("Should be able to withdraw an NFT and deposit to another accounts collection", func(t *testing.T) {
// 	tx := flow.Transaction{
// 		Script:         GenerateDepositScript(contractAddr, bastianAddress, 1),
// 		Nonce:          GetNonce(),
// 		ComputeLimit:   20,
// 		PayerAccount:   b.RootAccountAddress(),
// 		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
// 	}

// SignAndSubmit(tx, b, t, false)

// // Assert that the account's collection is correct
// _, err = b.ExecuteScript(GenerateInspectCollectionScript(contractAddr, bastianAddress, 1))
// if !assert.Nil(t, err) {
// 	t.Log(err.Error())
// }
// })

// 	t.Run("Should be able to transfer tokens from one vault to another", func(t *testing.T) {

// 		tx = flow.Transaction{
// 			Script:         generateWithdrawDepositScript(contractAddr, 1, 2, 8),
// 			Nonce:          GetNonce(),
// 			ComputeLimit:   20,
// 			PayerAccount:   b.RootAccountAddress(),
// 			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
// 		}

// 		SignAndSubmit(tx, b, t, false)

// 		// Assert that the vault's balance is correct
// 		_, err = b.ExecuteScript(generateInspectVaultScript(contractAddr, b.RootAccountAddress(), 1, 12))
// 		if !assert.Nil(t, err) {
// 			t.Log(err.Error())
// 		}

// 		// Assert that the vault's balance is correct
// 		_, err = b.ExecuteScript(generateInspectVaultScript(contractAddr, b.RootAccountAddress(), 2, 13))
// 		if !assert.Nil(t, err) {
// 			t.Log(err.Error())
// 		}
// 	})
// }
