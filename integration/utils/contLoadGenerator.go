package utils

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	flowsdk "github.com/onflow/flow-go-sdk"

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

const slowTransactionThreshold = 30 * time.Second

var accountCreationBatchSize = 250 // a higher number would hit max storage interaction limit
const tokensPerTransfer = 0.01     // flow testnets only have 10e6 total supply, so we choose a small amount here

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
	serviceAccount       *flowAccount
	flowTokenAddress     *flowsdk.Address
	fungibleTokenAddress *flowsdk.Address
	favContractAddress   *flowsdk.Address
	accounts             []*flowAccount
	availableAccounts    chan *flowAccount // queue with accounts available for   workers
	txTracker            *TxTracker
	workerStatsTracker   *WorkerStatsTracker
	workers              []*Worker
	stopped              bool
	loadType             LoadType
	follower             TxFollower
	feedbackEnabled      bool
	availableAccountsLo  int
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
	trackTxs bool,
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

	txStatsTracker := NewTxStatsTracker()
	txTracker, err := NewTxTracker(log, 5000, 100, loadedAccessAddr, time.Second, txStatsTracker)
	if err != nil {
		return nil, err
	}

	var follower TxFollower
	if feedbackEnabled {
		follower, err = NewTxFollower(context.TODO(), supervisorClient, WithLogger(log))
	} else {
		follower, err = NewNopTxFollower(context.TODO(), supervisorClient, WithLogger(log))
	}
	if err != nil {
		return nil, err
	}

	lGen := &ContLoadGenerator{
		log:                  log,
		loaderMetrics:        loaderMetrics,
		initialized:          false,
		tps:                  tps,
		numberOfAccounts:     numberOfAccounts,
		trackTxs:             trackTxs,
		flowClient:           flowClient,
		serviceAccount:       servAcc,
		fungibleTokenAddress: fungibleTokenAddress,
		flowTokenAddress:     flowTokenAddress,
		accounts:             make([]*flowAccount, 0),
		availableAccounts:    make(chan *flowAccount, numberOfAccounts),
		txTracker:            txTracker,
		workerStatsTracker:   NewWorkerStatsTracker(),
		follower:             follower,
		loadType:             loadType,
		feedbackEnabled:      feedbackEnabled,
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
	err := lg.SetupFavContract()
	if err != nil {
		lg.log.Error().Err(err).Msg("failed to setup fav contract")
		return err
	}

	return nil
}

func (lg *ContLoadGenerator) SetupFavContract() error {
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

	lg.sendTx(-1, deploymentTx)

	wait := lg.txTracker.AddTx(
		deploymentTx.ID(),
		txCallbacks{
			onExecuted: func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				lg.log.Debug().
					Str("status", res.Status.String()).
					Msg("fav contract deployment tx executed")

				if res.Error != nil {
					lg.log.Error().
						Err(res.Error).
						Msg("fav contract deployment tx failed")
					err = res.Error
				}
			},
			onExpired: func(_ flowsdk.Identifier) {
				lg.log.Error().Msg("fav contract deployment transaction has expired")
				err = fmt.Errorf("fav contract deployment transaction has expired")
			},
			onTimeout: func(_ flowsdk.Identifier) {
				lg.log.Error().Msg("fav contract deployment transaction has timed out")
				err = fmt.Errorf("fav contract deployment transaction has timed out")
			},
		},
		120*time.Second)

	<-wait

	lg.favContractAddress = acc.address

	return err
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
		SetGasLimit(9999).
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

	lg.serviceAccount.signerLock.Lock()
	err = createAccountTx.SignEnvelope(
		*lg.serviceAccount.address,
		lg.serviceAccount.accountKey.Index,
		lg.serviceAccount.signer,
	)
	if err != nil {
		lg.serviceAccount.signerLock.Unlock()
		return err
	}
	lg.serviceAccount.accountKey.SequenceNumber++
	lg.serviceAccount.signerLock.Unlock()

	err = lg.flowClient.SendTransaction(context.Background(), *createAccountTx)
	if err != nil {
		return err
	}

	executed := make(chan struct{})

	var i int
	log := lg.log.With().Str("tx_id", createAccountTx.ID().String()).Logger()
	wait := lg.txTracker.AddTx(
		createAccountTx.ID(),
		txCallbacks{
			onExecuted: func(txID flowsdk.Identifier, res *flowsdk.TransactionResult) {
				log.Trace().
					Str("status", res.Status.String()).
					Msg("account creation tx executed")

				if res.Error != nil {
					log.Error().
						Err(res.Error).
						Msg("account creation tx failed")
				}

				for _, event := range res.Events {
					log.Trace().
						Str("event_type", event.Type).
						Str("event", event.String()).
						Msg("account creation tx event")

					if event.Type == flowsdk.EventAccountCreated {
						accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
						accountAddress := accountCreatedEvent.Address()

						log.Trace().
							Hex("address", accountAddress.Bytes()).
							Msg("new account created")

						signer, err := crypto.NewInMemorySigner(privKey, accountKey.HashAlgo)
						if err != nil {
							panic(err)
						}

						newAcc := newFlowAccount(i, &accountAddress, accountKey, signer)
						i++

						lg.accounts = append(lg.accounts, newAcc)
						lg.availableAccounts <- newAcc

						log.Trace().
							Hex("address", accountAddress.Bytes()).
							Msg("new account added")
					}
				}
				close(executed)
			},
			onExpired: func(_ flowsdk.Identifier) {
				log.Error().Msg("setup transaction (account creation) has expired")
			},
			onTimeout: func(_ flowsdk.Identifier) {
				log.Error().Msg("setup transaction (account creation) has timed out")
			},
		},
		120*time.Second,
	)

	select {
	case <-wait:
	case <-executed:
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

	log.Trace().Msg("workerID=%d creating transaction")

	addKeysTx.SetReferenceBlockID(lg.follower.BlockID()).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	log.Trace().Msg("workerID=%d signing transaction")
	err = acc.signTx(addKeysTx, 0)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	lg.sendTx(workerID, addKeysTx)
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

	// Wait for completion before sending next transaction to avoid race condition
	ch := lg.follower.CompleteChanByID(transferTx.ID())

	startTime := time.Now()
	lg.sendTx(workerID, transferTx)

	log.Trace().
		Hex("txID", transferTx.ID().Bytes()).
		Msg("transaction sent")

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

	lg.sendTx(workerID, tx)
}

func (lg *ContLoadGenerator) sendTx(workerID int, tx *flowsdk.Transaction) {
	log := lg.log.With().Int("workerID", workerID).Str("tx_id", tx.ID().String()).Logger()
	log.Trace().Msg("sending transaction")

	err := lg.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		log.Error().Err(err).Msg("error sending transaction")
		return
	}

	lg.workerStatsTracker.AddTxSent()
	lg.loaderMetrics.TransactionSent()

	if lg.trackTxs {
		wait := lg.txTracker.AddTx(
			tx.ID(),
			txCallbacks{
				onFinalized: func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
					log.Trace().Str("tx_id", tx.ID().String()).Msg("finalized tx")
				},
				onSealed: func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
					log.Trace().Str("tx_id", tx.ID().String()).Msg("sealed tx")
				},
				onExpired: func(_ flowsdk.Identifier) {
					log.Warn().Str("tx_id", tx.ID().String()).Msg("tx expired")
				},
				onTimeout: func(_ flowsdk.Identifier) {
					log.Warn().Str("tx_id", tx.ID().String()).Msg("tx timed out")
				},
			},
			60*time.Second)
		<-wait
	}
}
