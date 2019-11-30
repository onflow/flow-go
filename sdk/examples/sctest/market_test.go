package sctest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

const (
	MarketContractFile = "./contracts/market.cdc"
)

func TestMarketDeployment(t *testing.T) {
	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	tokenCode := ReadFile(resourceTokenContractFile)
	_, err := b.CreateAccount(nil, tokenCode, GetNonce())
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	b.CommitBlock()

	nftCode := ReadFile(NFTContractFile)
	_, err = b.CreateAccount(nil, nftCode, GetNonce())
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	b.CommitBlock()

	// Should be able to deploy a contract as a new account with no keys.
	marketCode := ReadFile(MarketContractFile)
	_, err = b.CreateAccount(nil, marketCode, GetNonce())
	if !assert.Nil(t, err) {
		t.Log(err.Error())
	}
	b.CommitBlock()
}

func TestCreateSale(t *testing.T) {
	b := newEmulator()

	// first deploy the FT, NFT, and market code
	tokenCode := ReadFile(resourceTokenContractFile)
	tokenAddr, err := b.CreateAccount(nil, tokenCode, GetNonce())
	assert.Nil(t, err)
	b.CommitBlock()

	nftCode := ReadFile(NFTContractFile)
	nftAddr, err := b.CreateAccount(nil, nftCode, GetNonce())
	assert.Nil(t, err)
	b.CommitBlock()

	marketCode := ReadFile(MarketContractFile)
	marketAddr, err := b.CreateAccount(nil, marketCode, GetNonce())
	assert.Nil(t, err)
	b.CommitBlock()

	// create two new accounts
	bastianPrivateKey := randomKey()
	bastianPublicKey := bastianPrivateKey.PublicKey(keys.PublicKeyWeightThreshold)
	bastianAddress, err := b.CreateAccount([]flow.AccountPublicKey{bastianPublicKey}, nil, GetNonce())
	joshPrivateKey := randomKey()
	joshPublicKey := joshPrivateKey.PublicKey(keys.PublicKeyWeightThreshold)
	joshAddress, err := b.CreateAccount([]flow.AccountPublicKey{joshPublicKey}, nil, GetNonce())

	t.Run("Should be able to create FTs and NFT collections in each accounts storage", func(t *testing.T) {
		// create Fungible tokens and NFTs in each accounts storage and store references
		setupUsersTokens(t, b, tokenAddr, nftAddr, []flow.AccountPrivateKey{bastianPrivateKey, joshPrivateKey}, []flow.Address{bastianAddress, joshAddress})
	})

	t.Run("Can create sale collection", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateSaleScript(tokenAddr, marketAddr),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)
	})

	t.Run("Can put an NFT up for sale", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateStartSaleScript(nftAddr, marketAddr, 1, 10),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{bastianAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), bastianPrivateKey}, []flow.Address{b.RootAccountAddress(), bastianAddress}, false)
	})

	t.Run("Cannot buy an NFT for less than the sale price", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 1, 9),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{joshAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), joshPrivateKey}, []flow.Address{b.RootAccountAddress(), joshAddress}, true)
	})

	t.Run("Cannot buy an NFT that is not for sale", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 2, 10),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{joshAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), joshPrivateKey}, []flow.Address{b.RootAccountAddress(), joshAddress}, true)
	})

	t.Run("Can buy an NFT that is for sale", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, bastianAddress, 1, 10),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{joshAddress},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), joshPrivateKey}, []flow.Address{b.RootAccountAddress(), joshAddress}, false)

		_, _, err = b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, bastianAddress, 40))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, _, err = b.ExecuteScript(GenerateInspectVaultScript(tokenAddr, joshAddress, 20))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

		// Assert that the accounts' collections are correct
		_, _, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, bastianAddress, 1, false))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, _, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, bastianAddress, 2, false))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, _, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, joshAddress, 1, true))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		_, _, err = b.ExecuteScript(GenerateInspectCollectionScript(nftAddr, joshAddress, 2, true))
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}

	})

}
