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
	greatTokenContractFile = "./contracts/great-token.bpl"
)

// Creates a script that instantiates a new GreatNFTMinter instance
// and stores it in memory.
// Initial ID and special mod are arguments to the GreatNFTMinter constructor.
// The GreatNFTMinter must have been deployed already.
func generateCreateMinterScript(nftAddr flow.Address, initialID, specialMod int) []byte {
	template := `
		import GreatNFTMinter from 0x%s

		fun main(acct: Account) {
			let minter = GreatNFTMinter(firstID: %d, specialMod: %d)
			acct.storage[GreatNFTMinter] = minter
		}`
	return []byte(fmt.Sprintf(template, nftAddr, initialID, specialMod))
}

// Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func generateMintScript(nftCodeAddr flow.Address) []byte {
	template := `
		import GreatNFTMinter, GreatNFT from 0x%s

		fun main(acct: Account) {
			let minter = acct.storage[GreatNFTMinter] ?? panic("missing minter")
			let nft = minter.mint()

			acct.storage[GreatNFT] = nft
			acct.storage[GreatNFTMinter] = minter
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr.String()))
}

// Creates a script that retrieves an NFT from storage and makes assertions
// about its properties. If these assertions fail, the script panics.
func generateInspectNFTScript(nftCodeAddr, userAddr flow.Address, expectedID int, expectedIsSpecial bool) []byte {
	template := `
		import GreatNFT from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let nft = acct.storage[GreatNFT] ?? panic("missing nft")
			if nft.id() != %d {
				panic("incorrect id")
			}
			if nft.isSpecial() != %t {
				panic("incorrect specialness")
			}
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, expectedID, expectedIsSpecial))
}

func TestDeployment(t *testing.T) {
	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	nftCode := readFile(greatTokenContractFile)
	_, err := b.CreateAccount(nil, nftCode, getNonce())
	assert.Nil(t, err)
	b.CommitBlock()
}

func TestCreateMinter(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	nftCode := readFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode, getNonce())
	assert.Nil(t, err)

	// GreatNFTMinter must be instantiated with initialID > 0 and
	// specialMod > 1
	t.Run("Cannot create minter with negative initial ID", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateMinterScript(contractAddr, -1, 2),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, true)
	})

	t.Run("Cannot create minter with special mod < 2", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateMinterScript(contractAddr, 1, 1),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, true)
	})

	t.Run("Should be able to create minter", func(t *testing.T) {
		tx := flow.Transaction{
			Script:         generateCreateMinterScript(contractAddr, 1, 2),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		signAndSubmit(tx, b, t, false)
	})
}

func TestMinting(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	nftCode := readFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode, getNonce())
	assert.Nil(t, err)

	// Next, instantiate the minter
	createMinterTx := flow.Transaction{
		Script:         generateCreateMinterScript(contractAddr, 1, 2),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	signAndSubmit(createMinterTx, b, t, false)

	// Mint the first NFT
	mintTx := flow.Transaction{
		Script:         generateMintScript(contractAddr),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	signAndSubmit(mintTx, b, t, false)

	// Assert that ID/specialness are correct
	_, err = b.ExecuteScript(generateInspectNFTScript(contractAddr, b.RootAccountAddress(), 1, false))
	assert.Nil(t, err)

	// Mint a second NF
	mintTx2 := flow.Transaction{
		Script:         generateMintScript(contractAddr),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	signAndSubmit(mintTx2, b, t, false)

	// Assert that ID/specialness are correct
	_, err = b.ExecuteScript(generateInspectNFTScript(contractAddr, b.RootAccountAddress(), 2, true))
	assert.Nil(t, err)
}
