package utils

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

type flowAccount struct {
	i          int
	address    *flowsdk.Address
	accountKey *flowsdk.AccountKey
	seqNumber  uint64
	signer     crypto.InMemorySigner
	signerLock sync.Mutex
}

func (acc *flowAccount) signTx(tx *flowsdk.Transaction, keyID int) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()
	err := tx.SignEnvelope(*acc.address, keyID, acc.signer)
	if err != nil {
		return err
	}
	acc.seqNumber++
	return nil
}

func newFlowAccount(i int, address *flowsdk.Address, accountKey *flowsdk.AccountKey, signer crypto.InMemorySigner) *flowAccount {
	return &flowAccount{
		i:          i,
		address:    address,
		accountKey: accountKey,
		signer:     signer,
		seqNumber:  uint64(0),
		signerLock: sync.Mutex{},
	}
}

// BatchLoadGenerator submits a batch of transactions to the network
// by creating many accounts and transfer flow tokens between them
type BatchLoadGenerator struct {
	numberOfAccounts     int
	flowClient           *client.Client
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	accounts             []*flowAccount
	step                 int
	txTracker            *TxTracker
	statsTracker         *TxStatsTracker
}

// NewBatchLoadGenerator returns a new LoadGenerator
// TODO remove servAccPrivKeyHex when we open up account creation to everyone
func NewBatchLoadGenerator(fclient *client.Client,
	accessNodeAddress string,
	servAccPrivKeyHex string,
	serviceAccountAddress *flowsdk.Address,
	fungibleTokenAddress *flowsdk.Address,
	flowTokenAddress *flowsdk.Address,
	numberOfAccounts int,
	verbose bool) (*BatchLoadGenerator, error) {

	servAcc, err := loadServiceAccount(fclient, serviceAccountAddress, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	// TODO get these params hooked to the top level
	stTracker := NewTxStatsTracker(&StatsConfig{1, 1, 1, 1, 1, numberOfAccounts})
	txTracker, err := NewTxTracker(zerolog.New(os.Stderr), 5000, 100, accessNodeAddress, time.Second/10, stTracker)
	if err != nil {
		return nil, err
	}

	lGen := &BatchLoadGenerator{
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

func getBlockIDRef(c *client.Client) (flowsdk.Identifier, error) {
	ref, err := c.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return flowsdk.Identifier{}, err
	}
	return ref.ID, err
}

// Stats returns the statsTracker that captures stats for transactions submitted
func (lg *BatchLoadGenerator) Stats() *TxStatsTracker {
	return lg.statsTracker
}

// Close closes the transaction tracker gracefully.
func (lg *BatchLoadGenerator) Close() {
	lg.txTracker.Stop()
}

func (lg *BatchLoadGenerator) setupServiceAccountKeys() error {
	blockRef, err := getBlockIDRef(lg.flowClient)
	if err != nil {
		return err
	}

	cadenceKeys := make([]cadence.Value, lg.numberOfAccounts)
	for i := 0; i < lg.numberOfAccounts; i++ {
		cadenceKeys[i] = bytesToCadenceArray(lg.serviceAccount.accountKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		return err
	}

	addKeysTx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		AddAuthorizer(*lg.serviceAccount.address).
		SetReferenceBlockID(blockRef).
		SetProposalKey(*lg.serviceAccount.address, lg.serviceAccount.accountKey.Index, lg.serviceAccount.accountKey.SequenceNumber).
		SetPayer(*lg.serviceAccount.address)

	err = addKeysTx.AddArgument(cadenceKeysArray)
	if err != nil {
		return err
	}

	lg.serviceAccount.signerLock.Lock()
	defer lg.serviceAccount.signerLock.Unlock()

	err = addKeysTx.SignEnvelope(*lg.serviceAccount.address, lg.serviceAccount.accountKey.Index, lg.serviceAccount.signer)
	if err != nil {
		return err
	}
	lg.serviceAccount.accountKey.SequenceNumber++
	lg.step++

	err = lg.flowClient.SendTransaction(context.Background(), *addKeysTx)
	if err != nil {
		return err
	}

	txWG := sync.WaitGroup{}
	txWG.Add(1)
	lg.txTracker.AddTx(addKeysTx.ID(), nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			txWG.Done()
		},
		nil, // on sealed
		func(_ flowsdk.Identifier) {
			txWG.Done()
			panic("The setup transaction (service account keys) has expired. can not continue!")
		}, // on expired
		func(_ flowsdk.Identifier) {
			txWG.Done()
			panic("The setup transaction (service account keys) has timed out. can not continue!")
		}, // on timout
		nil, // on error,
		240)

	txWG.Wait()

	fmt.Println("load generator step 0 done")
	return nil

}

func (lg *BatchLoadGenerator) createAccounts() error {
	fmt.Printf("creating %d accounts...", lg.numberOfAccounts)

	blockRef, err := getBlockIDRef(lg.flowClient)
	if err != nil {
		return err
	}
	allTxWG := sync.WaitGroup{}
	for i := 0; i < lg.numberOfAccounts; i++ {
		privKey := randomPrivateKey()
		accountKey := flowsdk.NewAccountKey().
			FromPrivateKey(privKey).
			SetHashAlgo(crypto.SHA3_256).
			SetWeight(flowsdk.AccountKeyWeightThreshold)

		signer := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)

		createAccountTx := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKey},
			nil,
			*lg.serviceAccount.address,
		)

		// Generate an account creation script
		createAccountTx.
			SetReferenceBlockID(blockRef).
			SetProposalKey(*lg.serviceAccount.address, i+1, 0).
			SetPayer(*lg.serviceAccount.address)

		lg.serviceAccount.signerLock.Lock()
		err = createAccountTx.SignEnvelope(*lg.serviceAccount.address, i+1, lg.serviceAccount.signer)
		if err != nil {
			return err
		}
		lg.serviceAccount.signerLock.Unlock()

		err = lg.flowClient.SendTransaction(context.Background(), *createAccountTx)
		if err != nil {
			return err
		}
		allTxWG.Add(1)

		i := 0

		lg.txTracker.AddTx(createAccountTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				defer allTxWG.Done()
				for _, event := range res.Events {
					if event.Type == flowsdk.EventAccountCreated {
						accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
						accountAddress := accountCreatedEvent.Address()
						newAcc := newFlowAccount(i, &accountAddress, accountKey, signer)
						i++
						lg.accounts = append(lg.accounts, newAcc)
						fmt.Printf("new account %v added\n", accountAddress)
					}
				}
			},
			nil, // on sealed
			func(_ flowsdk.Identifier) {
				allTxWG.Done()
				panic("The setup transaction (account creation) has expired. can not continue!")
			}, // on expired
			func(_ flowsdk.Identifier) {
				allTxWG.Done()
				panic("The setup transaction (account creation) has timed out. can not continue!")
			}, // on timout
			nil, // on error
			240)
	}
	allTxWG.Wait()
	lg.step++
	fmt.Printf("done\n")
	return nil
}

func (lg *BatchLoadGenerator) distributeInitialTokens() error {
	blockRef, err := getBlockIDRef(lg.flowClient)
	if err != nil {
		return err
	}
	allTxWG := sync.WaitGroup{}
	fmt.Println("load generator step 2 started")
	for i := 0; i < len(lg.accounts); i++ {

		// Transfer 10000 tokens
		transferTx, err := TokenTransferTransaction(
			lg.fungibleTokenAddress,
			lg.flowTokenAddress,
			lg.accounts[i].address,
			10000)
		if err != nil {
			return err
		}
		transferTx = transferTx.
			SetReferenceBlockID(blockRef).
			SetProposalKey(*lg.serviceAccount.address, i+1, 1).
			SetPayer(*lg.serviceAccount.address).
			AddAuthorizer(*lg.serviceAccount.address)

		// TODO signer be thread safe
		lg.serviceAccount.signerLock.Lock()
		err = transferTx.SignEnvelope(*lg.serviceAccount.address, i+1, lg.serviceAccount.signer)
		if err != nil {
			return err
		}
		lg.serviceAccount.signerLock.Unlock()

		err = lg.flowClient.SendTransaction(context.Background(), *transferTx)
		if err != nil {
			return err
		}
		allTxWG.Add(1)

		lg.txTracker.AddTx(transferTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				fmt.Println(res)
				allTxWG.Done()
			},
			nil, nil, nil, nil, 240)
	}
	allTxWG.Wait()
	lg.step++
	fmt.Println("load generator step 2 done")
	return nil
}

func (lg *BatchLoadGenerator) rotateTokens() error {
	blockRef, err := getBlockIDRef(lg.flowClient)
	if err != nil {
		return err
	}
	allTxWG := sync.WaitGroup{}
	fmt.Printf("load generator step %d starting (%d accounts)... \n", lg.step, len(lg.accounts))

	for i := 0; i < len(lg.accounts); i++ {
		j := (i + 1) % len(lg.accounts)
		transferTx, err := TokenTransferTransaction(
			lg.fungibleTokenAddress,
			lg.flowTokenAddress,
			lg.accounts[j].address,
			10)
		if err != nil {
			return err
		}
		transferTx = transferTx.
			SetReferenceBlockID(blockRef).
			SetProposalKey(*lg.accounts[i].address, 0, lg.accounts[i].seqNumber).
			SetPayer(*lg.accounts[i].address).
			AddAuthorizer(*lg.accounts[i].address)

		// TODO signer be thread safe
		lg.accounts[i].signerLock.Lock()
		err = transferTx.SignEnvelope(*lg.accounts[i].address, 0, lg.accounts[i].signer)
		if err != nil {
			return err
		}
		lg.accounts[i].seqNumber++
		lg.accounts[i].signerLock.Unlock()

		err = lg.flowClient.SendTransaction(context.Background(), *transferTx)
		if err != nil {
			return err
		}

		allTxWG.Add(1)
		lg.txTracker.AddTx(transferTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				allTxWG.Done()
			},
			nil, // on sealed
			func(_ flowsdk.Identifier) {
				allTxWG.Done()
			}, // on expired
			func(_ flowsdk.Identifier) {
				allTxWG.Done()
			}, // on timout
			nil, // on error
			240)

	}
	allTxWG.Wait()
	fmt.Printf("load generator step %d is done \n", lg.step)
	lg.step++
	return nil
}

// Next submits the next batch of transactions to the network and waits
// until transactions are finalized, the first 3 calls setup accounts
// needed, and the rest of the calls rotates tokens between accounts
func (lg *BatchLoadGenerator) Next() error {
	switch lg.step {
	case 0:
		return lg.setupServiceAccountKeys()
	case 1:
		return lg.createAccounts()
	case 2:
		return lg.distributeInitialTokens()
	default:
		return lg.rotateTokens()
	}
}

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
