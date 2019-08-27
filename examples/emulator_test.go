package examples

import (
	"testing"
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/sdk/accounts"
	"github.com/dapperlabs/bamboo-node/sdk/emulator"
)

func TestSubmitTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	rawTx := &types.RawTransaction{
		Script: []byte(`
			fun main(account: Account) {
				account.storage["name"] = "Peter"
			}
		`),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}

	rootAddress := b.RootAccount()
	rootPrivateKey := b.RootKey()

	signedTx, err := rawTx.SignPayer(rootAddress, rootPrivateKey)
	if err != nil {
		t.Fatal("failed to sign")
	}

	err = b.SubmitTransaction(signedTx)
	if err != nil {
		t.Fatal("failed to sumbit transaction")
	}

	block := b.CommitBlock()
	t.Log(block.Hash().String())

	minedTx, err := b.GetTransaction(signedTx.Hash())
	if err != nil {
		t.Fatal("failed to get transaction")
	}

	t.Log(minedTx.Status.String())

	value, err := b.CallScript([]byte(`
		fun main(): String {
			let account = getAccount("0000000000000000000000000000000000000001")
			return ((account.storage["name"] ?? "") as? String) ?? ""
		}
	`))
	if err != nil {
		t.Fatal(err)
	}

	// should print "Peter"
	t.Log(value)
}

func TestCreateAccount(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	rootAddress := b.RootAccount()
	rootPrivateKey := b.RootKey()

	// generate new private key
	salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	accountPrivateKey, _ := salg.GeneratePrKey([]byte("RANDOM SEED"))

	// encode public key to bytes
	publicKey, _ := salg.EncodePubKey(accountPrivateKey.Pubkey())

	// generate an account creation transaction
	createAccountTx := accounts.CreateAccount(publicKey, nil)

	// sign account creation transaction as root account
	signedCreateAccountTx, _ := createAccountTx.SignPayer(rootAddress, rootPrivateKey)

	b.SubmitTransaction(signedCreateAccountTx)
	b.CommitBlock()

	// since the root account has address 0x01, this account will be 0x02
	accountAddress := types.HexToAddress("02")

	// sign transaction as the new account
	txA, _ := (&types.RawTransaction{
		Script: []byte(`
			fun main(account: Account) {
				account.storage["name"] = "Peter"
			}
		`),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(accountAddress, accountPrivateKey)

	b.SubmitTransaction(txA)
}
