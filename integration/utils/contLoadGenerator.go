package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

const tokensPerTransfer = 0.01 // flow testnets only have 10e6 total supply, so we choose a small amount here

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

	numberOfAccounts := tps * 10 // 1 second per block, factor 10 for delays to prevent sequence number collisions

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
	err := lg.createAccounts()
	if err != nil {
		return err
	}

	return nil
}

func (lg *ContLoadGenerator) Start() {
	// spawn workers
	for i := 0; i < lg.tps; i++ {
		worker := NewWorker(i, 1*time.Second, lg.sendTx)
		worker.Start()
		lg.workerStatsTracker.AddWorker()

		lg.workers = append(lg.workers, &worker)
	}

	lg.workerStatsTracker.StartPrinting(1 * time.Second)
}

func (lg *ContLoadGenerator) Stop() {
	for _, w := range lg.workers {
		w.Stop()
	}
	lg.txTracker.Stop()
	lg.workerStatsTracker.StopPrinting()
}

const createAccountsTransactionTemplate = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(publicKey: [UInt8], count: Int, initialTokenAmount: UFix64) {
  prepare(signer: AuthAccount) {
	let vault = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
      ?? panic("Could not borrow reference to the owner's Vault")

    var i = 0
    while i < count {
      let account = AuthAccount(payer: signer)
      account.addPublicKey(publicKey)

	  let receiver = account.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>()
		?? panic("Could not borrow receiver reference to the recipient's Vault")

      receiver.deposit(from: <-vault.withdraw(amount: initialTokenAmount))

      i = i + 1
    }
  }
}
`

func createAccountsTransaction(fungibleToken, flowToken flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(createAccountsTransactionTemplate, fungibleToken, flowToken))
}

func (lg *ContLoadGenerator) createAccounts() error {
	lg.log.Info().Msgf("creating and funding %d accounts...", lg.numberOfAccounts)

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
		SetScript(createAccountsTransaction(*lg.fungibleTokenAddress, *lg.flowTokenAddress)).
		SetReferenceBlockID(blockRef).
		SetProposalKey(
			*lg.serviceAccount.address,
			lg.serviceAccount.accountKey.ID,
			lg.serviceAccount.accountKey.SequenceNumber,
		).
		AddAuthorizer(*lg.serviceAccount.address).
		SetPayer(*lg.serviceAccount.address)

	publicKey := bytesToCadenceArray(accountKey.Encode())
	count := cadence.NewInt(lg.numberOfAccounts)

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
		lg.serviceAccount.accountKey.ID,
		lg.serviceAccount.signer,
	)
	if err != nil {
		return err
	}

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

func (lg *ContLoadGenerator) sendTx(workerID int) {
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
		stopped := false
		wg := sync.WaitGroup{}
		lg.txTracker.AddTx(transferTx.ID(),
			nil,
			func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", transferTx.ID().String()).Msgf("finalized tx")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on finalized
			func(_ flowsdk.Identifier, _ *flowsdk.TransactionResult) {
				lg.log.Trace().Str("tx_id", transferTx.ID().String()).Msgf("sealed tx")
			}, // on sealed
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", transferTx.ID().String()).Msgf("tx expired")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on expired
			func(_ flowsdk.Identifier) {
				lg.log.Warn().Str("tx_id", transferTx.ID().String()).Msgf("tx timed out")
				if !stopped {
					stopped = true
					wg.Done()
				}
			}, // on timout
			func(_ flowsdk.Identifier, err error) {
				lg.log.Error().Err(err).Str("tx_id", transferTx.ID().String()).Msgf("tx error")
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
