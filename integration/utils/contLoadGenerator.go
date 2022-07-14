package utils

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"time"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
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

const slowTransactionThreshold = 30 * time.Second

var accountCreationBatchSize = 750 // a higher number would hit max gRPC message size
const tokensPerTransfer = 0.01     // flow testnets only have 10e6 total supply, so we choose a small amount here

// ContLoadGenerator creates a continuous load of transactions to the network
// by creating many accounts and transfer flow tokens between them
type ContLoadGenerator struct {
	log                  zerolog.Logger
	loaderMetrics        *metrics.LoaderCollector
	tps                  int
	numberOfAccounts     int
	flowClient           access.Client
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	favContractAddress   *flowsdk.Address
	accounts             []*flowAccount
	availableAccounts    chan *flowAccount // queue with accounts available for workers
	workerStatsTracker   *WorkerStatsTracker
	workers              []*Worker
	stopped              bool
	loadType             LoadType
	follower             TxFollower
	availableAccountsLo  int
}

// NewContLoadGenerator returns a new ContLoadGenerator
func NewContLoadGenerator(
	log zerolog.Logger,
	loaderMetrics *metrics.LoaderCollector,
	flowClient access.Client,
	supervisorClient access.Client,
	loadedAccessAddr string,
	servAccPrivKeyHex string,
	serviceAccountAddress *flowsdk.Address,
	fungibleTokenAddress *flowsdk.Address,
	flowTokenAddress *flowsdk.Address,
	tps int,
	accountMultiplier int,
	loadType LoadType,
	feedbackEnabled bool,
) (*ContLoadGenerator, error) {
	// Create "enough" accounts to prevent sequence number collisions.
	numberOfAccounts := tps * accountMultiplier

	servAcc, err := loadServiceAccount(flowClient, serviceAccountAddress, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	var follower TxFollower
	if feedbackEnabled {
		follower, err = NewTxFollower(context.TODO(), supervisorClient, WithLogger(log), WithMetrics(loaderMetrics))
	} else {
		follower, err = NewNopTxFollower(context.TODO(), supervisorClient)
	}
	if err != nil {
		return nil, err
	}

	lGen := &ContLoadGenerator{
		log:                  log,
		loaderMetrics:        loaderMetrics,
		tps:                  tps,
		numberOfAccounts:     numberOfAccounts,
		flowClient:           flowClient,
		serviceAccount:       servAcc,
		fungibleTokenAddress: fungibleTokenAddress,
		flowTokenAddress:     flowTokenAddress,
		accounts:             make([]*flowAccount, 0),
		availableAccounts:    make(chan *flowAccount, numberOfAccounts),
		workerStatsTracker:   NewWorkerStatsTracker(),
		follower:             follower,
		loadType:             loadType,
		availableAccountsLo:  numberOfAccounts,
	}

	return lGen, nil
}

func (lg *ContLoadGenerator) Init() error {
	for i := 0; i < lg.numberOfAccounts; i += accountCreationBatchSize {
		if lg.stopped {
			return nil
		}

		num := lg.numberOfAccounts - i
		if num > accountCreationBatchSize {
			num = accountCreationBatchSize
		}

		lg.log.Info().Int("cumulative", i).Int("num", num).Int("numberOfAccounts", lg.numberOfAccounts).Msg("creating accounts")
		err := lg.createAccounts(num)
		if err != nil {
			return err
		}
	}
	err := lg.setupFavContract()
	if err != nil {
		lg.log.Error().Err(err).Msg("failed to setup fav contract")
		return err
	}

	return nil
}

func (lg *ContLoadGenerator) setupFavContract() error {
	// take one of the accounts
	if len(lg.accounts) == 0 {
		return errors.New("can't setup fav contract, zero accounts available")
	}

	acc := lg.accounts[0]

	lg.log.Trace().Msg("creating fav contract deployment script")
	deployScript := DeployingMyFavContractScript()

	lg.log.Trace().Msg("creating fav contract deployment transaction")
	deploymentTx := flowsdk.NewTransaction().
		SetReferenceBlockID(lg.follower.BlockID()).
		SetScript(deployScript).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msg("signing transaction")
	err := acc.signTx(deploymentTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msg("error signing transaction")
		return err
	}

	ch, err := lg.sendTx(-1, deploymentTx)
	if err != nil {
		return err
	}
	<-ch

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
	defer lg.log.Debug().Msg("stopped generator")

	lg.stopped = true
	wg := sync.WaitGroup{}
	wg.Add(len(lg.workers))
	for _, w := range lg.workers {
		w := w

		go func() {
			defer wg.Done()

			lg.log.Debug().Int("workerID", w.workerID).Msg("stopping worker")
			w.Stop()
		}()
	}
	wg.Wait()
	lg.workerStatsTracker.StopPrinting()
	lg.log.Debug().Msg("stopping follower")
	lg.follower.Stop()
}

func (lg *ContLoadGenerator) createAccounts(num int) error {
	privKey := randomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(privKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)

	// Generate an account creation script
	createAccountTx := flowsdk.NewTransaction().
		SetScript(CreateAccountsScript(*lg.fungibleTokenAddress, *lg.flowTokenAddress)).
		SetReferenceBlockID(lg.follower.BlockID()).
		SetGasLimit(999999).
		SetProposalKey(
			*lg.serviceAccount.address,
			lg.serviceAccount.accountKey.Index,
			lg.serviceAccount.accountKey.SequenceNumber,
		).
		AddAuthorizer(*lg.serviceAccount.address).
		SetPayer(*lg.serviceAccount.address)

	publicKey := bytesToCadenceArray(accountKey.PublicKey.Encode())
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

	err = lg.serviceAccount.signCreateAccountTx(createAccountTx)
	if err != nil {
		return err
	}

	ch, err := lg.sendTx(-1, createAccountTx)
	if err != nil {
		return err
	}
	<-ch

	log := lg.log.With().Str("tx_id", createAccountTx.ID().String()).Logger()
	result, err := WaitForTransactionResult(context.Background(), lg.flowClient, createAccountTx.ID())
	if err != nil {
		return fmt.Errorf("failed to get transactions result: %w", err)
	}

	log.Trace().Str("status", result.Status.String()).Msg("account creation tx executed")
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("account creation tx failed")
	}

	var accountsCreated int
	for _, event := range result.Events {
		log.Trace().Str("event_type", event.Type).Str("event", event.String()).Msg("account creation tx event")

		if event.Type == flowsdk.EventAccountCreated {
			accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
			accountAddress := accountCreatedEvent.Address()

			log.Trace().Hex("address", accountAddress.Bytes()).Msg("new account created")

			signer, err := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)
			if err != nil {
				return fmt.Errorf("singer creation failed: %w", err)
			}

			newAcc := newFlowAccount(accountsCreated, &accountAddress, accountKey, signer)
			accountsCreated++

			lg.accounts = append(lg.accounts, newAcc)
			lg.availableAccounts <- newAcc

			log.Trace().Hex("address", accountAddress.Bytes()).Msg("new account added")
		}
	}
	if accountsCreated != num {
		return fmt.Errorf("failed to create enough contracts, expected: %d, created: %d",
			num, accountsCreated)
	}
	return nil
}

func (lg *ContLoadGenerator) sendAddKeyTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	// TODO move this as a configurable parameter
	numberOfKeysToAdd := 40

	log.Trace().Msg("getting next available account")

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()

	log.Trace().Msg("creating add proposer key script")
	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := 0; i < numberOfKeysToAdd; i++ {
		cadenceKeys[i] = bytesToCadenceArray(lg.serviceAccount.accountKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		log.Error().Err(err).Msg("error getting add key to account script")
		return
	}

	addKeysTx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		AddAuthorizer(*acc.address).
		SetReferenceBlockID(lg.follower.BlockID()).
		SetGasLimit(9999).
		SetProposalKey(
			*lg.serviceAccount.address,
			lg.serviceAccount.accountKey.Index,
			lg.serviceAccount.accountKey.SequenceNumber,
		).
		SetPayer(*lg.serviceAccount.address)

	err = addKeysTx.AddArgument(cadenceKeysArray)
	if err != nil {
		log.Error().Err(err).Msg("error constructing add keys to account transaction")
		return
	}

	log.Trace().Msg("creating transaction")

	addKeysTx.SetReferenceBlockID(lg.follower.BlockID()).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	log.Trace().Msg("signing transaction")
	err = acc.signTx(addKeysTx, 0)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	ch, err := lg.sendTx(workerID, addKeysTx)
	if err != nil {
		return
	}
	<-ch
}

func (lg *ContLoadGenerator) sendTokenTransferTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	log.Trace().
		Int("availableAccounts", len(lg.availableAccounts)).
		Msg("getting next available account")

	if workerID == 0 {
		l := len(lg.availableAccounts)
		if l < lg.availableAccountsLo {
			lg.availableAccountsLo = l
			log.Debug().Int("availableAccountsLo", l).Int("numberOfAccounts", lg.numberOfAccounts).Msg("discovered new account low")
		}
	}

	var acc *flowAccount
	select {
	case acc = <-lg.availableAccounts:
	default:
		log.Error().Msg("next available account channel empty; skipping send")
		return
	}
	defer func() { lg.availableAccounts <- acc }()
	nextAcc := lg.accounts[(acc.i+1)%len(lg.accounts)]

	log.Trace().
		Float64("tokens", tokensPerTransfer).
		Hex("srcAddress", acc.address.Bytes()).
		Hex("dstAddress", nextAcc.address.Bytes()).
		Int("srcAccount", acc.i).
		Int("dstAccount", nextAcc.i).
		Msg("creating transfer script")

	transferTx, err := TokenTransferTransaction(
		lg.fungibleTokenAddress,
		lg.flowTokenAddress,
		nextAcc.address,
		tokensPerTransfer)
	if err != nil {
		log.Error().Err(err).Msg("error creating token transfer script")
		return
	}

	log.Trace().Msg("creating token transfer transaction")
	transferTx = transferTx.
		SetReferenceBlockID(lg.follower.BlockID()).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	log.Trace().Msg("signing transaction")
	err = acc.signTx(transferTx, 0)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	startTime := time.Now()
	ch, err := lg.sendTx(workerID, transferTx)
	if err != nil {
		return
	}

	log.Trace().Hex("txID", transferTx.ID().Bytes()).Msg("transaction sent")

	for {
		select {
		case <-ch:
			log.Trace().
				Hex("txID", transferTx.ID().Bytes()).
				Dur("duration", time.Since(startTime)).
				Msg("transaction confirmed")
			return
		case <-time.After(slowTransactionThreshold):
			log.Warn().
				Hex("txID", transferTx.ID().Bytes()).
				Dur("duration", time.Since(startTime)).
				Int("availableAccounts", len(lg.availableAccounts)).
				Msg("is taking too long")
		}
	}
}

// TODO update this to include loadtype
func (lg *ContLoadGenerator) sendFavContractTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()
	log.Trace().Msg("getting next available account")

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

	log.Trace().Msg("creating transaction")
	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(lg.follower.BlockID()).
		SetScript(txScript).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	log.Trace().Msg("signing transaction")
	err := acc.signTx(tx, 0)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	ch, err := lg.sendTx(workerID, tx)
	if err != nil {
		return
	}
	<-ch
}

func (lg *ContLoadGenerator) sendTx(workerID int, tx *flowsdk.Transaction) (<-chan struct{}, error) {
	log := lg.log.With().Int("workerID", workerID).Str("tx_id", tx.ID().String()).Logger()
	log.Trace().Msg("sending transaction")

	// Add watcher before sending the transaction to avoid race condition
	ch := lg.follower.CompleteChanByID(tx.ID())

	err := lg.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		log.Error().Err(err).Msg("error sending transaction")
		return nil, err
	}

	lg.workerStatsTracker.AddTxSent()
	lg.loaderMetrics.TransactionSent()
	return ch, err
}
