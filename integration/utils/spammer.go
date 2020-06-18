package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
	"github.com/onflow/flow-go-sdk/templates"
)

const (
	// Pinned to specific commit
	// More transactions listed here: https://github.com/onflow/flow-ft/tree/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/transactions
	FungibleTokenTransactionsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/src/transactions/"
	TransferTokens                   = "transfer_tokens.cdc"
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
	txTracker            *TxTracker
	statsTracker         *StatsTracker
}

// In case we have to creat the client
// flowClient, err := client.New(gs.accessAddr, grpc.WithInsecure())
// require.NoError(gs.T(), err, "could not get client")

func decodeAddressFromHex(hexinput string) (*flowsdk.Address, error) {
	var output flowsdk.Address
	inputBytes, err := hex.DecodeString(hexinput)
	if err != nil {
		return nil, err
	}
	copy(output[:], inputBytes)
	return &output, nil
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

	// TODO get these params from the command line
	stTracker := NewStatsTracker(&StatsConfig{1, 1, 1, 1, 1, numberOfAccounts})
	txTracker, err := NewTxTracker(5000, 2, "localhost:3569", true, time.Second/10, stTracker)
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

func (cg *LoadGenerator) getBlockIDRef() flowsdk.Identifier {
	ref, err := cg.flowClient.GetLatestBlockHeader(context.Background(), false)
	examples.Handle(err)
	return ref.ID
}

func (cg *LoadGenerator) Stats() *StatsTracker {
	return cg.statsTracker
}

func (cg *LoadGenerator) SetupServiceAccountKeys() error {

	blockRef := cg.getBlockIDRef()
	publicKeysStr := strings.Builder{}
	for i := 0; i < cg.numberOfAccounts; i++ {
		publicKeysStr.WriteString("signer.addPublicKey(")
		publicKeysStr.WriteString(languageEncodeBytes(cg.serviceAccount.accountKey.Encode()))
		publicKeysStr.WriteString(")\n")
	}
	script := fmt.Sprintf(`
		transaction {
		prepare(signer: AuthAccount) {
				%s
			}
		}`, publicKeysStr.String())

	addKeysTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript([]byte(script)).
		SetProposalKey(*cg.serviceAccount.address, cg.serviceAccount.accountKey.ID, cg.serviceAccount.accountKey.SequenceNumber).
		SetPayer(*cg.serviceAccount.address).
		AddAuthorizer(*cg.serviceAccount.address)

	cg.serviceAccount.signerLock.Lock()
	defer cg.serviceAccount.signerLock.Unlock()

	err := addKeysTx.SignEnvelope(*cg.serviceAccount.address, cg.serviceAccount.accountKey.ID, cg.serviceAccount.signer)
	if err != nil {
		return err
	}
	cg.serviceAccount.accountKey.SequenceNumber++
	cg.step++

	err = cg.flowClient.SendTransaction(context.Background(), *addKeysTx)
	examples.Handle(err)

	txWG := sync.WaitGroup{}
	txWG.Add(1)
	cg.txTracker.AddTx(addKeysTx.ID(), nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			txWG.Done()
		},
		nil, nil, nil, nil, 60)

	txWG.Wait()
	// TODO pass methods for handling errors
	// examples.Handle(accountCreationTxRes.Error)

	fmt.Println("load generator step 0 done")
	return nil

}

func (cg *LoadGenerator) CreateAccounts() error {
	fmt.Printf("creating %d accounts...", cg.numberOfAccounts)
	blockRef := cg.getBlockIDRef()
	allTxWG := sync.WaitGroup{}
	for i := 0; i < cg.numberOfAccounts; i++ {
		privKey := examples.RandomPrivateKey()
		accountKey := flowsdk.NewAccountKey().
			FromPrivateKey(privKey).
			SetHashAlgo(crypto.SHA3_256).
			SetWeight(flowsdk.AccountKeyWeightThreshold)
		signer := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)
		createAccountScript, err := templates.CreateAccount([]*flowsdk.AccountKey{accountKey}, nil)
		// Generate an account creation script
		examples.Handle(err)
		createAccountTx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(createAccountScript).
			AddAuthorizer(*cg.serviceAccount.address).
			SetProposalKey(*cg.serviceAccount.address, i+1, 0).
			SetPayer(*cg.serviceAccount.address)

		cg.serviceAccount.signerLock.Lock()
		err = createAccountTx.SignEnvelope(*cg.serviceAccount.address, i+1, cg.serviceAccount.signer)
		if err != nil {
			return err
		}
		cg.serviceAccount.signerLock.Unlock()

		err = cg.flowClient.SendTransaction(context.Background(), *createAccountTx)
		examples.Handle(err)
		allTxWG.Add(1)

		cg.txTracker.AddTx(createAccountTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				for _, event := range res.Events {
					if event.Type == flowsdk.EventAccountCreated {
						accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
						accountAddress := accountCreatedEvent.Address()
						newAcc := newFlowAccount(&accountAddress, accountKey, signer)
						cg.accounts = append(cg.accounts, newAcc)
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
	cg.step++
	fmt.Printf("done\n")
	return nil
}

func (cg *LoadGenerator) DistributeInitialTokens() error {

	blockRef := cg.getBlockIDRef()
	allTxWG := sync.WaitGroup{}
	fmt.Println("load generator step 2 started")
	for i := 0; i < cg.numberOfAccounts; i++ {

		// Transfer 10000 tokens
		transferScript := GenerateTransferScript(cg.fungibleTokenAddress, cg.flowTokenAddress, cg.accounts[i].address, 10000)
		transferTx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(transferScript).
			SetProposalKey(*cg.serviceAccount.address, i+1, 1).
			SetPayer(*cg.serviceAccount.address).
			AddAuthorizer(*cg.serviceAccount.address)

		// TODO signer be thread safe
		cg.serviceAccount.signerLock.Lock()
		err := transferTx.SignEnvelope(*cg.serviceAccount.address, 0, cg.serviceAccount.signer)
		cg.serviceAccount.signerLock.Unlock()

		err = cg.flowClient.SendTransaction(context.Background(), *transferTx)
		examples.Handle(err)
		allTxWG.Add(1)

		cg.txTracker.AddTx(transferTx.ID(),
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
	cg.step++
	fmt.Println("load generator step 2 done")
	return nil
}

func (cg *LoadGenerator) RotateTokens() error {
	blockRef := cg.getBlockIDRef()
	allTxWG := sync.WaitGroup{}
	fmt.Println("load generator step 3 started")

	for i := 0; i < cg.numberOfAccounts; i++ {
		j := (i + 1) % cg.numberOfAccounts
		transferScript := GenerateTransferScript(cg.fungibleTokenAddress, cg.accounts[i].address, cg.accounts[j].address, 10)
		transferTx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(transferScript).
			SetProposalKey(*cg.accounts[i].address, 0, cg.accounts[i].seqNumber).
			SetPayer(*cg.accounts[i].address).
			AddAuthorizer(*cg.accounts[i].address)

		// TODO signer be thread safe
		cg.accounts[i].signerLock.Lock()
		err := transferTx.SignEnvelope(*cg.accounts[i].address, 0, cg.accounts[i].signer)
		cg.accounts[i].seqNumber++
		cg.accounts[i].signerLock.Unlock()

		err = cg.flowClient.SendTransaction(context.Background(), *transferTx)
		examples.Handle(err)
		allTxWG.Add(1)

		cg.txTracker.AddTx(transferTx.ID(),
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
	cg.step++
	fmt.Println("load generator step 3 done")
	return nil
}

func (cg *LoadGenerator) Next() error {
	switch cg.step {
	case 0:
		return cg.SetupServiceAccountKeys()
	case 1:
		return cg.CreateAccounts()
	case 2:
		return cg.DistributeInitialTokens()
	default:
		return cg.RotateTokens()
	}
	return nil
}

// languageEncodeBytes converts a byte slice to a comma-separated list of uint8 integers.
func languageEncodeBytes(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

// TODO remove the need to download many times
// GenerateTransferScript Creates a script that transfer some amount of FTs
func GenerateTransferScript(ftAddr, flowToken, toAddr *flowsdk.Address, amount int) []byte {
	mintCode, err := DownloadFile(FungibleTokenTransactionsBaseURL + TransferTokens)
	examples.Handle(err)

	withFTAddr := strings.ReplaceAll(string(mintCode), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x04", "0x"+toAddr.Hex(), 1)

	withAmount := strings.Replace(string(withToAddr), fmt.Sprintf("%d.0", amount), "0.01", 1)

	return []byte(withAmount)
}

// TODO use context deadlines

// TODO: Consider moving some of the following helpers to a common package, or just use any that are in the SDK once they're added there

// DownloadFile will download a url a byte slice
func DownloadFile(url string) ([]byte, error) {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func main() {

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
	lg, err := NewLoadGenerator(flowClient, priv, serviceAccountAddressHex, fungibleTokenAddressHex, flowTokenAddressHex, 10)
	if err != nil {
		panic(err)
	}

	rounds := 5

	// extra 3 is for setup
	for i := 0; i < rounds+3; i++ {
		lg.Next()
	}

	fmt.Println(lg.Stats().AvgTTF())
	// cg.txTracker.close()
	// wait for all
	time.Sleep(time.Second * 30)
	// TODO else do the transfers
}
