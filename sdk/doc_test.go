package sdk_test

import (
	"context"
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/sdk/accounts"
	"github.com/dapperlabs/bamboo-node/sdk/client"
)

func ExampleCreateAccount_complete() {
	// load an existing account for signing
	myAccount, err := accounts.LoadAccountFromFile("./bamboo.json")
	if err != nil {
		panic("failed to load account!")
	}

	// generate a fresh key-pair for the new account
	salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	prKey, err := salg.GeneratePrKey([]byte("elephant ears"))
	if err != nil {
		panic("failed to generate key-pair!")
	}

	pubKeyBytes, _ := salg.EncodePubKey(prKey.Pubkey())

	// generate an account creation transaction
	tx := accounts.CreateAccount(pubKeyBytes, nil)

	signedTx, err := tx.SignPayer(myAccount.Account, myAccount.Key)
	if err != nil {
		panic("failed to sign transaction!")
	}

	// connect to node and submit transaction
	c, err := client.New("localhost:5000")
	if err != nil {
		panic("failed to connect to node!")
	}

	err = c.SendTransaction(context.Background(), *signedTx)
	if err != nil {
		panic("failed to submit transaction!")
	}
}

// Load a user account from a JSON file.
func ExampleLoadAccountFromFile() {
	account, err := accounts.LoadAccountFromFile("./bamboo.json")
	if err != nil {
		panic("failed to load account!")
	}

	fmt.Printf("Loaded account with address %s", account.Account.String())
}
