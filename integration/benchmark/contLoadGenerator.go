package benchmark

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/cadence"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/module/metrics"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	evmTypes "github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

type LoadType string

const (
	TokenTransferLoadType LoadType = "token-transfer"
	TokenAddKeysLoadType  LoadType = "add-keys"
	CompHeavyLoadType     LoadType = "computation-heavy"
	EventHeavyLoadType    LoadType = "event-heavy"
	LedgerHeavyLoadType   LoadType = "ledger-heavy"
	ConstExecCostLoadType LoadType = "const-exec" // for an empty transactions with various tx arguments
	ExecDataHeavyLoadType LoadType = "exec-data-heavy"
	EVMLoadType           LoadType = "evm"
)

const lostTransactionThreshold = 90 * time.Second

var accountCreationBatchSize = 750 // a higher number would hit max gRPC message size

const (
	// flow testnets only have 10e6 total supply, so we choose a small amounts here
	tokensPerTransfer = 0.000001
	tokensPerAccount  = 10
)

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
	ctx context.Context

	log                zerolog.Logger
	loaderMetrics      *metrics.LoaderCollector
	loadParams         LoadParams
	networkParams      NetworkParams
	constExecParams    ConstExecParams
	flowClient         access.Client
	serviceAccount     *account.FlowAccount
	favContractAddress *flowsdk.Address
	availableAccounts  chan *account.FlowAccount // queue with accounts available for workers
	workerStatsTracker *WorkerStatsTracker
	stoppedChannel     chan struct{}
	follower           TxFollower
	workFunc           workFunc

	workersMutex sync.Mutex
	workers      []*Worker

	accountsMutex sync.Mutex
	accounts      []*account.FlowAccount
}

type NetworkParams struct {
	ServAccPrivKeyHex     string
	ServiceAccountAddress *flowsdk.Address
	FungibleTokenAddress  *flowsdk.Address
	FlowTokenAddress      *flowsdk.Address
	ChainId               flow.ChainID
}

type LoadParams struct {
	NumberOfAccounts int
	LoadType         LoadType

	// TODO(rbtz): inject a TxFollower
	FeedbackEnabled bool
}

// New returns a new load generator
func New(
	ctx context.Context,
	log zerolog.Logger,
	workerStatsTracker *WorkerStatsTracker,
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

	servAcc, err := account.LoadServiceAccount(ctx, flowClient, networkParams.ServiceAccountAddress, networkParams.ServAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error loading service account %w", err)
	}

	var follower TxFollower
	if loadParams.FeedbackEnabled {
		follower, err = NewTxFollower(ctx, flowClient, WithLogger(log), WithMetrics(loaderMetrics))
	} else {
		follower, err = NewNopTxFollower(ctx, flowClient)
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
		ctx:                ctx,
		log:                log,
		loaderMetrics:      loaderMetrics,
		loadParams:         loadParams,
		networkParams:      networkParams,
		constExecParams:    constExecParams,
		flowClient:         flowClient,
		serviceAccount:     servAcc,
		accounts:           make([]*account.FlowAccount, 0),
		availableAccounts:  make(chan *account.FlowAccount, loadParams.NumberOfAccounts),
		workerStatsTracker: workerStatsTracker,
		follower:           follower,
		stoppedChannel:     make(chan struct{}),
	}

	lg.log.Info().Int("num_keys", lg.serviceAccount.NumKeys()).Msg("service account loaded")

	// TODO(rbtz): hide load implementation behind an interface
	switch loadParams.LoadType {
	case TokenTransferLoadType:
		lg.workFunc = lg.sendTokenTransferTx
	case TokenAddKeysLoadType:
		lg.workFunc = lg.sendAddKeyTx
	case ConstExecCostLoadType:
		lg.workFunc = lg.sendConstExecCostTx
	case CompHeavyLoadType, EventHeavyLoadType, LedgerHeavyLoadType, ExecDataHeavyLoadType:
		lg.workFunc = lg.sendFavContractTx
	case EVMLoadType:
		lg.workFunc = lg.sendEVMTx
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

func (lg *ContLoadGenerator) populateServiceAccountKeys(num int) error {
	if lg.serviceAccount.NumKeys() >= num {
		return nil
	}

	key1, _ := lg.serviceAccount.GetKey()
	lg.log.Info().
		Stringer("HashAlgo", key1.HashAlgo).
		Stringer("SigAlgo", key1.SigAlgo).
		Int("Index", key1.Index).
		Int("Weight", key1.Weight).
		Msg("service account info")
	key1.Done()

	numberOfKeysToAdd := num - lg.serviceAccount.NumKeys()

	lg.log.Info().Int("num_keys_to_add", numberOfKeysToAdd).Msg("adding keys to service account")

	addKeysTx, err := lg.createAddKeyTx(*lg.serviceAccount.Address, uint(numberOfKeysToAdd))
	if err != nil {
		return fmt.Errorf("error creating add key tx: %w", err)
	}

	addKeysTx.SetReferenceBlockID(lg.follower.BlockID())

	key, err := lg.serviceAccount.GetKey()
	if err != nil {
		return fmt.Errorf("error getting service account key: %w", err)
	}
	defer key.Done()

	err = key.SignTx(addKeysTx)
	if err != nil {
		return fmt.Errorf("error signing transaction: %w", err)
	}

	ch, err := lg.sendTx(0, addKeysTx)
	if err != nil {
		return fmt.Errorf("error sending transaction: %w", err)
	}
	defer key.IncrementSequenceNumber()

	var result flowsdk.TransactionResult
	select {
	case result = <-ch:
	case <-lg.Done():
		return fmt.Errorf("load generator stopped")
	}

	lg.log.Info().Stringer("result", result.Status).Msg("add key tx")
	if result.Error != nil {
		return fmt.Errorf("error adding keys to service account: %w", result.Error)
	}

	// reload service account until it has enough keys
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for service account to have %d keys", num)
		case <-lg.Done():
			return fmt.Errorf("load generator stopped")
		default:
		}

		lg.serviceAccount, err = account.LoadServiceAccount(lg.ctx, lg.flowClient, lg.serviceAccount.Address, lg.networkParams.ServAccPrivKeyHex)
		if err != nil {
			return fmt.Errorf("error loading service account %w", err)
		}
		lg.log.Info().Int("num_keys", lg.serviceAccount.NumKeys()).Msg("service account reloaded")

		if lg.serviceAccount.NumKeys() >= num {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

// TODO(rbtz): make part of New
func (lg *ContLoadGenerator) Init() error {
	err := lg.populateServiceAccountKeys(50)
	if err != nil {
		return fmt.Errorf("error populating service account keys: %w", err)
	}

	g := errgroup.Group{}
	for i := 0; i < lg.loadParams.NumberOfAccounts; i += accountCreationBatchSize {
		i := i
		g.Go(func() error {
			if lg.stopped() {
				return lg.ctx.Err()
			}

			num := lg.loadParams.NumberOfAccounts - i
			if num > accountCreationBatchSize {
				num = accountCreationBatchSize
			}

			lg.log.Info().Int("cumulative", i).Int("num", num).Int("numberOfAccounts", lg.loadParams.NumberOfAccounts).Msg("creating accounts")
			for {
				err := lg.createAccounts(num)
				if errors.Is(err, account.ErrNoKeysAvailable) {
					lg.log.Warn().Err(err).Msg("error creating accounts, retrying...")
					time.Sleep(1 * time.Second)
					continue
				}
				return err
			}
		})
		// This is needed to avoid hitting the gRPC message size limit.
		time.Sleep(1 * time.Second)
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error creating accounts: %w", err)
	}

	// TODO(rbtz): create an interface for different load types: Setup()
	if lg.loadParams.LoadType != ConstExecCostLoadType {
		err := lg.setupFavContract()
		if err != nil {
			lg.log.Error().Err(err).Msg("failed to setup fav contract")
			return err
		}
		err = lg.setupEVMAccount()
		if err != nil {
			lg.log.Error().Err(err).Msg("failed to setup evm account")
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

	return nil
}

func (lg *ContLoadGenerator) setupEVMAccount() error {

	chain := lg.networkParams.ChainId.Chain()

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	// generate test address
	addressBytes, err := hex.DecodeString("3da9cb19b06A645BA6F12F79D8d7fcdd8A00cfD4")
	if err != nil {
		return err
	}
	addressCadenceBytes := make([]cadence.Value, 20)
	for i := range addressCadenceBytes {
		addressCadenceBytes[i] = cadence.UInt8(addressBytes[i])
	}
	addressArg := cadence.NewArray(addressCadenceBytes).WithType(stdlib.EVMAddressBytesCadenceType)

	amountArg, err := cadence.NewUFix64("10000.0")
	if err != nil {
		return err
	}

	// Fund evm address
	txBody := flowsdk.NewTransaction().
		SetScript([]byte(fmt.Sprintf(
			`
						import EVM from %s
						import FungibleToken from %s
						import FlowToken from %s

						transaction(address: [UInt8; 20], amount: UFix64) {
							let fundVault: @FlowToken.Vault

							prepare(signer: AuthAccount) {
								let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
									?? panic("Could not borrow reference to the owner's Vault!")
						
								// 1.0 Flow for the EVM gass fees
								self.fundVault <- vaultRef.withdraw(amount: amount+1.0) as! @FlowToken.Vault
							}

							execute {
								let acc <- EVM.createBridgedAccount()
								acc.deposit(from: <-self.fundVault)
								let fundAddress = EVM.EVMAddress(bytes: address)
								acc.call(
										to: fundAddress,
										data: [],
										gasLimit: 21000,
										value: EVM.Balance(flow: amount))

								destroy acc
							}
						}
					`,
			sc.FlowServiceAccount.Address.HexWithPrefix(),
			sc.FungibleToken.Address.HexWithPrefix(),
			sc.FlowToken.Address.HexWithPrefix(),
		))).
		SetReferenceBlockID(lg.follower.BlockID()).
		SetComputeLimit(9999)

	err = txBody.AddArgument(addressArg)
	if err != nil {
		return err
	}
	err = txBody.AddArgument(amountArg)
	if err != nil {
		return err
	}

	key, err := lg.serviceAccount.GetKey()
	if err != nil {
		lg.log.Error().Err(err).Msg("error getting key")
		return err
	}
	defer key.Done()

	err = key.SignTx(txBody)
	if err != nil {
		return err
	}

	// Do not wait for the transaction to be sealed.
	ch, err := lg.sendTx(-1, txBody)
	if err != nil {
		return err
	}
	defer key.IncrementSequenceNumber()

	res := <-ch
	if res.Error != nil {
		return res.Error
	}

	lg.workerStatsTracker.IncTxExecuted()
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
		SetComputeLimit(9999)

	lg.log.Trace().Msg("signing transaction")

	key, err := acc.GetKey()
	if err != nil {
		lg.log.Error().Err(err).Msg("error getting key")
		return err
	}
	defer key.Done()

	err = key.SignTx(deploymentTx)
	if err != nil {
		lg.log.Error().Err(err).Msg("error signing transaction")
		return err
	}

	ch, err := lg.sendTx(-1, deploymentTx)
	if err != nil {
		return err
	}
	defer key.IncrementSequenceNumber()

	<-ch
	lg.workerStatsTracker.IncTxExecuted()

	lg.favContractAddress = acc.Address
	return nil
}

func (lg *ContLoadGenerator) startWorkers(num int) error {
	for i := 0; i < num; i++ {
		worker := NewWorker(len(lg.workers), 1*time.Second, lg.workFunc)
		lg.log.Trace().Int("workerID", worker.workerID).Msg("starting worker")
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

	idx := len(lg.workers) - num
	toRemove := lg.workers[idx:]
	lg.workers = lg.workers[:idx]

	for _, w := range toRemove {
		go func(w *Worker) {
			lg.log.Trace().Int("workerID", w.workerID).Msg("stopping worker")
			w.Stop()
		}(w)
	}
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

	if lg.stopped() {
		return fmt.Errorf("SetTPS called after loader is stopped: %w", context.Canceled)
	}

	return lg.unsafeSetTPS(desired)
}

func (lg *ContLoadGenerator) unsafeSetTPS(desired uint) error {
	currentTPS := len(lg.workers)
	diff := int(desired) - currentTPS

	var err error
	switch {
	case diff > 0:
		err = lg.startWorkers(diff)
	case diff < 0:
		err = lg.stopWorkers(-diff)
	}

	if err == nil {
		lg.loaderMetrics.SetTPSConfigured(desired)
	}
	return err
}

func (lg *ContLoadGenerator) Stop() {
	lg.workersMutex.Lock()
	defer lg.workersMutex.Unlock()

	if lg.stopped() {
		lg.log.Warn().Msg("Stop() called on generator when already stopped")
		return
	}

	defer lg.log.Debug().Msg("stopped generator")

	lg.log.Debug().Msg("stopping follower")
	lg.follower.Stop()
	lg.log.Debug().Msg("stopping workers")
	_ = lg.unsafeSetTPS(0)
	close(lg.stoppedChannel)
}

func (lg *ContLoadGenerator) Done() <-chan struct{} {
	return lg.stoppedChannel
}

func (lg *ContLoadGenerator) createAccounts(num int) error {
	privKey := account.RandomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(privKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)

	// Generate an account creation script
	createAccountTx := flowsdk.NewTransaction().
		SetScript(CreateAccountsScript(*lg.networkParams.FungibleTokenAddress, *lg.networkParams.FlowTokenAddress)).
		SetReferenceBlockID(lg.follower.BlockID()).
		SetComputeLimit(999999)

	publicKey := bytesToCadenceArray(accountKey.PublicKey.Encode())
	count := cadence.NewInt(num)

	initialTokenAmount, err := cadence.NewUFix64FromParts(
		tokensPerAccount,
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

	key, err := lg.serviceAccount.GetKey()
	if err != nil {
		lg.log.Error().Err(err).Msg("error getting key")
		return err
	}
	defer key.Done()

	err = key.SignTx(createAccountTx)
	if err != nil {
		return err
	}

	// Do not wait for the transaction to be sealed.
	ch, err := lg.sendTx(-1, createAccountTx)
	if err != nil {
		return err
	}
	defer key.IncrementSequenceNumber()

	var result flowsdk.TransactionResult
	select {
	case result = <-ch:
		lg.workerStatsTracker.IncTxExecuted()
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout waiting for account creation tx to be executed")
	case <-lg.Done():
		return fmt.Errorf("loader stopped while waiting for account creation tx to be executed")
	}

	log := lg.log.With().Str("tx_id", createAccountTx.ID().String()).Logger()
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

			newAcc, err := account.New(accountsCreated, &accountAddress, privKey, []*flowsdk.AccountKey{accountKey})
			if err != nil {
				return fmt.Errorf("failed to create account: %w", err)
			}
			accountsCreated++

			lg.accountsMutex.Lock()
			lg.accounts = append(lg.accounts, newAcc)
			lg.accountsMutex.Unlock()
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

	key, err := lg.serviceAccount.GetKey()
	if err != nil {
		return nil, err
	}
	key.Done() // we don't actually need it

	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := uint(0); i < numberOfKeysToAdd; i++ {
		accountKey := key.AccountKey
		cadenceKeys[i] = bytesToCadenceArray(accountKey.PublicKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		lg.log.Error().Err(err).Msg("error getting add key to account script")
		return nil, err
	}

	addKeysTx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		AddAuthorizer(accountAddress).
		SetReferenceBlockID(lg.follower.BlockID()).
		SetComputeLimit(9999)

	err = addKeysTx.AddArgument(cadenceKeysArray)
	if err != nil {
		lg.log.Error().Err(err).Msg("error constructing add keys to account transaction")
		return nil, err
	}

	return addKeysTx, nil
}

func (lg *ContLoadGenerator) sendAddKeyTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	// TODO move this as a configurable parameter
	numberOfKeysToAdd := uint(50)

	log.Trace().Msg("getting next available account")

	acc := <-lg.availableAccounts
	defer func() { lg.availableAccounts <- acc }()

	log.Trace().Msg("creating add proposer key script")

	addKeysTx, err := lg.createAddKeyTx(*acc.Address, numberOfKeysToAdd)
	if err != nil {
		log.Error().Err(err).Msg("error creating AddKey transaction")
		return
	}

	log.Trace().Msg("creating transaction")

	addKeysTx.SetReferenceBlockID(lg.follower.BlockID())

	log.Trace().Msg("signing transaction")

	key, err := acc.GetKey()
	if err != nil {
		log.Error().Err(err).Msg("error getting service account key")
		return
	}
	defer key.Done()

	err = key.SignTx(addKeysTx)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	ch, err := lg.sendTx(workerID, addKeysTx)
	if err != nil {
		return
	}
	defer key.IncrementSequenceNumber()
	<-ch
	lg.workerStatsTracker.IncTxExecuted()
}

func (lg *ContLoadGenerator) addKeysToProposerAccount(proposerPayerAccount *account.FlowAccount) error {
	if proposerPayerAccount == nil {
		return errors.New("proposerPayerAccount is nil")
	}

	addKeysToPayerTx, err := lg.createAddKeyTx(*lg.accounts[0].Address, lg.constExecParams.PayerKeyCount)
	if err != nil {
		lg.log.Error().Msg("failed to create add-key transaction for const-exec")
		return err
	}
	addKeysToPayerTx.SetReferenceBlockID(lg.follower.BlockID())

	lg.log.Info().Msg("signing the add-key transaction for const-exec")

	key, err := lg.accounts[0].GetKey()
	if err != nil {
		lg.log.Error().Err(err).Msg("error getting key")
		return err
	}
	defer key.Done()

	err = key.SignTx(addKeysToPayerTx)
	if err != nil {
		lg.log.Error().Err(err).Msg("error signing the add-key transaction for const-exec")
		return err
	}

	lg.log.Info().Msg("issuing the add-key transaction for const-exec")
	ch, err := lg.sendTx(0, addKeysToPayerTx)
	if err != nil {
		return err
	}
	defer key.IncrementSequenceNumber()

	<-ch
	lg.workerStatsTracker.IncTxExecuted()

	lg.log.Info().Msg("the add-key transaction for const-exec is done")
	return nil
}

func (lg *ContLoadGenerator) sendConstExecCostTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	txScriptNoComment := ConstExecCostTransaction(lg.constExecParams.AuthAccountNum, 0)

	proposerKey, err := lg.accounts[0].GetKey()
	if err != nil {
		log.Error().Err(err).Msg("error getting key")
		return
	}
	defer proposerKey.Done()

	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(lg.follower.BlockID()).
		SetScript(txScriptNoComment).
		SetComputeLimit(10). // const-exec tx has empty transaction
		SetProposalKey(*proposerKey.Address, proposerKey.Index, proposerKey.SequenceNumber).
		SetPayer(*proposerKey.Address)

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
		tx = tx.AddAuthorizer(*lg.accounts[i].Address)
	}

	log.Trace().Msg("Authorizers signing tx")
	for i := uint(1); i < lg.constExecParams.AuthAccountNum+1; i++ {
		key, err := lg.accounts[i].GetKey()
		if err != nil {
			log.Error().Err(err).Msg("error getting key")
			return
		}

		err = key.SignPayload(tx)
		key.Done() // authorizers don't need to increment their sequence number

		if err != nil {
			log.Error().Err(err).Msg("error signing payload")
			return
		}
	}

	log.Trace().Msg("Payer signing tx")
	for i := uint(1); i < lg.constExecParams.PayerKeyCount; i++ {
		key, err := lg.accounts[i].GetKey()
		if err != nil {
			log.Error().Err(err).Msg("error getting key")
			return
		}

		err = tx.SignEnvelope(*key.Address, key.Index, key.Signer)
		key.Done() // payers don't need to increment their sequence number

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
	defer proposerKey.IncrementSequenceNumber()

	<-ch
	lg.workerStatsTracker.IncTxExecuted()

	log.Trace().Msg("const-exec tx suceeded")
}

func (lg *ContLoadGenerator) sendTokenTransferTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	log.Trace().
		Int("availableAccounts", len(lg.availableAccounts)).
		Msg("getting next available account")

	var acc *account.FlowAccount

	select {
	case acc = <-lg.availableAccounts:
	default:
		log.Error().Msg("next available account channel empty; skipping send")
		return
	}
	defer func() { lg.availableAccounts <- acc }()
	nextAcc := lg.accounts[(acc.ID+1)%len(lg.accounts)]

	log.Trace().
		Float64("tokens", tokensPerTransfer).
		Hex("srcAddress", acc.Address.Bytes()).
		Hex("dstAddress", nextAcc.Address.Bytes()).
		Int("srcAccount", acc.ID).
		Int("dstAccount", nextAcc.ID).
		Msg("creating transfer script")

	transferTx, err := TokenTransferTransaction(
		lg.networkParams.FungibleTokenAddress,
		lg.networkParams.FlowTokenAddress,
		nextAcc.Address,
		tokensPerTransfer)
	if err != nil {
		log.Error().Err(err).Msg("error creating token transfer script")
		return
	}

	log.Trace().Msg("creating token transfer transaction")
	transferTx = transferTx.
		SetReferenceBlockID(lg.follower.BlockID()).
		SetComputeLimit(9999)

	log.Trace().Msg("signing transaction")

	key, err := acc.GetKey()
	if err != nil {
		log.Error().Err(err).Msg("error getting key")
		return
	}
	defer key.Done()

	err = key.SignTx(transferTx)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	startTime := time.Now()
	ch, err := lg.sendTx(workerID, transferTx)
	if err != nil {
		return
	}
	defer key.IncrementSequenceNumber()

	log = log.With().Hex("tx_id", transferTx.ID().Bytes()).Logger()
	log.Trace().Msg("transaction sent")

	t := time.NewTimer(lostTransactionThreshold)
	defer t.Stop()

	select {
	case result := <-ch:
		if result.Error != nil {
			lg.workerStatsTracker.IncTxFailed()
		}
		log.Trace().
			Dur("duration", time.Since(startTime)).
			Err(result.Error).
			Str("status", result.Status.String()).
			Msg("transaction confirmed")
	case <-t.C:
		lg.loaderMetrics.TransactionLost()
		log.Warn().
			Dur("duration", time.Since(startTime)).
			Int("availableAccounts", len(lg.availableAccounts)).
			Msg("transaction lost")
		lg.workerStatsTracker.IncTxTimedout()
	case <-lg.Done():
		return
	}
	lg.workerStatsTracker.IncTxExecuted()
}

func (lg *ContLoadGenerator) sendEVMTx(workerID int) {
	log := lg.log.With().Int("workerID", workerID).Logger()

	log.Trace().
		Int("availableAccounts", len(lg.availableAccounts)).
		Msg("getting next available account")

	var acc *account.FlowAccount

	select {
	case acc = <-lg.availableAccounts:
	default:
		log.Error().Msg("next available account channel empty; skipping send")
		return
	}
	defer func() { lg.availableAccounts <- acc }()
	nextAcc := lg.accounts[(acc.ID+1)%len(lg.accounts)]

	log.Trace().
		Float64("tokens", tokensPerTransfer).
		Hex("srcAddress", acc.Address.Bytes()).
		Hex("dstAddress", nextAcc.Address.Bytes()).
		Int("srcAccount", acc.ID).
		Int("dstAccount", nextAcc.ID).
		Msg("creating transfer script")

	key, err := acc.GetKey()
	if err != nil {
		log.Error().Err(err).Msg("error getting key")
		return
	}
	defer key.Done()

	nonce := key.SequenceNumber
	to := gethcommon.HexToAddress("")
	gasPrice := big.NewInt(0)

	amount := new(big.Int).Div(evmTypes.OneFlowBalance, big.NewInt(10000000))
	evmTx := types.NewTx(&types.LegacyTx{Nonce: nonce, To: &to, Value: amount, Gas: params.TxGas, GasPrice: gasPrice, Data: nil})

	privateKey, err := gethcrypto.HexToECDSA("e43eb57b2f3be8009ea545059e171dc4fdf543ae97220a76c0684706357f7f39")
	if err != nil {
		log.Error().Err(err).Msg("error getting key")
		return
	}

	signed, err := types.SignTx(evmTx, emulator.GetDefaultSigner(), privateKey)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}
	var encoded bytes.Buffer
	err = signed.EncodeRLP(&encoded)
	if err != nil {
		log.Error().Err(err).Msg("error encoding transaction")
		return
	}

	encodedCadence := make([]cadence.Value, 0)
	for _, b := range encoded.Bytes() {
		encodedCadence = append(encodedCadence, cadence.UInt8(b))
	}
	transactionBytes := cadence.NewArray(encodedCadence).WithType(stdlib.EVMTransactionBytesCadenceType)

	sc := systemcontracts.SystemContractsForChain(lg.networkParams.ChainId)
	txBody := flowsdk.NewTransaction().
		SetScript([]byte(fmt.Sprintf(
			`
						import EVM from %s
						import FungibleToken from %s
						import FlowToken from %s
						
						transaction(encodedTx: [UInt8]) {
							//let bridgeVault: @FlowToken.Vault
						
							prepare(signer: AuthAccount){}
						
							execute {
								let feeAcc <- EVM.createBridgedAccount()
								EVM.run(tx: encodedTx, coinbase: feeAcc.address())
								destroy feeAcc
							}
						}
					`,
			sc.EVMContract.Address.HexWithPrefix(),
			sc.FungibleToken.Address.HexWithPrefix(),
			sc.FlowToken.Address.HexWithPrefix(),
		))).
		SetReferenceBlockID(lg.follower.BlockID()).
		SetComputeLimit(9999)

	err = txBody.AddArgument(transactionBytes)
	if err != nil {
		log.Error().Err(err).Msg("error adding argument")
		return
	}

	log.Trace().Msg("signing transaction")

	err = key.SignTx(txBody)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	startTime := time.Now()
	ch, err := lg.sendTx(workerID, txBody)
	if err != nil {
		return
	}
	defer key.IncrementSequenceNumber()

	log = log.With().Hex("tx_id", txBody.ID().Bytes()).Logger()
	log.Trace().Msg("transaction sent")

	t := time.NewTimer(lostTransactionThreshold)
	defer t.Stop()

	select {
	case result := <-ch:
		if result.Error != nil {
			lg.workerStatsTracker.IncTxFailed()
		}
		log.Trace().
			Dur("duration", time.Since(startTime)).
			Err(result.Error).
			Str("status", result.Status.String()).
			Msg("transaction confirmed")
	case <-t.C:
		lg.loaderMetrics.TransactionLost()
		log.Warn().
			Dur("duration", time.Since(startTime)).
			Int("availableAccounts", len(lg.availableAccounts)).
			Msg("transaction lost")
		lg.workerStatsTracker.IncTxTimedout()
	case <-lg.Done():
		return
	}
	lg.workerStatsTracker.IncTxExecuted()
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
	case ExecDataHeavyLoadType:
		txScript = ExecDataHeavyScript(*lg.favContractAddress)
	default:
		log.Error().Msg("unknown load type")
		return
	}

	log.Trace().Msg("creating transaction")
	tx := flowsdk.NewTransaction().
		SetReferenceBlockID(lg.follower.BlockID()).
		SetScript(txScript).
		SetComputeLimit(9999)

	log.Trace().Msg("signing transaction")

	key, err := acc.GetKey()
	if err != nil {
		log.Error().Err(err).Msg("error getting key")
		return
	}
	defer key.Done()

	err = key.SignTx(tx)
	if err != nil {
		log.Error().Err(err).Msg("error signing transaction")
		return
	}

	ch, err := lg.sendTx(workerID, tx)
	if err != nil {
		return
	}
	defer key.IncrementSequenceNumber()

	<-ch
	lg.workerStatsTracker.IncTxExecuted()
}

func (lg *ContLoadGenerator) sendTx(workerID int, tx *flowsdk.Transaction) (<-chan flowsdk.TransactionResult, error) {
	log := lg.log.With().Int("workerID", workerID).Str("tx_id", tx.ID().String()).Logger()
	log.Trace().Msg("sending transaction")

	// Add watcher before sending the transaction to avoid race condition
	ch := lg.follower.Follow(tx.ID())

	err := lg.flowClient.SendTransaction(lg.ctx, *tx)
	if err != nil {
		log.Error().Err(err).Msg("error sending transaction")
		return nil, err
	}

	lg.workerStatsTracker.IncTxSent()
	lg.loaderMetrics.TransactionSent()
	return ch, err
}
