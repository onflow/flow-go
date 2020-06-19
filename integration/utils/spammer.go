package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
)

// This should only used for testing reasons
type flowAccount struct {
	address    *flowsdk.Address
	accountKey *flowsdk.AccountKey
	seqNumber  uint64
	signer     crypto.InMemorySigner
	signerLock sync.Mutex
}

func newFlowAccount(address *flowsdk.Address,
	accountKey *flowsdk.AccountKey,
	signer crypto.InMemorySigner) *flowAccount {
	return &flowAccount{address: address,
		accountKey: accountKey,
		signer:     signer,
		seqNumber:  uint64(0),
		signerLock: sync.Mutex{},
	}
}

type LoadGenerator struct {
	numberOfAccounts     int
	flowClient           *client.Client
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	accounts             []*flowAccount
	step                 int
	scriptCreator        *ScriptCreator
	txTracker            *TxTracker
	statsTracker         *StatsTracker
}

// TODO flowsdk.Testnet as chainID
// TODO remove the need for servAccPrivKeyHex when we open it up to everyone
func NewLoadGenerator(fclient *client.Client,
	servAccPrivKeyHex string,
	serviceAccountAddressHex string,
	fungibleTokenAddressHex string,
	flowTokenAddressHex string,
	numberOfAccounts int) (*LoadGenerator, error) {

	serviceAccountAddress, err := decodeAddressFromHex(serviceAccountAddressHex)
	if err != nil {
		return nil, err
	}

	fungibleTokenAddress, err := decodeAddressFromHex(fungibleTokenAddressHex)
	if err != nil {
		return nil, err
	}

	flowTokenAddress, err := decodeAddressFromHex(flowTokenAddressHex)
	if err != nil {
		return nil, err
	}

	servAcc, err := loadServiceAccount(fclient, serviceAccountAddress, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	// TODO get these params hooked to the top level
	stTracker := NewStatsTracker(&StatsConfig{1, 1, 1, 1, 1, numberOfAccounts})
	txTracker, err := NewTxTracker(5000, 2, "localhost:3569", true, time.Second/10, stTracker)
	if err != nil {
		return nil, err
	}

	scriptCreator, err := NewScriptCreator()
	if err != nil {
		return nil, err
	}

	lGen := &LoadGenerator{
		numberOfAccounts:     numberOfAccounts,
		flowClient:           fclient,
		serviceAccount:       servAcc,
		fungibleTokenAddress: fungibleTokenAddress,
		flowTokenAddress:     flowTokenAddress,
		accounts:             make([]*flowAccount, 0),
		step:                 0,
		txTracker:            txTracker,
		statsTracker:         stTracker,
		scriptCreator:        scriptCreator,
	}
	return lGen, nil
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

func (lg *LoadGenerator) getBlockIDRef() flowsdk.Identifier {
	ref, err := lg.flowClient.GetLatestBlockHeader(context.Background(), false)
	examples.Handle(err)
	return ref.ID
}

func (lg *LoadGenerator) Stats() *StatsTracker {
	return lg.statsTracker
}

func (lg *LoadGenerator) SetupServiceAccountKeys() error {

	blockRef := lg.getBlockIDRef()
	keys := make([]*flowsdk.AccountKey, 0)
	for i := 0; i < lg.numberOfAccounts; i++ {
		keys = append(keys, lg.serviceAccount.accountKey)
	}
	script, err := lg.scriptCreator.AddKeyToAccountScript(keys)
	if err != nil {
		return err
	}

	addKeysTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript([]byte(script)).
		SetProposalKey(*lg.serviceAccount.address, lg.serviceAccount.accountKey.ID, lg.serviceAccount.accountKey.SequenceNumber).
		SetPayer(*lg.serviceAccount.address).
		AddAuthorizer(*lg.serviceAccount.address)

	lg.serviceAccount.signerLock.Lock()
	defer lg.serviceAccount.signerLock.Unlock()

	err = addKeysTx.SignEnvelope(*lg.serviceAccount.address, lg.serviceAccount.accountKey.ID, lg.serviceAccount.signer)
	if err != nil {
		return err
	}
	lg.serviceAccount.accountKey.SequenceNumber++
	lg.step++

	err = lg.flowClient.SendTransaction(context.Background(), *addKeysTx)
	examples.Handle(err)

	txWG := sync.WaitGroup{}
	txWG.Add(1)
	lg.txTracker.AddTx(addKeysTx.ID(), nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			txWG.Done()
		},
		nil, nil, nil, nil, 60)

	txWG.Wait()

	fmt.Println("load generator step 0 done")
	return nil

}

func (lg *LoadGenerator) CreateAccounts() error {
	fmt.Printf("creating %d accounts...", lg.numberOfAccounts)
	blockRef := lg.getBlockIDRef()
	allTxWG := sync.WaitGroup{}
	for i := 0; i < lg.numberOfAccounts; i++ {
		privKey := examples.RandomPrivateKey()
		accountKey := flowsdk.NewAccountKey().
			FromPrivateKey(privKey).
			SetHashAlgo(crypto.SHA3_256).
			SetWeight(flowsdk.AccountKeyWeightThreshold)
		signer := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)
		// TODO herer
		script, err := lg.scriptCreator.CreateAccountScript(accountKey)
		if err != nil {
			return err
		}
		// Generate an account creation script
		examples.Handle(err)
		createAccountTx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(script).
			AddAuthorizer(*lg.serviceAccount.address).
			SetProposalKey(*lg.serviceAccount.address, i+1, 0).
			SetPayer(*lg.serviceAccount.address)

		lg.serviceAccount.signerLock.Lock()
		err = createAccountTx.SignEnvelope(*lg.serviceAccount.address, i+1, lg.serviceAccount.signer)
		if err != nil {
			return err
		}
		lg.serviceAccount.signerLock.Unlock()

		err = lg.flowClient.SendTransaction(context.Background(), *createAccountTx)
		examples.Handle(err)
		allTxWG.Add(1)

		lg.txTracker.AddTx(createAccountTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				for _, event := range res.Events {
					if event.Type == flowsdk.EventAccountCreated {
						accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
						accountAddress := accountCreatedEvent.Address()
						newAcc := newFlowAccount(&accountAddress, accountKey, signer)
						lg.accounts = append(lg.accounts, newAcc)
						fmt.Println("account added")
					}
				}
				allTxWG.Done()
			},
			nil, // on sealed
			nil, // on expired
			nil, // on timout
			nil, // on error
			30)

		fmt.Println("<<<", i)
	}
	allTxWG.Wait()
	lg.step++
	fmt.Printf("done\n")
	return nil
}

func (lg *LoadGenerator) DistributeInitialTokens() error {

	blockRef := lg.getBlockIDRef()
	allTxWG := sync.WaitGroup{}
	fmt.Println("load generator step 2 started")
	for i := 0; i < lg.numberOfAccounts; i++ {

		// Transfer 10000 tokens
		transferScript, err := lg.scriptCreator.TokenTransferScript(
			lg.fungibleTokenAddress,
			lg.flowTokenAddress,
			lg.accounts[i].address,
			10000)
		if err != nil {
			return err
		}
		transferTx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(transferScript).
			SetProposalKey(*lg.serviceAccount.address, i+1, 1).
			SetPayer(*lg.serviceAccount.address).
			AddAuthorizer(*lg.serviceAccount.address)

		// TODO signer be thread safe
		lg.serviceAccount.signerLock.Lock()
		err = transferTx.SignEnvelope(*lg.serviceAccount.address, 0, lg.serviceAccount.signer)
		lg.serviceAccount.signerLock.Unlock()

		err = lg.flowClient.SendTransaction(context.Background(), *transferTx)
		examples.Handle(err)
		allTxWG.Add(1)

		lg.txTracker.AddTx(transferTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				allTxWG.Done()
			},
			nil, nil, nil, nil, 60)

	}
	allTxWG.Wait()
	lg.step++
	fmt.Println("load generator step 2 done")
	return nil
}

func (lg *LoadGenerator) RotateTokens() error {
	blockRef := lg.getBlockIDRef()
	allTxWG := sync.WaitGroup{}
	fmt.Println("load generator step 3 started")

	for i := 0; i < lg.numberOfAccounts; i++ {
		j := (i + 1) % lg.numberOfAccounts
		transferScript, err := lg.scriptCreator.TokenTransferScript(
			lg.fungibleTokenAddress,
			lg.accounts[i].address,
			lg.accounts[j].address,
			10)
		if err != nil {
			return err
		}
		transferTx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(transferScript).
			SetProposalKey(*lg.accounts[i].address, 0, lg.accounts[i].seqNumber).
			SetPayer(*lg.accounts[i].address).
			AddAuthorizer(*lg.accounts[i].address)

		// TODO signer be thread safe
		lg.accounts[i].signerLock.Lock()
		err = transferTx.SignEnvelope(*lg.accounts[i].address, 0, lg.accounts[i].signer)
		lg.accounts[i].seqNumber++
		lg.accounts[i].signerLock.Unlock()

		err = lg.flowClient.SendTransaction(context.Background(), *transferTx)
		examples.Handle(err)
		allTxWG.Add(1)
		lg.txTracker.AddTx(transferTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				allTxWG.Done()
			},
			nil, // on sealed
			nil, // on expired
			nil, // on timout
			nil, // on error
			30)

	}
	allTxWG.Wait()
	lg.step++
	fmt.Println("load generator step 3 done")
	return nil
}

func (lg *LoadGenerator) Next() error {
	switch lg.step {
	case 0:
		return lg.SetupServiceAccountKeys()
	case 1:
		return lg.CreateAccounts()
	case 2:
		return lg.DistributeInitialTokens()
	default:
		return lg.RotateTokens()
	}
	return nil
}

func decodeAddressFromHex(hexinput string) (*flowsdk.Address, error) {
	var output flowsdk.Address
	inputBytes, err := hex.DecodeString(hexinput)
	if err != nil {
		return nil, err
	}
	copy(output[:], inputBytes)
	return &output, nil
}

func main() {

	// addressGen := flowsdk.NewAddressGenerator(flowsdk.Testnet)
	// serviceAccountAddressHex = addressGen.NextAddress()
	// fmt.Println("Root Service Address:", serviceAccountAddressHex)
	// fungibleTokenAddress = addressGen.NextAddress()
	// fmt.Println("Fungible Address:", fungibleTokenAddress)
	// flowTokenAddress = addressGen.NextAddress()
	// fmt.Println("Flow Address:", flowTokenAddress)

	serviceAccountAddressHex := "8c5303eaa26202d6"
	fungibleTokenAddressHex := "9a0766d93b6608b7"
	flowTokenAddressHex := "7e60df042a9c0868"

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(unittest.ServiceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	// RLP decode the key
	ServiceAccountPrivateKey, err := flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// get the private key string
	priv := hex.EncodeToString(ServiceAccountPrivateKey.PrivateKey.Encode())

	flowClient, err := client.New("localhost:3569", grpc.WithInsecure())
	lg, err := NewLoadGenerator(flowClient, priv, serviceAccountAddressHex, fungibleTokenAddressHex, flowTokenAddressHex, 100)
	if err != nil {
		panic(err)
	}

	rounds := 5

	// extra 3 is for setup
	for i := 0; i < rounds+3; i++ {
		lg.Next()
	}

	fmt.Println(lg.Stats())
	lg.txTracker.Stop()
}
