package examples

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/sdk/accounts"
	"github.com/dapperlabs/bamboo-node/sdk/emulator"
)

var (
	path     = "/Users/axiomzen/dev/src/github.com/dapperlabs/dapper-flow-working-session/bpl-contract/scripts/"
	nextAcct = 2
)

func getCode(fileName string) []byte {
	code, err := ioutil.ReadFile(path + fileName)
	if err != nil {
		panic(err)
	}
	return code
}

func newTransaction(script []byte) *types.RawTransaction {
	return &types.RawTransaction{
		Script:       script,
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}
}

func sendTransaction(b *emulator.EmulatedBlockchain, tx *types.RawTransaction, senderAddr types.Address, senderKey crypto.PrKey) error {
	signedTx, err := tx.SignPayer(senderAddr, senderKey)
	if err != nil {
		return err
	}

	return b.SubmitTransaction(signedTx)
}

func deployToken(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey) error {
	code := getCode("token.bpl")
	tx := newTransaction(code)
	return sendTransaction(b, tx, senderAddr, senderKey)
}

func createAccount(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey) {
	// generate new private key
	salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	accountPrivateKey, _ := salg.GeneratePrKey([]byte(nextAcct))

	// encode public key to bytes
	publicKey, _ := salg.EncodePubKey(accountPrivateKey.Pubkey())

	// generate an account creation transaction
	createAccountTx := accounts.CreateAccount(publicKey, nil)

	// sign account creation transaction as root account
	signedCreateAccountTx, _ := createAccountTx.SignPayer(rootAddress, rootPrivateKey)

	b.SubmitTransaction(signedCreateAccountTx)
	b.CommitBlock()
}

func TestDeployTokenContract(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	rootAddr := b.RootAccount()
	rootKey := b.RootKey()

	if err := deployToken(b, rootAddr, rootKey); err != nil {
		t.Error(err.Error())
	}
}
