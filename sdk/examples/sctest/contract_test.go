// Package sctest implements a sample BPL contract and example testing
// code using the emulator test blockchain.
package sctest

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

const (
	greatTokenContractFile = "./contracts/great-token.bpl"
)

func readFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// Returns a nonce value that is guaranteed to be unique.
var getNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()

// Creates a script that instantiates a new GreatNFTMinter instance
// and stores it in memory.
// Initial ID and special mod are arguments to the GreatNFTMinter constructor.
// The GreatNFTMinter must have been deployed already.
func generateCreateMinterScript(nftAddr types.Address, initialID, specialMod int) []byte {
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
func generateMintScript(nftCodeAddr types.Address) []byte {
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
func generateInspectNFTScript(nftCodeAddr, userAddr types.Address, expectedID int, expectedIsSpecial bool) []byte {
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

func newEmulator() *emulator.EmulatedBlockchain {
	return emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
		OnLogMessage: func(msg string) {},
	})
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
		tx := types.Transaction{
			Script:         generateCreateMinterScript(contractAddr, -1, 2),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []types.Address{b.RootAccountAddress()},
		}

		err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		assert.Nil(t, err)
		err = b.SubmitTransaction(&tx)
		if assert.Error(t, err) {
			assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
		}
	})

	t.Run("Cannot create minter with special mod < 2", func(t *testing.T) {
		tx := types.Transaction{
			Script:         generateCreateMinterScript(contractAddr, 1, 1),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []types.Address{b.RootAccountAddress()},
		}

		_ = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		err = b.SubmitTransaction(&tx)
		if assert.Error(t, err) {
			assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
		}
	})

	t.Run("Should be able to create minter", func(t *testing.T) {
		tx := types.Transaction{
			Script:         generateCreateMinterScript(contractAddr, 1, 2),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []types.Address{b.RootAccountAddress()},
		}

		err = tx.AddSignature(b.RootAccountAddress(), b.RootKey())
		assert.Nil(t, err)
		err = b.SubmitTransaction(&tx)
		assert.Nil(t, err)
		b.CommitBlock()
	})
}

func TestMinting(t *testing.T) {
	b := newEmulator()

	// First, deploy the contract
	nftCode := readFile(greatTokenContractFile)
	contractAddr, err := b.CreateAccount(nil, nftCode, getNonce())
	assert.Nil(t, err)

	// Next, instantiate the minter
	createMinterTx := types.Transaction{
		Script:         generateCreateMinterScript(contractAddr, 1, 2),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []types.Address{b.RootAccountAddress()},
	}

	err = createMinterTx.AddSignature(b.RootAccountAddress(), b.RootKey())
	assert.Nil(t, err)
	err = b.SubmitTransaction(&createMinterTx)
	assert.Nil(t, err)

	// Mint the first NFT
	mintTx := types.Transaction{
		Script:         generateMintScript(contractAddr),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []types.Address{b.RootAccountAddress()},
	}

	err = mintTx.AddSignature(b.RootAccountAddress(), b.RootKey())
	assert.Nil(t, err)
	err = b.SubmitTransaction(&mintTx)
	assert.Nil(t, err)

	// Assert that ID/specialness are correct
	_, err = b.CallScript(generateInspectNFTScript(contractAddr, b.RootAccountAddress(), 1, false))
	assert.Nil(t, err)

	// Mint a second NFT
	mintTx2 := types.Transaction{
		Script:         generateMintScript(contractAddr),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []types.Address{b.RootAccountAddress()},
	}

	err = mintTx2.AddSignature(b.RootAccountAddress(), b.RootKey())
	assert.Nil(t, err)
	err = b.SubmitTransaction(&mintTx2)
	assert.Nil(t, err)

	// Assert that ID/specialness are correct
	_, err = b.CallScript(generateInspectNFTScript(contractAddr, b.RootAccountAddress(), 2, true))
	assert.Nil(t, err)
}
