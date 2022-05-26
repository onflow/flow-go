package utils

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
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

var accountCreationBatchSize = 50 // a higher number would hit max storage interaction limit
const tokensPerTransfer = 0.01    // flow testnets only have 10e6 total supply, so we choose a small amount here

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
	availableAccounts    chan *flowAccount                             // queue with accounts available for   workers
	happeningAccounts    chan func() (*flowAccount, string, time.Time) // queue with accounts happening after worker processing
	txTracker            *TxTracker
	txStatsTracker       *TxStatsTracker
	workerStatsTracker   *WorkerStatsTracker
	workers              []*Worker
	blockRef             BlockRef
	stopped              bool
	loadType             LoadType
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
	tps int,
	loadType LoadType,
	feedbackEnabled bool,
) (*ContLoadGenerator, error) {

	multiplier := 10
	if feedbackEnabled {
		multiplier = 20                // bigger if feedbackEnabled otherwise we can run out of accounts for sending transactions
		accountCreationBatchSize = 250 // due to the bigger multiplier we enlarge accountCreationBatchSize to create the accounts faster
	}
	numberOfAccounts := tps * multiplier // 1 second per block, factor 10 for delays to prevent sequence number collisions

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
		happeningAccounts:    make(chan func() (*flowAccount, string, time.Time), numberOfAccounts),
		txTracker:            txTracker,
		txStatsTracker:       txStatsTracker,
		workerStatsTracker:   NewWorkerStatsTracker(),
		blockRef:             NewBlockRef(supervisorClient),
		loadType:             loadType,
		feedbackEnabled:      feedbackEnabled,
		availableAccountsLo:  numberOfAccounts,
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

	lg.sendTx(-1, deploymentTx)

	wg := sync.WaitGroup{}
	wg.Add(1)

	lg.txTracker.AddTx(deploymentTx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			defer wg.Done()

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
		nil, // on sealed
		func(_ flowsdk.Identifier) {
			lg.log.Error().Msg("fav contract deployment transaction has expired")
			err = fmt.Errorf("fav contract deployment transaction has expired")
			wg.Done()
		}, // on expired
		func(_ flowsdk.Identifier) {
			lg.log.Error().Msg("fav contract deployment transaction has timed out")
			err = fmt.Errorf("fav contract deployment transaction has timed out")
			wg.Done()
		}, // on timeout
		func(_ flowsdk.Identifier, terr error) {
			lg.log.Error().Err(terr).Msg("fav contract deployment transaction has encountered an error")
			err = terr
			wg.Done()
		}, // on error
		120)

	wg.Wait()

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

					lg.log.Trace().
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
		lg.log.Error().Err(err).Msgf("workerID=%d error getting reference block", workerID)
		return
	}

	lg.log.Trace().Msgf("workerID=%d getting next available account", workerID)

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()

	lg.log.Trace().Msgf("workerID=%d creating add proposer key script", workerID)
	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := 0; i < numberOfKeysToAdd; i++ {
		cadenceKeys[i] = bytesToCadenceArray(lg.serviceAccount.accountKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error getting add key to account script", workerID)
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
		lg.log.Error().Err(err).Msgf("workerID=%d error constructing add keys to account transaction", workerID)
		return
	}

	lg.log.Trace().Msgf("workerID=%d creating transaction", workerID)

	addKeysTx.SetReferenceBlockID(blockRef).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("workerID=%d signing transaction", workerID)
	err = acc.signTx(addKeysTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error signing transaction", workerID)
		return
	}

	lg.sendTx(workerID, addKeysTx)
}

func (lg *ContLoadGenerator) pushAccountsHappening(workerID int, acc *flowAccount, tx_id string, timeNow time.Time) {
	lg.log.Trace().Msgf("workerID=%d pushing happening address", workerID)
	lg.happeningAccounts <- (func() (*flowAccount, string, time.Time) { return acc, tx_id, timeNow })
}

func (lg *ContLoadGenerator) probeAccountsHappening(workerID int) {
	channelElements := len(lg.happeningAccounts)
	lg.log.Trace().Msgf("workerID=%d probing happening address from channel with %d elements", workerID, channelElements)
	for i := 0; i < channelElements; i++ {
		acc, tx_id, timeAtSendTx := (<-lg.happeningAccounts)()
		txIdFile := fmt.Sprintf("/dev/shm/flow-transaction-feedback/%s/%s/%s.txt", tx_id[0:2], tx_id[2:4], tx_id[4:]) // e.g. /dev/shm/flow-transaction-feedback/70/b9/4146a01bc7843dc9547837e19ba0da6899fdd83ae61e2cf2e3fb4555da6a.txt
		lg.log.Trace().Msgf("workerID=%d probing happening address %x account %d tx_id %s AKA %s", workerID, acc.address.Bytes(), acc.i, tx_id, txIdFile)
		if _, err := os.Stat(txIdFile); errors.Is(err, fs.ErrNotExist) {
			// come here if file does not exist; means account is still happening...
			lg.pushAccountsHappening(workerID, acc, tx_id, timeAtSendTx)

			elapsed := time.Since(timeAtSendTx).Seconds()
			elapsedSuspicious := 30.0
			if (elapsed >= elapsedSuspicious) && (elapsed < (1 + elapsedSuspicious)) {
				// come here to capture transactions which mysteriously have had no feedback after 30 seconds
				// note: this should only be output once as its age reaches and passes through a certain amount of seconds...
				lg.log.Warn().Msgf("workerID=%d address %x account %d tx_id=%s has been happening for %f seconds and counting", workerID, acc.address.Bytes(), acc.i, tx_id, elapsed)
			}
		} else {
			// come here if file exists; means account is good to be used again
			lg.availableAccounts <- acc
			e := os.Remove(txIdFile)
			if e != nil {
				lg.log.Error().Err(err).Msgf("workerID=%d error removing txIdFile=%s", workerID, txIdFile)
				panic("ERROR: failed to remove txIdFile")
			}
		}
	}
}

func (lg *ContLoadGenerator) sendTokenTransferTx(workerID int) {

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error getting reference block", workerID)
		return
	}

	if 0 == workerID {
		if lg.feedbackEnabled {
			// come here if feedbackEnabled so that worker with workerID 0 can probe to see if accounts are no longer happening
			defer lg.probeAccountsHappening(workerID)

			// come here to have workerID 0 keep track of lowest number of available accounts in channel
			if len(lg.availableAccounts) < lg.availableAccountsLo {
				lg.availableAccountsLo = len(lg.availableAccounts)
				lg.log.Debug().Msgf("workerID=%d discovered new availableAccountsLo=%d of numberOfAccounts=%d", workerID, lg.availableAccountsLo, lg.numberOfAccounts)
			}
		}
	}

	lg.log.Trace().Msgf("workerID=%d getting next available account from channel with %d elements", workerID, len(lg.availableAccounts))
	var acc *flowAccount
	var ok bool
	select { // use select so that channel is non-blocking
	case acc, ok = <-lg.availableAccounts:
		if ok {
			// come here if read from next available account channel
		} else {
			// come here if next available account channel closed; in theory this should never happen
			lg.log.Error().Msgf("workerID=%d ERROR: next available account channel closed; skipping send", workerID)
			return
		}
	default:
		// come here if nothing to read from next available account channel; in theory this should never happen
		lg.log.Error().Msgf("workerID=%d ERROR: next available account channel empty; skipping send // channels availableAccounts[%d] happeningAccounts[%d]", workerID, len(lg.availableAccounts), len(lg.happeningAccounts))
		return
	}
	if !lg.feedbackEnabled {
		// come here if feedback disabled and happening accounts get recycled by time... which can result in transaction execution sequence mismatch errors
		defer func() { lg.availableAccounts <- acc }()
	}

	lg.log.Trace().Msgf("workerID=%d getting next account", workerID)
	nextAcc := lg.accounts[(acc.i+1)%len(lg.accounts)]

	lg.log.Trace().Msgf("workerID=%d creating transfer script for tokensPerTransfer %f from address %x to %x account %d to %d", workerID, tokensPerTransfer, acc.address.Bytes(), nextAcc.address.Bytes(), acc.i, nextAcc.i)
	transferTx, err := TokenTransferTransaction(
		lg.fungibleTokenAddress,
		lg.flowTokenAddress,
		nextAcc.address,
		tokensPerTransfer)
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error creating token transfer script", workerID)
		return
	}

	lg.log.Trace().Msgf("workerID=%d creating token transfer transaction", workerID)
	transferTx = transferTx.
		SetReferenceBlockID(blockRef).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("workerID=%d signing transaction", workerID)
	err = acc.signTx(transferTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error signing transaction", workerID)
		return
	}

	lg.sendTx(workerID, transferTx)

	if lg.feedbackEnabled {
		// come here if feedback enable and happening accounts get recycled after checking they make it into a block
		lg.pushAccountsHappening(workerID, acc, transferTx.ID().String(), time.Now())
	}
}

// TODO update this to include loadtype
func (lg *ContLoadGenerator) sendFavContractTx(workerID int) {

	blockRef, err := lg.blockRef.Get()
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error getting reference block", workerID)
		return
	}

	lg.log.Trace().Msgf("workerID=%d getting next available account", workerID)

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

	lg.log.Trace().Msgf("workerID=%d creating transaction", workerID)
	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(blockRef).
		SetScript(txScript).
		SetGasLimit(9999).
		SetProposalKey(*acc.address, 0, acc.seqNumber).
		SetPayer(*acc.address).
		AddAuthorizer(*acc.address)

	lg.log.Trace().Msgf("workerID=%d signing transaction", workerID)
	err = acc.signTx(tx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error signing transaction", workerID)
		return
	}

	lg.sendTx(workerID, tx)
}

func (lg *ContLoadGenerator) sendTx(workerID int, tx *flowsdk.Transaction) {
	// TODO move this as a configurable parameter

	lg.log.Trace().Msgf("workerID=%d sending transaction tx_id=%s", workerID, tx.ID().String())
	err := lg.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		lg.log.Error().Err(err).Msgf("workerID=%d error sending transaction", workerID)
		return
	}

	lg.log.Trace().Msgf("workerID=%d tracking sent transaction", workerID)
	lg.workerStatsTracker.AddTxSent()
	lg.loaderMetrics.TransactionSent()

	if lg.trackTxs {
		stopped := false
		wg := sync.WaitGroup{}
		lg.txTracker.AddTx(tx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", tx.ID().String()).Msgf("workerID=%d finalized tx", workerID)
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on finalized
			func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", tx.ID().String()).Msgf("workerID=%d sealed tx", workerID)
			}, // on sealed
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", tx.ID().String()).Msgf("workerID=%d tx expired", workerID)
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on expired
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", tx.ID().String()).Msgf("workerID=%d tx timed out", workerID)
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on timout
			func(_ flowsdk.Identifier, err error) {
				lg.log.Error().Err(err).Str("tx_id", tx.ID().String()).Msgf("workerID=%d tx error", workerID)
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
