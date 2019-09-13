package examples

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/accounts"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

var (
	path     = "../scripts2/"
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

func sendTransaction(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey, code []byte) error {
	tx := &types.RawTransaction{
		Script:       code,
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}
	signedTx, err := tx.SignPayer(senderAddr, senderKey)
	if err != nil {
		return err
	}

	return b.SubmitTransaction(signedTx)
}

func sendToken(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey) error {
	code := getCode("transaction1.bpl")
	return sendTransaction(b, senderAddr, senderKey, code)
}

func deployCode(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey, code []byte) (crypto.PrKey, error) {
	return createAccount(b, senderAddr, senderKey, code)
}

func addTokenToAccount(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey) error {
	code := getCode("setup.bpl")
	return sendTransaction(b, senderAddr, senderKey, code)
}

func addTokenToAccountWithToken(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey) error {
	code := getCode("setup_with_token.bpl")
	return sendTransaction(b, senderAddr, senderKey, code)
}

func createAccount(b *emulator.EmulatedBlockchain, senderAddr types.Address, senderKey crypto.PrKey, code []byte) (crypto.PrKey, error) {
	// generate new private key
	salg, err := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	if err != nil {
		return nil, err
	}
	accountPrivateKey, err := salg.GeneratePrKey([]byte{byte(nextAcct)})
	if err != nil {
		return nil, err
	}

	// encode public key to bytes
	publicKey, err := salg.EncodePubKey(accountPrivateKey.Pubkey())
	if err != nil {
		return nil, err
	}

	// generate an account creation transaction
	createAccountTx := accounts.CreateAccount(publicKey, code)

	// sign account creation transaction as root account
	signedCreateAccountTx, err := createAccountTx.SignPayer(senderAddr, senderKey)
	if err != nil {
		return nil, err
	}

	err = b.SubmitTransaction(signedCreateAccountTx)
	if err != nil {
		return nil, err
	}

	b.CommitBlock()

	return accountPrivateKey, nil
}

func TestTokenContract(t *testing.T) {
	options := emulator.DefaultOptions
	options.RuntimeLogger = func(msg string) { fmt.Println(msg) }

	b := emulator.NewEmulatedBlockchain(options)

	rootAddr := b.RootAccount()
	rootKey := b.RootKey()

	accountA := types.HexToAddress("03")
	accountB := types.HexToAddress("04")

	// token deployed at 02
	if _, err := deployCode(b, rootAddr, rootKey, getCode("token.bpl")); err != nil {
		t.Error(err.Error())
	}

	// make an account with token support and no tokens at 03
	keyA, err := createAccount(b, rootAddr, rootKey, nil)
	if err != nil {
		t.Error(err.Error())
	}

	err = addTokenToAccount(b, accountA, keyA)
	if err != nil {
		t.Error(err.Error())
	}

	// make another account with token support and a token at 04
	keyB, err := createAccount(b, rootAddr, rootKey, nil)
	if err != nil {
		t.Error(err.Error())
	}

	err = addTokenToAccountWithToken(b, accountB, keyB)
	if err != nil {
		t.Error(err.Error())
	}

	// 04 gives token to 03
	if err := sendToken(b, accountB, keyB); err != nil {
		t.Error(err.Error())
	}

	b.CommitBlock()

	_, err = b.CallScript([]byte(`
		import Token, TokenContainer from 0x0000000000000000000000000000000000000002

		fun logTokens(_ acc: String) {
			let receiver = getAccount(acc)
			let receiverTcAny = receiver.storage["tc"] ?? panic("receiver does not have a TokenContainer") 
			let receiverTc = (receiverTcAny as? TokenContainer) ?? panic("could not cast receiver tc to TokenContainer")

			log(receiverTc._tokens)
		}

		fun main() {
			logTokens("03")
			logTokens("04")
		}
	`))
	if err != nil {
		t.Error(err.Error())
	}
}
