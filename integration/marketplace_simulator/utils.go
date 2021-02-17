package marketplace

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
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
	accountKeys := acc.Keys

	privateKey, err := crypto.DecodePrivateKeyHex(accountKeys[0].SigAlgo, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding serice account private key hex %w", err)
	}

	signer := crypto.NewInMemorySigner(privateKey, accountKeys[0].HashAlgo)

	return &flowAccount{
		Address:     servAccAddress,
		accountKeys: accountKeys,
		signer:      signer,
		signerLock:  sync.Mutex{},
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

func AddKeyToAccountScript() ([]byte, error) {
	return []byte(`
    transaction(keys: [[UInt8]]) {
      prepare(signer: AuthAccount) {
      for key in keys {
        signer.addPublicKey(key)
      }
      }
    }
    `), nil
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

func makeMomentRange(min, max uint64) []uint64 {
	a := make([]uint64, max-min)
	for i := range a {
		a[i] = min + uint64(i)
	}
	return a
}

func generateBatchTransferMomentScript(nftAddr, tokenCodeAddr, recipientAddr *flowsdk.Address, momentIDs []uint64) []byte {
	template := `
		import NonFungibleToken from 0x%s
		import TopShot from 0x%s
		transaction {
			let transferTokens: @NonFungibleToken.Collection
			
			prepare(acct: AuthAccount) {
				let momentIDs = [%s]
		
				self.transferTokens <- acct.borrow<&TopShot.Collection>(from: /storage/MomentCollection)!.batchWithdraw(ids: momentIDs)
			}
		
			execute {
				// get the recipient's public account object
				let recipient = getAccount(0x%s)
		
				// get the Collection reference for the receiver
				let receiverRef = recipient.getCapability(/public/MomentCollection).borrow<&{TopShot.MomentCollectionPublic}>()!
		
				// deposit the NFT in the receivers collection
				receiverRef.batchDeposit(tokens: <-self.transferTokens)
			}
		}`

	// Stringify moment IDs
	momentIDList := ""
	for _, momentID := range momentIDs {
		id := strconv.Itoa(int(momentID))
		momentIDList = momentIDList + `UInt64(` + id + `), `
	}
	// Remove comma and space from last entry
	if idListLen := len(momentIDList); idListLen > 2 {
		momentIDList = momentIDList[:len(momentIDList)-2]
	}
	script := []byte(fmt.Sprintf(template, nftAddr, tokenCodeAddr.String(), momentIDList, recipientAddr))
	return script
}

func generateBatchTransferMomentfromShardedCollectionScript(nftAddr, tokenCodeAddr, shardedAddr, recipientAddr *flowsdk.Address, momentIDs []uint64) []byte {
	template := `
		import NonFungibleToken from 0x%s
		import TopShot from 0x%s
		import TopShotShardedCollection from 0x%s
		transaction {
			let transferTokens: @NonFungibleToken.Collection
			
			prepare(acct: AuthAccount) {
				let momentIDs = [%s]
		
				self.transferTokens <- acct.borrow<&TopShotShardedCollection.ShardedCollection>(from: /storage/ShardedMomentCollection)!.batchWithdraw(ids: momentIDs)
			}
		
			execute {
				// get the recipient's public account object
				let recipient = getAccount(0x%s)
		
				// get the Collection reference for the receiver
				let receiverRef = recipient.getCapability(/public/MomentCollection).borrow<&{TopShot.MomentCollectionPublic}>()!
		
				// deposit the NFT in the receivers collection
				receiverRef.batchDeposit(tokens: <-self.transferTokens)
			}
		}`

	// Stringify moment IDs
	momentIDList := ""
	for _, momentID := range momentIDs {
		id := strconv.Itoa(int(momentID))
		momentIDList = momentIDList + `UInt64(` + id + `), `
	}
	// Remove comma and space from last entry
	if idListLen := len(momentIDList); idListLen > 2 {
		momentIDList = momentIDList[:len(momentIDList)-2]
	}
	return []byte(fmt.Sprintf(template, nftAddr, tokenCodeAddr.String(), shardedAddr, momentIDList, recipientAddr))
}
