package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

type LoadType string

const (
	TokenTransferLoadType LoadType = "token-transfer"
	TokenAddKeysLoadType  LoadType = "add-keys"
	CompHeavyLoadType     LoadType = "computation-heavy"
	EventHeavyLoadType    LoadType = "event-heavy"
	LedgerHeavyLoadType   LoadType = "ledger-heavy"
)

const accountCreationBatchSize = 100
const tokensPerTransfer = 0.01 // flow testnets only have 10e6 total supply, so we choose a small amount here

// ContLoadGenerator creates a continuous load of transactions to the network
// by creating many accounts and transfer flow tokens between them
type ContLoadGenerator struct {
	log                  zerolog.Logger
	loaderMetrics        *metrics.LoaderCollector
	initialized          bool
	tps                  int
	numberOfAccounts     int
	trackTxs             bool
	flowClient           *client.Client
	supervisorClient     *client.Client
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	favContractAddress   *flowsdk.Address
	accounts             []*flowAccount
	availableAccounts    chan *flowAccount // queue with accounts that are available for workers
	txTracker            *TxTracker
	txStatsTracker       *TxStatsTracker
	workerStatsTracker   *WorkerStatsTracker
	workers              []*Worker
	blockRef             BlockRef
	stopped              bool
	loadType             LoadType
}

// NewContLoadGenerator returns a new ContLoadGenerator
func NewContLoadGenerator(
	log zerolog.Logger,
	loaderMetrics *metrics.LoaderCollector,
	flowClient *client.Client,
	supervisorClient *client.Client,
	loadedAccessAddr string,
	servAccPrivKeyHex string,
	serviceAccountAddress *flowsdk.Address,
	fungibleTokenAddress *flowsdk.Address,
	flowTokenAddress *flowsdk.Address,
	tps int,
	loadType LoadType,
) (*ContLoadGenerator, error) {

	numberOfAccounts := tps * 10 // 1 second per block, factor 10 for delays to prevent sequence number collisions

	servAcc, err := loadServiceAccount(flowClient, serviceAccountAddress, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	// TODO get these params hooked to the top level
	txStatsTracker := NewTxStatsTracker(&StatsConfig{1, 1, 1, 1, 1, numberOfAccounts})
	txTracker, err := NewTxTracker(log, 5000, 100, loadedAccessAddr, time.Second, txStatsTracker)
	if err != nil {
		return nil, err
	}

	lGen := &ContLoadGenerator{
		log:                  log,
		loaderMetrics:        loaderMetrics,
		initialized:          false,
		tps:                  tps,
		numberOfAccounts:     numberOfAccounts,
		trackTxs:             false,
		flowClient:           flowClient,
		supervisorClient:     supervisorClient,
		serviceAccount:       servAcc,
		fungibleTokenAddress: fungibleTokenAddress,
		flowTokenAddress:     flowTokenAddress,
		accounts:             make([]*flowAccount, 0),
		availableAccounts:    make(chan *flowAccount, numberOfAccounts),
		txTracker:            txTracker,
		txStatsTracker:       txStatsTracker,
		workerStatsTracker:   NewWorkerStatsTracker(),
		blockRef:             NewBlockRef(supervisorClient),
		loadType:             loadType,
	}

	return lGen, nil
}

func (lg *ContLoadGenerator) Init() error {
	for i := 0; i < lg.numberOfAccounts; i += accountCreationBatchSize {
		if lg.stopped == true {
			return nil
		}

		num := lg.numberOfAccounts - i
		if num > accountCreationBatchSize {
			num = accountCreationBatchSize
		}
		err := lg.createAccounts(num)
		if err != nil {
			return err
		}
	}
	err := lg.SetupFavContract()
	if err != nil {
		lg.log.Error().Err(err).Msgf("failed to setup fav contract")
		return err
	}

	return nil
}

func (lg *ContLoadGenerator) SetupFavContract() error {
	// take one of the accounts
	if len(lg.accounts) == 0 {
		return fmt.Errorf("can't setup fav contract, zero accounts available")
	}

	acc := lg.accounts[0]

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		lg.log.Error().Err(err).Msgf("error getting reference block")
		return err
	}

	lg.log.Trace().Msgf("creating fav contract deployment script")
	deployScript := DeployingMyFavContractScript()

	lg.log.Trace().Msgf("creating fav contract deployment transaction")
	deploymentTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(deployScript).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("signing transaction")
	err = acc.signTx(deploymentTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error signing transaction")
		return err
	}

	lg.sendTx(deploymentTx)
	lg.favContractAddress = acc.address

	return nil
}

func (lg *ContLoadGenerator) Start() {
	// spawn workers
	for i := 0; i < lg.tps; i++ {
		var worker Worker

		switch lg.loadType {
		case TokenTransferLoadType:
			worker = NewWorker(i, 1*time.Second, lg.sendTokenTransferTx)
		case TokenAddKeysLoadType:
			worker = NewWorker(i, 1*time.Second, lg.sendAddKeyTx)
		// other types
		default:
			worker = NewWorker(i, 1*time.Second, lg.sendFavContractTx)
		}

		worker.Start()
		lg.workerStatsTracker.AddWorker()

		lg.workers = append(lg.workers, &worker)
	}

	lg.workerStatsTracker.StartPrinting(1 * time.Second)
}

func (lg *ContLoadGenerator) Stop() {
	lg.stopped = true
	for _, w := range lg.workers {
		w.Stop()
	}
	lg.txTracker.Stop()
	lg.workerStatsTracker.StopPrinting()
}

func (lg *ContLoadGenerator) createAccounts(num int) error {
	lg.log.Info().Msgf("creating and funding %d accounts...", num)

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}

	privKey := randomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(privKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)

	// Generate an account creation script
	createAccountTx := flowsdk.NewTransaction().
		SetScript(CreateAccountsScript(*lg.fungibleTokenAddress, *lg.flowTokenAddress)).
		SetReferenceBlockID(blockRef).
		SetGasLimit(9999).
		SetProposalKey(
			*lg.serviceAccount.address,
			lg.serviceAccount.accountKey.Index,
			lg.serviceAccount.accountKey.SequenceNumber,
		).
		AddAuthorizer(*lg.serviceAccount.address).
		SetPayer(*lg.serviceAccount.address)

	publicKey := bytesToCadenceArray(accountKey.Encode())
	count := cadence.NewInt(num)

	initialTokenAmount, err := cadence.NewUFix64FromParts(
		24*60*60*tokensPerTransfer, //  (24 hours at 1 block per second and 10 tokens sent)
		0,
	)
	if err != nil {
		return err
	}

	err = createAccountTx.AddArgument(publicKey)
	if err != nil {
		return err
	}

	err = createAccountTx.AddArgument(count)
	if err != nil {
		return err
	}

	err = createAccountTx.AddArgument(initialTokenAmount)
	if err != nil {
		return err
	}

	lg.serviceAccount.signerLock.Lock()
	err = createAccountTx.SignEnvelope(
		*lg.serviceAccount.address,
		lg.serviceAccount.accountKey.Index,
		lg.serviceAccount.signer,
	)
	if err != nil {
		return err
	}
	lg.serviceAccount.accountKey.SequenceNumber++
	lg.serviceAccount.signerLock.Unlock()

	err = lg.flowClient.SendTransaction(context.Background(), *createAccountTx)
	if err != nil {
		return err
	}

	wg.Add(1)

	i := 0

	lg.txTracker.AddTx(createAccountTx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			defer wg.Done()

			lg.log.Debug().
				Str("status", res.Status.String()).
				Msg("account creation tx executed")

			if res.Error != nil {
				lg.log.Error().
					Err(res.Error).
					Msg("account creation tx failed")
			}

			for _, event := range res.Events {
				lg.log.Trace().
					Str("event_type", event.Type).
					Str("event", event.String()).
					Msg("account creatin tx event")

				if event.Type == flowsdk.EventAccountCreated {
					accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
					accountAddress := accountCreatedEvent.Address()

					lg.log.Debug().
						Hex("address", accountAddress.Bytes()).
						Msg("new account created")

					signer := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)

					newAcc := newFlowAccount(i, &accountAddress, accountKey, signer)
					i++

					lg.accounts = append(lg.accounts, newAcc)
					lg.availableAccounts <- newAcc

					lg.log.Debug().
						Hex("address", accountAddress.Bytes()).
						Msg("new account added")
				}
			}
		},
		nil, // on sealed
		func(_ flowsdk.Identifier) {
			lg.log.Error().Msg("setup transaction (account creation) has expired")
			wg.Done()
		}, // on expired
		func(_ flowsdk.Identifier) {
			lg.log.Error().Msg("setup transaction (account creation) has timed out")
			wg.Done()
		}, // on timeout
		func(_ flowsdk.Identifier, err error) {
			lg.log.Error().Err(err).Msg("setup transaction (account creation) encountered an error")
			wg.Done()
		}, // on error
		120)

	wg.Wait()

	lg.log.Info().Msgf("created %d accounts", len(lg.accounts))

	return nil
}

func (lg *ContLoadGenerator) sendAddKeyTx(workerID int) {
	// TODO move this as a configurable parameter
	numberOfKeysToAdd := 40
	blockRef, err := lg.blockRef.Get()
	if err != nil {
		lg.log.Error().Err(err).Msgf("error getting reference block")
		return
	}

	lg.log.Trace().Msgf("getting next available account")

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()

	lg.log.Trace().Msgf("creating add proposer key script")
	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := 0; i < numberOfKeysToAdd; i++ {
		cadenceKeys[i] = bytesToCadenceArray(lg.serviceAccount.accountKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		lg.log.Error().Err(err).Msgf("error getting add key to account script")
		return
	}

	addKeysTx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		AddAuthorizer(*acc.address).
		SetReferenceBlockID(blockRef).
		SetGasLimit(9999).
		SetProposalKey(
			*lg.serviceAccount.address,
			lg.serviceAccount.accountKey.Index,
			lg.serviceAccount.accountKey.SequenceNumber,
		).
		SetPayer(*lg.serviceAccount.address)

	err = addKeysTx.AddArgument(cadenceKeysArray)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error constructing add keys to account transaction")
		return
	}

	lg.log.Trace().Msgf("creating transaction")

	addKeysTx.SetReferenceBlockID(blockRef).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("signing transaction")
	err = acc.signTx(addKeysTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error signing transaction")
		return
	}

	lg.sendTx(addKeysTx)
}

func (lg *ContLoadGenerator) sendTokenTransferTx(workerID int) {

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
	transferScript, err := TokenTransferScript(
		lg.fungibleTokenAddress,
		lg.flowTokenAddress,
		nextAcc.address,
		tokensPerTransfer)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error creating token transfer script")
		return
	}

	lg.log.Trace().Msgf("creating token transfer transaction")
	transferTx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(transferScript).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("signing transaction")
	err = acc.signTx(transferTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error signing transaction")
		return
	}

	lg.sendTx(transferTx)
}

// TODO update this to include loadtype
func (lg *ContLoadGenerator) sendFavContractTx(workerID int) {

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		lg.log.Error().Err(err).Msgf("error getting reference block")
		return
	}

	lg.log.Trace().Msgf("getting next available account")

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()
	var txScript []byte

	switch lg.loadType {
	case CompHeavyLoadType:
		txScript = ComputationHeavyScript(*lg.favContractAddress)
	case EventHeavyLoadType:
		txScript = EventHeavyScript(*lg.favContractAddress)
	case LedgerHeavyLoadType:
		txScript = LedgerHeavyScript(*lg.favContractAddress)
	}

	lg.log.Trace().Msgf("creating transaction")
	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(txScript).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("signing transaction")
	err = acc.signTx(tx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error signing transaction")
		return
	}

	lg.sendTx(tx)
}

func (lg *ContLoadGenerator) sendTx(tx *flowsdk.Transaction) {
	// TODO move this as a configurable parameter

	lg.log.Trace().Msgf("sending transaction")
	err := lg.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		lg.log.Error().Err(err).Msgf("error sending transaction")
		return
	}

	lg.log.Trace().Msgf("tracking sent transaction")
	lg.workerStatsTracker.AddTxSent()
	lg.loaderMetrics.TransactionSent()

	if lg.trackTxs {
		stopped := false
		wg := sync.WaitGroup{}
		lg.txTracker.AddTx(tx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", tx.ID().String()).Msgf("finalized tx")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on finalized
			func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", tx.ID().String()).Msgf("sealed tx")
			}, // on sealed
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", tx.ID().String()).Msgf("tx expired")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on expired
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", tx.ID().String()).Msgf("tx timed out")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on timout
			func(_ flowsdk.Identifier, err error) {
				lg.log.Error().Err(err).Str("tx_id", tx.ID().String()).Msgf("tx error")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on error
			60)
		wg.Add(1)
		wg.Wait()
	}
}
