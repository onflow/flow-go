package sdk

import (
	"context"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/sdk"
	"github.com/dapperlabs/bamboo-node/sdk/client"
)

func ExampleCreateAccount_complete() {
	// load an existing account for signing
	myAccount, err := sdk.LoadAccountFromFile("./bamboo.json")
	if err != nil {
		panic("failed to load account!")
	}

	// generate a fresh key-pair for the new account
	keyPair, err := crypto.GenerateKeyPair("elephant ears")
	if err != nil {
		panic("failed to generate key-pair!")
	}

	// generate an account creation transaction
	tx := sdk.CreateAccount(keyPair.PublicKey, nil)
	tx.SetComputeLimit(100)

	signedTx := tx.SignPayer(myAccount.Account, myAccount.KeyPair)

	// connect to node and submit transaction
	c := client.New("localhost:5000")

	err := c.SendTransaction(context.Background(), signedTx)
	if err != nil {
		panic("failed to submit transaction!")
	}
}

// Load a user account from a JSON file.
func ExampleLoadAccountFromFile() {
	account, err := sdk.LoadAccountFromFile("./bamboo.json")
}
