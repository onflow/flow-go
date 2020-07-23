package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

const tokensPerTransfer = 10

// ContLoadGenerator creates a continuous load of transactions to the network
// by creating many accounts and transfer flow tokens between them
type ContLoadGenerator struct {
	log                  zerolog.Logger
	initialized          bool
	tps                  int
	numberOfAccounts     int
	trackTxs             bool
	flowClient           *client.Client
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	accounts             []*flowAccount
	availableAccounts    chan *flowAccount // queue with accounts that are available for workers
	scriptCreator        *ScriptCreator
	txTracker            *TxTracker
	txStatsTracker       *TxStatsTracker
	workerStatsTracker   *WorkerStatsTracker
	workers              []*Worker
	blockRef             BlockRef
}

// NewContLoadGenerator returns a new ContLoadGenerator
func NewContLoadGenerator(
	log zerolog.Logger,
	fclient *client.Client,
	accessNodeAddress string,
	servAccPrivKeyHex string,
	serviceAccountAddress *flowsdk.Address,
	fungibleTokenAddress *flowsdk.Address,
	flowTokenAddress *flowsdk.Address,
	tps int,
) (*ContLoadGenerator, error) {

	numberOfAccounts := tps * 10 * 10 // 1 second per block, 10 seconds to finalize, factor 10 for delays to prevent sequence number collisions

	servAcc, err := loadServiceAccount(fclient, serviceAccountAddress, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	// TODO get these params hooked to the top level
	txStatsTracker := NewTxStatsTracker(&StatsConfig{1, 1, 1, 1, 1, numberOfAccounts})
	txTracker, err := NewTxTracker(log, 5000, 100, accessNodeAddress, time.Second, txStatsTracker)
	if err != nil {
		return nil, err
	}

	scriptCreator, err := NewScriptCreator()
	if err != nil {
		return nil, err
	}

	lGen := &ContLoadGenerator{
		log:                  log,
		initialized:          false,
		tps:                  tps,
		numberOfAccounts:     numberOfAccounts,
		trackTxs:             true,
		flowClient:           fclient,
		serviceAccount:       servAcc,
		fungibleTokenAddress: fungibleTokenAddress,
		flowTokenAddress:     flowTokenAddress,
		accounts:             make([]*flowAccount, 0),
		availableAccounts:    make(chan *flowAccount, numberOfAccounts),
		txTracker:            txTracker,
		txStatsTracker:       txStatsTracker,
		workerStatsTracker:   NewWorkerStatsTracker(),
		scriptCreator:        scriptCreator,
		blockRef:             NewBlockRef(fclient),
	}

	return lGen, nil
}

func (lg *ContLoadGenerator) Init() error {
	err := lg.setupServiceAccountKeys()
	if err != nil {
		return err
	}
	err = lg.createAccounts()
	if err != nil {
		return err
	}
	err = lg.distributeInitialTokens()
	return err
}

func (lg *ContLoadGenerator) Start() error {
	// spawn workers
	for i := 0; i < lg.tps; i++ {
		worker := NewWorker(i, 1*time.Second, lg.sendTx)
		worker.Start()
		lg.workerStatsTracker.AddWorker()

		lg.workers = append(lg.workers, &worker)
	}

	lg.workerStatsTracker.StartPrinting(1 * time.Second)

	return nil
}

func (lg *ContLoadGenerator) Stop() error {
	for _, w := range lg.workers {
		w.Stop()
	}
	lg.txTracker.Stop()
	lg.workerStatsTracker.StopPrinting()

	return nil
}

func (lg *ContLoadGenerator) setupServiceAccountKeys() error {
	lg.log.Info().Msg("setting up service account keys...")

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		return err
	}
	keys := make([]*flowsdk.AccountKey, 0)
	for i := 0; i < lg.numberOfAccounts; i++ {
		keys = append(keys, lg.serviceAccount.accountKey)
	}

	addKeysTx, err := lg.scriptCreator.AddKeysToAccountTransaction(*lg.serviceAccount.address, keys)
	if err != nil {
		return err
	}

	addKeysTx.
		SetReferenceBlockID(blockRef).
		SetProposalKey(*lg.serviceAccount.address, lg.serviceAccount.accountKey.ID, lg.serviceAccount.accountKey.SequenceNumber).
		SetPayer(*lg.serviceAccount.address)

	lg.serviceAccount.signerLock.Lock()
	defer lg.serviceAccount.signerLock.Unlock()

	err = addKeysTx.SignEnvelope(*lg.serviceAccount.address, lg.serviceAccount.accountKey.ID, lg.serviceAccount.signer)
	if err != nil {
		return err
	}
	lg.serviceAccount.accountKey.SequenceNumber++

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
			lg.log.Fatal().Msg("The setup transaction (service account keys) has expired. can not continue!")
		}, // on expired
		func(_ flowsdk.Identifier) {
			txWG.Done()
			lg.log.Fatal().Msg("The setup transaction (service account keys) has timed out. can not continue!")
		}, // on timout
		nil, // on error,
		240)

	txWG.Wait()

	lg.log.Info().Msg("set up service account keys")
	return nil

}

func (lg *ContLoadGenerator) createAccounts() error {
	lg.log.Info().Msgf("creating %d accounts...", lg.numberOfAccounts)
	blockRef, err := lg.blockRef.Get()
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
				lg.log.Trace().Str("status", res.Status.String()).
					Msg("account creation tx executed")
				for _, event := range res.Events {
					lg.log.Trace().Str("event_type", event.Type).Str("event", event.String()).
						Msg("account creatin tx event")
					if event.Type == flowsdk.EventAccountCreated {
						accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
						accountAddress := accountCreatedEvent.Address()
						lg.log.Debug().Hex("address", accountAddress.Bytes()).Msg("new account created")
						newAcc := newFlowAccount(i, &accountAddress, accountKey, signer)
						i++
						lg.accounts = append(lg.accounts, newAcc)
						lg.availableAccounts <- newAcc
						lg.log.Debug().Hex("address", accountAddress.Bytes()).Msg("new account added")
					}
				}
			},
			nil, // on sealed
			func(_ flowsdk.Identifier) {
				lg.log.Fatal().Msg("The setup transaction (account creation) has expired. can not continue!")
				allTxWG.Done()
			}, // on expired
			func(_ flowsdk.Identifier) {
				lg.log.Fatal().Msg("The setup transaction (account creation) has timed out. can not continue!")
				allTxWG.Done()
			}, // on timout
			nil, // on error
			240)
	}
	allTxWG.Wait()

	lg.log.Info().Msgf("created %d accounts", len(lg.accounts))

	return nil
}

func (lg *ContLoadGenerator) fundAccount(blockRef flowsdk.Identifier, acc *flowAccount, done func()) error {
	// Transfer tokens
	transferScript, err := lg.scriptCreator.TokenTransferScript(
		lg.fungibleTokenAddress,
		lg.flowTokenAddress,
		acc.address,
		24*60*60*tokensPerTransfer) //  (24 hours a 1 block per second a 10 tokens sent)
	if err != nil {
		return err
	}
	transferTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(transferScript).
		SetProposalKey(*lg.serviceAccount.address, acc.i+1, 1).
		SetPayer(*lg.serviceAccount.address).
		AddAuthorizer(*lg.serviceAccount.address)

	// TODO signer be thread safe
	lg.serviceAccount.signerLock.Lock()
	err = transferTx.SignEnvelope(*lg.serviceAccount.address, acc.i+1, lg.serviceAccount.signer)
	if err != nil {
		return err
	}
	lg.serviceAccount.signerLock.Unlock()

	err = lg.flowClient.SendTransaction(context.Background(), *transferTx)
	if err != nil {
		return err
	}

	lg.txTracker.AddTx(transferTx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			// fmt.Println(res)
			done()
		},
		nil, nil, nil, nil, 240)

	return nil
}

func (lg *ContLoadGenerator) distributeInitialTokens() error {
	lg.log.Info().Msgf("distributing initial tokens...")
	blockRef, err := lg.blockRef.Get()
	if err != nil {
		return err
	}
	allTxWG := sync.WaitGroup{}

	for i := 0; i < len(lg.accounts); i++ {
		allTxWG.Add(1)
		err := lg.fundAccount(blockRef, lg.accounts[i], func() { allTxWG.Done() })
		if err != nil {
			lg.log.Error().Err(err).Msg("error funding account")
		}
	}
	allTxWG.Wait()

	lg.log.Info().Msgf("distributed initial tokens")
	return nil
}

func (lg *ContLoadGenerator) sendTx(workerID int) {
	lg.log.Debug().Msgf("sending tx from worker %v", workerID)

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		lg.log.Error().Err(err).Msgf("error getting reference block")
		return
	}

	lg.log.Trace().Msgf("getting next available account")

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()

	lg.log.Trace().Msgf("getting next account")
	nextAcc := lg.accounts[(acc.i+1)%len(lg.accounts)]

	lg.log.Trace().Msgf("creating transfer script")
	transferScript, err := lg.scriptCreator.TokenTransferScript(
		lg.fungibleTokenAddress,
		acc.address,
		nextAcc.address,
		tokensPerTransfer)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error creating token trasferscript")
		return
	}

	lg.log.Trace().Msgf("creating transaction")
	transferTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(transferScript).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("signing transaction")
	acc.signerLock.Lock()
	err = transferTx.SignEnvelope(*acc.address, 0, acc.signer)
	if err != nil {
		acc.signerLock.Unlock()
		lg.log.Error().Err(err).Msgf("error signing transaction")
		return
	}
	acc.seqNumber++
	acc.signerLock.Unlock()

	lg.log.Trace().Msgf("sending transaction")
	err = lg.flowClient.SendTransaction(context.Background(), *transferTx)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error sending transaction")
		return
	}

	lg.log.Trace().Msgf("tracking sent transaction")
	lg.workerStatsTracker.AddTxSent()

	if lg.trackTxs {
		wg := sync.WaitGroup{}
		wg.Add(1)
		lg.txTracker.AddTx(transferTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", transferTx.ID().String()).Msgf("finalized tx")
				wg.Done()
			}, // on finalized
			func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", transferTx.ID().String()).Msgf("sealed tx")
			}, // on sealed
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", transferTx.ID().String()).Msgf("tx expired")
				wg.Done()
			}, // on expired
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", transferTx.ID().String()).Msgf("tx timed out")
				wg.Done()
			}, // on timout
			func(_ flowsdk.Identifier, err error) {
				lg.log.Error().Err(err).Str("tx_id", transferTx.ID().String()).Msgf("tx error")
				wg.Done()
			}, // on error
			60)
		wg.Wait()
	}
}
