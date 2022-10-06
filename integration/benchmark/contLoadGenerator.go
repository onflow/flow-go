package benchmark

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/cadence"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

type LoadType string

const (
	TokenTransferLoadType LoadType = "token-transfer"
	TokenAddKeysLoadType  LoadType = "add-keys"
	CompHeavyLoadType     LoadType = "computation-heavy"
	EventHeavyLoadType    LoadType = "event-heavy"
	LedgerHeavyLoadType   LoadType = "ledger-heavy"
	ConstExecCostLoadType LoadType = "const-exec" // for an empty transactions with various tx arguments
)

const slowTransactionThreshold = 30 * time.Second

var accountCreationBatchSize = 750 // a higher number would hit max gRPC message size
const tokensPerTransfer = 0.01     // flow testnets only have 10e6 total supply, so we choose a small amount here

// ConstExecParam hosts all parameters for const-exec load type
type ConstExecParams struct {
	MaxTxSizeInByte uint
	AuthAccountNum  uint
	ArgSizeInByte   uint
	PayerKeyCount   uint
}

// ContLoadGenerator creates a continuous load of transactions to the network
// by creating many accounts and transfer flow tokens between them
type ContLoadGenerator struct {
	log                 zerolog.Logger
	loaderMetrics       *metrics.LoaderCollector
	loadParams          LoadParams
	networkParams       NetworkParams
	constExecParams     ConstExecParams
	flowClient          access.Client
	serviceAccount      *flowAccount
	favContractAddress  *flowsdk.Address
	accounts            []*flowAccount
	availableAccounts   chan *flowAccount // queue with accounts available for workers
	workerStatsTracker  *WorkerStatsTracker
	stoppedChannel      chan struct{}
	follower            TxFollower
	availableAccountsLo int
	workFunc            workFunc

	workersMutex sync.Mutex
	workers      []*Worker
}

type NetworkParams struct {
	ServAccPrivKeyHex     string
	ServiceAccountAddress *flowsdk.Address
	FungibleTokenAddress  *flowsdk.Address
	FlowTokenAddress      *flowsdk.Address
}

type LoadParams struct {
	NumberOfAccounts int
	LoadType         LoadType

	// TODO(rbtz): inject a TxFollower
	FeedbackEnabled bool
}

// New returns a new load generator
func New(
	// TODO(rbtz): use context
	ctx context.Context,
	log zerolog.Logger,
	loaderMetrics *metrics.LoaderCollector,
	flowClients []access.Client,
	networkParams NetworkParams,
	loadParams LoadParams,
	constExecParams ConstExecParams,
) (*ContLoadGenerator, error) {
	if len(flowClients) == 0 {
		return nil, errors.New("no flow clients available")
	}
	// TODO(rbtz): add loadbalancing between multiple clients
	flowClient := flowClients[0]

	servAcc, err := loadServiceAccount(flowClient, networkParams.ServiceAccountAddress, networkParams.ServAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	var follower TxFollower
	if loadParams.FeedbackEnabled {
		follower, err = NewTxFollower(context.TODO(), flowClient, WithLogger(log), WithMetrics(loaderMetrics))
	} else {
		follower, err = NewNopTxFollower(context.TODO(), flowClient)
	}
	if err != nil {
		return nil, err
	}

	// check and cap params for const-exec mode
	if loadParams.LoadType == ConstExecCostLoadType {
		if constExecParams.MaxTxSizeInByte > flow.DefaultMaxTransactionByteSize {
			errMsg := fmt.Sprintf("MaxTxSizeInByte(%d) is larger than DefaultMaxTransactionByteSize(%d).",
				constExecParams.MaxTxSizeInByte,
				flow.DefaultMaxTransactionByteSize)
			log.Error().Msg(errMsg)

			return nil, errors.New(errMsg)
		}

		// accounts[0] will be used as the proposer\payer
		if constExecParams.AuthAccountNum > uint(loadParams.NumberOfAccounts-1) {
			errMsg := fmt.Sprintf("Number of authorizer(%d) is larger than max possible(%d).",
				constExecParams.AuthAccountNum,
				loadParams.NumberOfAccounts-1)
			log.Error().Msg(errMsg)

			return nil, errors.New(errMsg)
		}

		if constExecParams.ArgSizeInByte > flow.DefaultMaxTransactionByteSize {
			errMsg := fmt.Sprintf("ArgSizeInByte(%d) is larger than DefaultMaxTransactionByteSize(%d).",
				constExecParams.ArgSizeInByte,
				flow.DefaultMaxTransactionByteSize)
			log.Error().Msg(errMsg)
			return nil, errors.New(errMsg)
		}
	}

	lg := &ContLoadGenerator{
		log:                 log,
		loaderMetrics:       loaderMetrics,
		loadParams:          loadParams,
		networkParams:       networkParams,
		constExecParams:     constExecParams,
		flowClient:          flowClient,
		serviceAccount:      servAcc,
		accounts:            make([]*flowAccount, 0),
		availableAccounts:   make(chan *flowAccount, loadParams.NumberOfAccounts),
		workerStatsTracker:  NewWorkerStatsTracker(),
		follower:            follower,
		availableAccountsLo: loadParams.NumberOfAccounts,
		stoppedChannel:      make(chan struct{}),
	}

	// TODO(rbtz): hide load implementation behind an interface
	switch loadParams.LoadType {
	case TokenTransferLoadType:
		lg.workFunc = lg.sendTokenTransferTx
	case TokenAddKeysLoadType:
		lg.workFunc = lg.sendAddKeyTx
	case ConstExecCostLoadType:
		lg.workFunc = lg.sendConstExecCostTx
	case CompHeavyLoadType, EventHeavyLoadType, LedgerHeavyLoadType:
		lg.workFunc = lg.sendFavContractTx
	default:
		return nil, fmt.Errorf("unknown load type: %s", loadParams.LoadType)
	}

	return lg, nil
}

func (lg *ContLoadGenerator) stopped() bool {
	select {
	case <-lg.stoppedChannel:
		return true
	default:
		return false
	}
}

// TODO(rbtz): make part of New
func (lg *ContLoadGenerator) Init() error {
	for i := 0; i < lg.loadParams.NumberOfAccounts; i += accountCreationBatchSize {
		if lg.stopped() {
			return nil
		}

		num := lg.loadParams.NumberOfAccounts - i
		if num > accountCreationBatchSize {
			num = accountCreationBatchSize
		}

		lg.log.Info().Int("cumulative", i).Int("num", num).Int("numberOfAccounts", lg.loadParams.NumberOfAccounts).Msg("creating accounts")
		err := lg.createAccounts(num)
		if err != nil {
			return err
		}
	}

	// TODO(rbtz): create an interface for different load types: Setup()
	if lg.loadParams.LoadType != ConstExecCostLoadType {
		err := lg.setupFavContract()
		if err != nil {
			lg.log.Error().Err(err).Msg("failed to setup fav contract")
			return err
		}
	} else {
		lg.log.Info().Int("numberOfAccountsCreated", len(lg.accounts)).
			Msg("new accounts created. Grabbing the first as the proposer/payer " +
				"and adding multiple keys to that account")

		err := lg.addKeysToProposerAccount(lg.accounts[0])
		if err != nil {
			lg.log.Error().Msg("failed to create add-key transaction for const-exec")
			return err
		}
	}

	lg.workerStatsTracker.StartPrinting(1 * time.Second)

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
	lg.workerStatsTracker.IncTxExecuted()

	lg.favContractAddress = acc.address
	return nil
}

func (lg *ContLoadGenerator) startWorkers(num int) error {
	for i := 0; i < num; i++ {
		worker := NewWorker(len(lg.workers), 1*time.Second, lg.workFunc)
		worker.Start()
		lg.workers = append(lg.workers, worker)
	}
	lg.workerStatsTracker.AddWorkers(num)
	return nil
}

func (lg *ContLoadGenerator) stopWorkers(num int) error {
	if num > len(lg.workers) {
		return fmt.Errorf("can't stop %d workers, only %d available", num, len(lg.workers))
	}

	wg := sync.WaitGroup{}
	wg.Add(num)

	idx := len(lg.workers) - num
	toRemove := lg.workers[idx:]
	lg.workers = lg.workers[:idx]

	for _, w := range toRemove {
		go func(w *Worker) {
			defer wg.Done()
			lg.log.Debug().Int("workerID", w.workerID).Msg("stopping worker")
			w.Stop()
		}(w)
	}
	wg.Wait()
	lg.workerStatsTracker.AddWorkers(-num)

	return nil
}

// SetTPS compares the given TPS to the current TPS to determine whether to increase
// or decrease the load.
// It increases/decreases the load by adjusting the number of workers, since each worker
// is responsible for sending the load at 1 TPS.
func (lg *ContLoadGenerator) SetTPS(desired uint) error {
	lg.workersMutex.Lock()
	defer lg.workersMutex.Unlock()

	currentTPS := len(lg.workers)
	diff := int(desired) - currentTPS

	switch {
	case diff > 0:
		return lg.startWorkers(diff)
	case diff < 0:
		return lg.stopWorkers(-diff)
	}
	return nil
}

func (lg *ContLoadGenerator) Stop() {
	if lg.stopped() {
		lg.log.Warn().Msg("Stop() called on generator when already stopped")
		return
	}

	defer lg.log.Debug().Msg("stopped generator")

	lg.log.Debug().Msg("stopping workers")
	_ = lg.SetTPS(0)
	lg.workerStatsTracker.StopPrinting()
	lg.log.Debug().Msg("stopping follower")
	lg.follower.Stop()
	close(lg.stoppedChannel)
}

func (lg *ContLoadGenerator) Done() <-chan struct{} {
	return lg.stoppedChannel
}

func (lg *ContLoadGenerator) GetTxExecuted() int {
	return lg.workerStatsTracker.GetTxExecuted()
}

func (lg *ContLoadGenerator) AvgTpsBetween(start, stop time.Time) float64 {
	return lg.workerStatsTracker.AvgTPSBetween(start, stop)
}

func (lg *ContLoadGenerator) createAccounts(num int) error {
	privKey := randomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(privKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)

	// Generate an account creation script
	createAccountTx := flowsdk.NewTransaction().
		SetScript(CreateAccountsScript(*lg.networkParams.FungibleTokenAddress, *lg.networkParams.FlowTokenAddress)).
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
	lg.workerStatsTracker.IncTxExecuted()

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

func (lg *ContLoadGenerator) createAddKeyTx(accountAddress flowsdk.Address, numberOfKeysToAdd uint) (*flowsdk.Transaction, error) {
	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := uint(0); i < numberOfKeysToAdd; i++ {
		cadenceKeys[i] = bytesToCadenceArray(lg.serviceAccount.accountKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		log.Error().Err(err).Msg("error getting add key to account script")
		return nil, err
	}

	addKeysTx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		AddAuthorizer(accountAddress).
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
		return nil, err
	}

	return addKeysTx, nil

}

func (lg *ContLoadGenerator) sendAddKeyTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	// TODO move this as a configurable parameter
	numberOfKeysToAdd := uint(40)

	log.Trace().Msg("getting next available account")

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()

	log.Trace().Msg("creating add proposer key script")

	addKeysTx, err := lg.createAddKeyTx(*acc.address, numberOfKeysToAdd)
	if err != nil {
		log.Error().Err(err).Msg("error creating AddKey transaction")
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
	lg.workerStatsTracker.IncTxExecuted()
}

func (lg *ContLoadGenerator) addKeysToProposerAccount(proposerPayerAccount *flowAccount) error {
	if proposerPayerAccount == nil {
		return errors.New("proposerPayerAccount is nil")
	}

	addKeysToPayerTx, err := lg.createAddKeyTx(*lg.accounts[0].address, lg.constExecParams.PayerKeyCount)
	if err != nil {
		lg.log.Error().Msg("failed to create add-key transaction for const-exec")
		return err
	}
	addKeysToPayerTx.SetReferenceBlockID(lg.follower.BlockID()).
		SetProposalKey(*lg.accounts[0].address, 0, lg.accounts[0].seqNumber).
		SetPayer(*lg.accounts[0].address)

	lg.log.Info().Msg("signing the add-key transaction for const-exec")
	err = lg.accounts[0].signTx(addKeysToPayerTx, 0)
	if err != nil {
		lg.log.Error().Err(err).Msg("error signing the add-key transaction for const-exec")
		return err
	}

	lg.log.Info().Msg("issuing the add-key transaction for const-exec")
	ch, err := lg.sendTx(0, addKeysToPayerTx)
	if err != nil {
		return err
	}
	<-ch
	lg.workerStatsTracker.IncTxExecuted()

	lg.log.Info().Msg("the add-key transaction for const-exec is done")
	return nil
}

func (lg *ContLoadGenerator) sendConstExecCostTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	txScriptNoComment := ConstExecCostTransaction(lg.constExecParams.AuthAccountNum, 0)

	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(lg.follower.BlockID()).
		SetScript(txScriptNoComment).
		SetGasLimit(10). // const-exec tx has empty transaction
		SetProposalKey(*lg.accounts[0].address, 0, lg.accounts[0].seqNumber).
		SetPayer(*lg.accounts[0].address)
	lg.accounts[0].seqNumber += 1

	txArgStr := generateRandomStringWithLen(lg.constExecParams.ArgSizeInByte)
	txArg, err := cadence.NewString(txArgStr)
	if err != nil {
		log.Trace().Msg("Failed to generate cadence String parameter. Using empty string.")
	}
	err = tx.AddArgument(txArg)
	if err != nil {
		log.Trace().Msg("Failed to add argument. Skipping.")
	}

	// Add authorizers. lg.accounts[0] used as proposer\payer
	log.Trace().Msg("Adding tx authorizers")
	for i := uint(1); i < lg.constExecParams.AuthAccountNum+1; i++ {
		tx = tx.AddAuthorizer(*lg.accounts[i].address)
	}

	log.Trace().Msg("Authorizers signing tx")
	for i := uint(1); i < lg.constExecParams.AuthAccountNum+1; i++ {
		err := lg.accounts[i].signPayload(tx, 0)
		if err != nil {
			log.Error().Err(err).Msg("error signing payload")
			return
		}
	}

	log.Trace().Msg("Payer signing tx")
	for i := uint(0); i < lg.constExecParams.PayerKeyCount; i++ {
		err = lg.accounts[0].signTx(tx, int(i))
		if err != nil {
			log.Error().Err(err).Msg("error signing transaction")
			return
		}
	}

	// calculate RLP-encoded binary size of the transaction without comment
	txSizeWithoutComment := uint(len(tx.Encode()))
	if txSizeWithoutComment > lg.constExecParams.MaxTxSizeInByte {
		log.Error().Msg(fmt.Sprintf("current tx size(%d) without comment "+
			"is larger than max tx size configured(%d)",
			txSizeWithoutComment, lg.constExecParams.MaxTxSizeInByte))
		return
	}

	// now adding comment to fulfill the final transaction size
	commentSizeInByte := lg.constExecParams.MaxTxSizeInByte - txSizeWithoutComment
	txScriptWithComment := ConstExecCostTransaction(lg.constExecParams.AuthAccountNum, commentSizeInByte)
	tx = tx.SetScript(txScriptWithComment)

	txSizeWithComment := uint(len(tx.Encode()))
	log.Trace().Uint("Max Tx Size", lg.constExecParams.MaxTxSizeInByte).
		Uint("Actual Tx Size", txSizeWithComment).
		Uint("Tx Arg Size", lg.constExecParams.ArgSizeInByte).
		Uint("Num of Authorizers", lg.constExecParams.AuthAccountNum).
		Uint("Num of payer keys", lg.constExecParams.PayerKeyCount).
		Uint("Script comment length", commentSizeInByte).
		Msg("Generating one const-exec transaction")

	log.Trace().Msg("Issuing tx")
	ch, err := lg.sendTx(workerID, tx)
	if err != nil {
		log.Error().Err(err).Msg("const-exec tx failed")
		return
	}
	<-ch
	lg.workerStatsTracker.IncTxExecuted()

	log.Trace().Msg("const-exec tx suceeded")
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
			log.Debug().Int("availableAccountsLo", l).Int("numberOfAccounts", lg.loadParams.NumberOfAccounts).Msg("discovered new account low")
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
		lg.networkParams.FungibleTokenAddress,
		lg.networkParams.FlowTokenAddress,
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
			lg.workerStatsTracker.IncTxExecuted()
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

	switch lg.loadParams.LoadType {
	case CompHeavyLoadType:
		txScript = ComputationHeavyScript(*lg.favContractAddress)
	case EventHeavyLoadType:
		txScript = EventHeavyScript(*lg.favContractAddress)
	case LedgerHeavyLoadType:
		txScript = LedgerHeavyScript(*lg.favContractAddress)
	default:
		log.Error().Msg("unknown load type")
		return
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
	ch := lg.follower.Follow(tx.ID())

	err := lg.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		log.Error().Err(err).Msg("error sending transaction")
		return nil, err
	}

	lg.workerStatsTracker.AddTxSent()
	lg.loaderMetrics.TransactionSent()
	return ch, err
}
