package sctest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	greatTokenContractFile = "./contracts/great-token.cdc"
)

func TestDeployment(t *testing.T) {
	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	nftCode := ReadFile(greatTokenContractFile)
	_, err := b.CreateAccount(nil, nftCode, GetNonce())
	assert.Nil(t, err)
	b.CommitBlock()
}

func TestCreateMinter(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	nftCode := ReadFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode, GetNonce())
	assert.Nil(t, err)

	// GreatNFTMinter must be instantiated with initialID > 0 and
	// specialMod > 1
	t.Run("Cannot create minter with negative initial ID", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateMinterScript(contractAddr, -1, 2),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)
	})

	t.Run("Cannot create minter with special mod < 2", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateMinterScript(contractAddr, 1, 1),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, true)
	})

	t.Run("Should be able to create minter", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         GenerateCreateMinterScript(contractAddr, 1, 2),
			Nonce:          GetNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)
	})
}

func TestMinting(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	nftCode := ReadFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode, GetNonce())
	assert.Nil(t, err)

	// Next, instantiate the minter
	createMinterTx := flow.Transaction{
		Script:         GenerateCreateMinterScript(contractAddr, 1, 2),
		Nonce:          GetNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	SignAndSubmit(createMinterTx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	// Mint the first NFT
	mintTx := flow.Transaction{
		Script:         GenerateMintScript(contractAddr),
		Nonce:          GetNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	SignAndSubmit(mintTx, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	// Assert that ID/specialness are correct
	_, err = b.ExecuteScript(GenerateInspectNFTScript(contractAddr, b.RootAccountAddress(), 1, false))
	assert.Nil(t, err)

	// Mint a second NF
	mintTx2 := flow.Transaction{
		Script:         GenerateMintScript(contractAddr),
		Nonce:          GetNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	SignAndSubmit(mintTx2, b, t, []flow.AccountPrivateKey{b.RootKey()}, []flow.Address{b.RootAccountAddress()}, false)

	// Assert that ID/specialness are correct
	_, err = b.ExecuteScript(GenerateInspectNFTScript(contractAddr, b.RootAccountAddress(), 2, true))
	assert.Nil(t, err)
}
