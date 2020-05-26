package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
	"github.com/onflow/flow-go-sdk/templates"
)

const GreatTokenContractFile = "./great-token.cdc"

func main() {
	DeployContractDemo()
}

func DeployContractDemo() {
	// Connect to an emulator running locally
	ctx := context.Background()
	flowClient, err := client.New("127.0.0.1:3569", grpc.WithInsecure())
	examples.Handle(err)

	rootAcctAddr, rootAcctKey, rootSigner := examples.RootAccount(flowClient)

	myPrivateKey := examples.RandomPrivateKey()
	myAcctKey := flow.NewAccountKey().
		FromPrivateKey(myPrivateKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flow.AccountKeyWeightThreshold)
	mySigner := crypto.NewInMemorySigner(myPrivateKey, myAcctKey.HashAlgo)

	// Generate an account creation script
	createAccountScript, err := templates.CreateAccount([]*flow.AccountKey{myAcctKey}, nil)
	examples.Handle(err)

	createAccountTx := flow.NewTransaction().
		SetScript(createAccountScript).
		SetProposalKey(rootAcctAddr, rootAcctKey.ID, rootAcctKey.SequenceNumber).
		SetPayer(rootAcctAddr)

	err = createAccountTx.SignEnvelope(rootAcctAddr, rootAcctKey.ID, rootSigner)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *createAccountTx)
	examples.Handle(err)

	accountCreationTxRes := examples.WaitForSeal(ctx, flowClient, createAccountTx.ID())
	examples.Handle(accountCreationTxRes.Error)

	// Successful Tx, increment sequence number
	rootAcctKey.SequenceNumber++

	var myAddress flow.Address

	for _, event := range accountCreationTxRes.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			myAddress = accountCreatedEvent.Address()
		}
	}

	fmt.Println("My Address:", myAddress.Hex())

	// Deploy the Great NFT contract
	nftCode := examples.ReadFile(GreatTokenContractFile)
	deployScript, err := templates.CreateAccount(nil, nftCode)

	deployContractTx := flow.NewTransaction().
		SetScript(deployScript).
		SetProposalKey(myAddress, myAcctKey.ID, myAcctKey.SequenceNumber).
		SetPayer(myAddress)

	err = deployContractTx.SignEnvelope(
		myAddress,
		myAcctKey.ID,
		crypto.NewInMemorySigner(myPrivateKey, myAcctKey.HashAlgo),
	)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *deployContractTx)
	examples.Handle(err)

	deployContractTxResp := examples.WaitForSeal(ctx, flowClient, deployContractTx.ID())
	examples.Handle(deployContractTxResp.Error)

	// Successful Tx, increment sequence number
	myAcctKey.SequenceNumber++

	var nftAddress flow.Address

	for _, event := range deployContractTxResp.Events {
		if event.Type == flow.EventAccountCreated {
			accountCreatedEvent := flow.AccountCreatedEvent(event)
			nftAddress = accountCreatedEvent.Address()
		}
	}

	fmt.Println("My Address:", nftAddress.Hex())

	// Next, instantiate the minter
	createMinterScript := GenerateCreateMinterScript(nftAddress, 1, 2)

	createMinterTx := flow.NewTransaction().
		SetScript(createMinterScript).
		SetProposalKey(myAddress, myAcctKey.ID, myAcctKey.SequenceNumber).
		SetPayer(myAddress).
		AddAuthorizer(myAddress)

	err = createMinterTx.SignEnvelope(myAddress, myAcctKey.ID, mySigner)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *createMinterTx)
	examples.Handle(err)

	createMinterTxResp := examples.WaitForSeal(ctx, flowClient, deployContractTx.ID())
	examples.Handle(createMinterTxResp.Error)

	// Successful Tx, increment sequence number
	myAcctKey.SequenceNumber++

	mintScript := GenerateMintScript(nftAddress)

	// Mint the NFT
	mintTx := flow.NewTransaction().
		SetScript(mintScript).
		SetProposalKey(myAddress, myAcctKey.ID, myAcctKey.SequenceNumber).
		SetPayer(myAddress).
		AddAuthorizer(myAddress)

	err = mintTx.SignEnvelope(myAddress, myAcctKey.ID, mySigner)
	examples.Handle(err)

	err = flowClient.SendTransaction(ctx, *mintTx)
	examples.Handle(err)

	mintTxResp := examples.WaitForSeal(ctx, flowClient, mintTx.ID())
	examples.Handle(mintTxResp.Error)

	// Successful Tx, increment sequence number
	myAcctKey.SequenceNumber++

	fmt.Println("NFT minted!")

	result, err := flowClient.ExecuteScriptAtLatestBlock(ctx, GenerateGetNFTIDScript(nftAddress, myAddress))
	examples.Handle(err)

	myTokenID := result.(cadence.Int)

	fmt.Printf("You now own the Great NFT with ID: %d\n", myTokenID.Int())
}

// GenerateCreateMinterScript Creates a script that instantiates
// a new GreatNFTMinter instance and stores it in memory.
// Initial ID and special mod are arguments to the GreatNFTMinter constructor.
// The GreatNFTMinter must have been deployed already.
func GenerateCreateMinterScript(nftAddr flow.Address, initialID, specialMod int) []byte {
	template := `
		 import GreatToken from 0x%s
 
		 transaction {
 
			 prepare(acct: AuthAccount) {
				 let minter <- GreatToken.createGreatNFTMinter(firstID: %d, specialMod: %d)
				 acct.save(<-minter, to: /storage/GreatNFTMinter)
			 }
		 }
	 `

	return []byte(fmt.Sprintf(template, nftAddr, initialID, specialMod))
}

// GenerateMintScript Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func GenerateMintScript(nftCodeAddr flow.Address) []byte {
	template := `
		 import GreatToken from 0x%s
 
		 transaction {
			 prepare(acct: AuthAccount) {
			   let minter = acct.borrow<&GreatToken.GreatNFTMinter>(from: /storage/GreatNFTMinter)!
			   if let nft <- acct.load<@GreatToken.GreatNFT>(from: /storage/GreatNFT) {
				   destroy nft
			   }
			   acct.save(<-minter.mint(), to: /storage/GreatNFT)
			   acct.link<&GreatToken.GreatNFT>(/public/GreatNFT, target: /storage/GreatNFT)
			 }
		   }
	 `

	return []byte(fmt.Sprintf(template, nftCodeAddr.String()))
}

// GenerateGetNFTIDScript creates a script that retrieves an NFT from storage and returns its ID.
func GenerateGetNFTIDScript(nftCodeAddr, userAddr flow.Address) []byte {
	template := `
		 import GreatToken from 0x%s
 
		 pub fun main(): Int {
			 let acct = getAccount(0x%s)
			 let nft = acct.getCapability(/public/GreatNFT)!.borrow<&GreatToken.GreatNFT>()!
			 return nft.id()
		 }
	 `

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr))
}
