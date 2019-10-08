// Package sctest implements a sample BPL contract and example testing
// code using the emulator test blockchain.
package sctest

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	. "github.com/onsi/gomega"
)

const (
	greatTokenContractFile = "./contracts/great-token.bpl"
)

func readFile(path string) []byte {
	contents, _ := ioutil.ReadFile(path)
	return contents
}

// Taken from sdk/emulator/emulator_test.go
func bytesToString(b []byte) string {
	if b == nil {
		return "nil"
	}
	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

func generateMintScript(nftCodeAddr types.Address) []byte {
	template := `
		import GreatNFTMinter, GreatNFT from 0x%s

		fun main(acct: Account) {
			var minter = GreatNFTMinter()
			var nft = minter.maybeMint()

			//acct.storage["my_nft"] = nft
		}`

	filledTemplate := fmt.Sprintf(template, nftCodeAddr.String())
	return []byte(filledTemplate)
}

// Taken from sdk/emulator/emulator_test.go
func generateCreateAccountScript(publicKey, code []byte) []byte {
	script := fmt.Sprintf(`
		fun main() {
			createAccount(%s, %s)
		}
	`, bytesToString(publicKey), bytesToString(code))
	return []byte(script)
}

func newEmulator() *emulator.EmulatedBlockchain {
	return emulator.NewEmulatedBlockchain(&emulator.EmulatedBlockchainOptions{
		RuntimeLogger: func(msg string) { fmt.Println(msg) },
	})
}

// Taken from sdk/emulator/emulator_test.go
// TODO: Expose this from emulator package after multi-key changes are finished.
func createAccount(b *emulator.EmulatedBlockchain, publicKey, code []byte) (types.Address, error) {
	createAccountScript := generateCreateAccountScript(publicKey, code)
	fmt.Println(string(createAccountScript))

	tx1 := &types.Transaction{
		Script:             createAccountScript,
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	err := b.SubmitTransaction(tx1)
	if err != nil {
		return types.Address{}, err
	}

	return b.LastCreatedAccount().Address, nil
}

func TestDeployment(t *testing.T) {
	RegisterTestingT(t)

	b := newEmulator()

	// Should be able to deploy a contract as a new account with no keys.
	nftCode := readFile(greatTokenContractFile)
	contractAddr, err := createAccount(b, nil, nftCode)
	Expect(err).ToNot(HaveOccurred())

	mintScript := generateMintScript(contractAddr)
	fmt.Println(string(mintScript))

	tx := types.Transaction{
		Script:         mintScript,
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []types.Address{b.RootAccountAddress()},
	}

	tx.AddSignature(b.RootAccountAddress(), b.RootKey())
	err = b.SubmitTransaction(&tx)
	Expect(err).ToNot(HaveOccurred())
	b.CommitBlock()

	// acctPrKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	// acctPubKey, _ := privateKeyB.Publickey().Encode()
}

func TestMinting(t *testing.T) {
	RegisterTestingT(t)
	b := newEmulator()

	// First, deploy the contract
	nftCode := readFile(greatTokenContractFile)
	contractAddr, err := createAccount(b, nil, nftCode)
	Expect(err).ToNot(HaveOccurred())

	// Create a transaction that mints a GreatToken by calling the minter.
	mintScript := generateMintScript(contractAddr)
	fmt.Println(string(mintScript))

	tx := types.Transaction{
		Script:         mintScript,
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []types.Address{b.RootAccountAddress()},
	}

	tx.AddSignature(b.RootAccountAddress(), b.RootKey())
	err = b.SubmitTransaction(&tx)
	Expect(err).ToNot(HaveOccurred())
	b.CommitBlock()
}
