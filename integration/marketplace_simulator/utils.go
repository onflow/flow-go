package marketplace

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	nbaData "github.com/dapperlabs/nba-smart-contracts/lib/go/templates/data"
	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

// randomPrivateKey returns a randomly generated ECDSA P-256 private key.
func randomPrivateKey() crypto.PrivateKey {
	seed := make([]byte, crypto.MinSeedLength)

	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	privateKey, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

func loadServiceAccount(flowClient *client.Client,
	servAccAddress *flowsdk.Address,
	servAccPrivKeyHex string) (*flowAccount, error) {

	acc, err := flowClient.GetAccount(context.Background(), *servAccAddress)
	if err != nil {
		return nil, fmt.Errorf("error while calling get account for service account %w", err)
	}
	accountKey := acc.Keys[0]

	privateKey, err := crypto.DecodePrivateKeyHex(accountKey.SigAlgo, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding serice account private key hex %w", err)
	}

	signer := crypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)

	return &flowAccount{
		address:    servAccAddress,
		accountKey: accountKey,
		seqNumber:  accountKey.SequenceNumber,
		signer:     signer,
		signerLock: sync.Mutex{},
	}, nil
}

const createAccountsScriptTemplate = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(publicKey: [UInt8], count: Int, initialTokenAmount: UFix64) {
  prepare(signer: AuthAccount) {
	let vault = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
      ?? panic("Could not borrow reference to the owner's Vault")

    var i = 0
    while i < count {
      let account = AuthAccount(payer: signer)
      account.addPublicKey(publicKey)

	  let receiver = account.getCapability(/public/flowTokenReceiver)
        .borrow<&{FungibleToken.Receiver}>()
		?? panic("Could not borrow receiver reference to the recipient's Vault")

      receiver.deposit(from: <-vault.withdraw(amount: initialTokenAmount))

      i = i + 1
    }
  }
}
`

// createAccountsScript returns a transaction script for creating an account
func createAccountsScript(fungibleToken, flowToken flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(createAccountsScriptTemplate, fungibleToken, flowToken))
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values)
}

func samplePlay() *nbaData.PlayMetadata {
	num := int32(10)
	return &nbaData.PlayMetadata{
		FullName:             "Ben Simmons",
		FirstName:            "Ben",
		LastName:             "Simmons",
		Birthdate:            "1996-07-20",
		Birthplace:           "Melbourne,, AUS",
		JerseyNumber:         "25",
		DraftTeam:            "Philadelphia 76ers",
		TeamAtMomentNBAID:    "1610612755",
		TeamAtMoment:         "Philadelphia 76ers",
		PrimaryPosition:      "PG",
		PlayerPosition:       "G",
		TotalYearsExperience: "2",
		NbaSeason:            "2019-20",
		DateOfMoment:         "2019-12-29T01:00:00Z",
		PlayCategory:         "Jump Shot",
		PlayType:             "2 Pointer",
		HomeTeamName:         "Miami Heat",
		AwayTeamName:         "Philadelphia 76ers",
		HomeTeamScore:        &num,
		AwayTeamScore:        &num,
		Height:               &num,
		Weight:               &num,
	}
}
