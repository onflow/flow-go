package lib

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
)

const (
	CounterDefaultValue     = -3
	CounterInitializedValue = 2
)

var (
	// CounterContract is a simple counter contract in Cadence
	CounterContract = dsl.Contract{
		Name: "Testing",
		Members: []dsl.CadenceCode{
			dsl.Resource{
				Name: "Counter",
				Code: `
				access(all) var count: Int

				init() {
					self.count = 0
				}
				access(all) fun add(_ count: Int) {
					self.count = self.count + count
				}`,
			},
			dsl.Code(`
				access(all) fun createCounter(): @Counter {
					return <-create Counter()
				}`,
			),
		},
	}
)

// TestFlowScheduledTransactionHandlerContract creates a test contract DSL for testing FlowTransactionScheduler
func TestFlowScheduledTransactionHandlerContract(transactionScheduler sdk.Address, flowToken sdk.Address, fungibleToken sdk.Address) dsl.Contract {
	return dsl.Contract{
		Name: "TestFlowTransactionSchedulerHandler",
		Imports: []dsl.Import{
			{
				Names:   []string{"FlowTransactionScheduler"},
				Address: transactionScheduler,
			},
			{
				Names:   []string{"FlowToken"},
				Address: flowToken,
			},
			{
				Names:   []string{"FungibleToken"},
				Address: fungibleToken,
			},
		},
		Members: []dsl.CadenceCode{
			dsl.Code(`
				access(all) var scheduledTransactions: @{UInt64: FlowTransactionScheduler.ScheduledTransaction}
				access(all) var executedTransactions: [UInt64]

				access(all) let HandlerStoragePath: StoragePath
				access(all) let HandlerPublicPath: PublicPath
				
				access(all) resource Handler: FlowTransactionScheduler.TransactionHandler {
					
					access(FlowTransactionScheduler.Execute) 
					fun executeTransaction(id: UInt64, data: AnyStruct?) {
						TestFlowTransactionSchedulerHandler.executedTransactions.append(id)
					}
				}

				access(all) fun createHandler(): @Handler {
					return <- create Handler()
				}

				access(all) fun addScheduledTransaction(tx: @FlowTransactionScheduler.ScheduledTransaction) {
					self.scheduledTransactions[tx.id] <-! tx
				}

				access(all) fun cancelTransaction(id: UInt64): @FlowToken.Vault {
					let tx <- self.scheduledTransactions.remove(key: id)
						?? panic("Invalid ID: \(id) tx not found")
					return <-FlowTransactionScheduler.cancel(scheduledTx: <-tx)
				}

				access(all) fun getExecutedTransactions(): [UInt64] {
					return self.executedTransactions
				}

				access(all) init() {
					self.scheduledTransactions <- {}
					self.executedTransactions = []

					self.HandlerStoragePath = /storage/testTransactionHandler
					self.HandlerPublicPath = /public/testTransactionHandler
				}
			`),
		},
	}
}

// CreateCounterTx is a transaction script for creating an instance of the counter in the account storage of the
// authorizing account NOTE: the counter contract must be deployed first
func CreateCounterTx(counterAddress sdk.Address) dsl.Transaction {
	return dsl.Transaction{
		Imports: dsl.Imports{
			dsl.Import{
				Address: counterAddress,
			},
		},
		Content: dsl.Prepare{
			Content: dsl.Code(fmt.Sprintf(
				`
					var maybeCounter <- signer.storage.load<@Testing.Counter>(from: /storage/counter)

					if maybeCounter == nil {
						maybeCounter <-! Testing.createCounter()
					}

					maybeCounter?.add(%d)
					signer.storage.save(<-maybeCounter!, to: /storage/counter)

					let counterCap = signer.capabilities.storage.issue<&Testing.Counter>(/storage/counter)
					signer.capabilities.publish(counterCap, at: /public/counter)
				`,
				CounterInitializedValue,
			)),
		},
	}
}

// ReadCounterScript is a read-only script for reading the current value of the counter contract
func ReadCounterScript(contractAddress sdk.Address, accountAddress sdk.Address) dsl.Main {
	return dsl.Main{
		Import: dsl.Import{
			Names:   []string{"Testing"},
			Address: contractAddress,
		},
		ReturnType: "Int",
		Code: fmt.Sprintf(
			`
			  let account = getAccount(0x%s)
			  let counter = account.capabilities.borrow<&Testing.Counter>(/public/counter)
			  return counter?.count ?? %d
			`,
			accountAddress.Hex(),
			CounterDefaultValue,
		),
	}
}

// CreateCounterPanicTx is a transaction script that creates a counter instance in the root account, but panics after
// manipulating state. It can be used to test whether execution state stays untouched/will revert. NOTE: the counter
// contract must be deployed first
func CreateCounterPanicTx(chain flow.Chain) dsl.Transaction {
	return dsl.Transaction{
		Imports: dsl.Imports{
			dsl.Import{
				Address: sdk.Address(chain.ServiceAddress()),
			},
		},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				var maybeCounter <- signer.storage.load<@Testing.Counter>(from: /storage/counter)

				if maybeCounter == nil {
					maybeCounter <-! Testing.createCounter()
				}

				maybeCounter?.add(2)
				signer.storage.save(<-maybeCounter!, to: /storage/counter)

				let counterCap = signer.capabilities.storage.issue<&Testing.Counter>(/storage/counter)
				signer.capabilities.publish(counterCap, at: /public/counter)

				panic("fail for testing purposes")
				`),
		},
	}
}

// ReadCounter executes a script to read the value of a counter. The counter
// must have been deployed and created.
func ReadCounter(ctx context.Context, client *testnet.Client, address sdk.Address) (int, error) {

	res, err := client.ExecuteScript(ctx, ReadCounterScript(address, address))
	if err != nil {
		return 0, err
	}

	return res.(cadence.Int).Int(), nil
}

// GetAccount returns a new account address, key, and signer.
func GetAccount(chain flow.Chain) (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer, error) {

	addr := sdk.Address(chain.ServiceAddress())

	key := RandomPrivateKey()
	signer, err := sdkcrypto.NewInMemorySigner(key, sdkcrypto.SHA3_256)
	if err != nil {
		return sdk.Address{}, nil, nil, err
	}

	acct := sdk.NewAccountKey().
		FromPrivateKey(key).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	return addr, acct, signer, nil
}

// RandomPrivateKey returns a randomly generated ECDSA P-256 private key.
func RandomPrivateKey() sdkcrypto.PrivateKey {
	seed := make([]byte, sdkcrypto.MinSeedLength)

	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	privateKey, err := sdkcrypto.GeneratePrivateKey(sdkcrypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

func SDKTransactionFixture(opts ...func(*sdk.Transaction)) sdk.Transaction {
	tx := sdk.Transaction{
		Script:             []byte("access(all) fun main() {}"),
		ReferenceBlockID:   sdk.Identifier(unittest.IdentifierFixture()),
		GasLimit:           10,
		ProposalKey:        convert.ToSDKProposalKey(unittest.ProposalKeyFixture()),
		Payer:              sdk.Address(unittest.AddressFixture()),
		Authorizers:        []sdk.Address{sdk.Address(unittest.AddressFixture())},
		PayloadSignatures:  []sdk.TransactionSignature{},
		EnvelopeSignatures: []sdk.TransactionSignature{convert.ToSDKTransactionSignature(unittest.TransactionSignatureFixture())},
	}

	for _, apply := range opts {
		apply(&tx)
	}

	return tx
}

func WithTransactionDSL(txDSL dsl.Transaction) func(tx *sdk.Transaction) {
	return func(tx *sdk.Transaction) {
		tx.Script = []byte(txDSL.ToCadence())
	}
}

func WithReferenceBlock(id sdk.Identifier) func(tx *sdk.Transaction) {
	return func(tx *sdk.Transaction) {
		tx.ReferenceBlockID = id
	}
}

// WithChainID modifies the default fixture to use addresses consistent with the
// given chain ID.
func WithChainID(chainID flow.ChainID) func(tx *sdk.Transaction) {
	service := convert.ToSDKAddress(chainID.Chain().ServiceAddress())
	return func(tx *sdk.Transaction) {
		tx.Payer = service
		tx.Authorizers = []sdk.Address{service}

		tx.ProposalKey.Address = service
		for i, sig := range tx.PayloadSignatures {
			sig.Address = service
			tx.PayloadSignatures[i] = sig
		}
		for i, sig := range tx.EnvelopeSignatures {
			sig.Address = service
			tx.EnvelopeSignatures[i] = sig
		}
	}
}

// LogStatus logs current information about the test network state.
func LogStatus(t *testing.T, ctx context.Context, log zerolog.Logger, client *testnet.Client) {
	// retrieves latest FINALIZED snapshot
	snapshot, err := client.GetLatestProtocolSnapshot(ctx)
	if err != nil {
		log.Err(err).Msg("failed to get finalized snapshot")
		return
	}

	sealingSegment, err := snapshot.SealingSegment()
	require.NoError(t, err)
	sealed := sealingSegment.Sealed()
	finalized := sealingSegment.Finalized()

	phase, err := snapshot.EpochPhase()
	require.NoError(t, err)
	epoch, err := snapshot.Epochs().Current()
	require.NoError(t, err)
	counter := epoch.Counter()

	log.Info().Uint64("final_height", finalized.Height).
		Uint64("final_view", finalized.View).
		Uint64("sealed_height", sealed.Height).
		Uint64("sealed_view", sealed.View).
		Str("cur_epoch_phase", phase.String()).
		Uint64("cur_epoch_counter", counter).
		Msg("test run status")
}

// LogStatusPeriodically periodically logs information about the test network state.
// It can be run as a goroutine at the beginning of a test run to provide period
func LogStatusPeriodically(t *testing.T, parent context.Context, log zerolog.Logger, client *testnet.Client, period time.Duration) {
	log = log.With().Str("util", "status_logger").Logger()
	for {
		select {
		case <-parent.Done():
			return
		case <-time.After(period):
		}

		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		LogStatus(t, ctx, log, client)
		cancel()
	}
}

// ScheduleTransactionAtTimestamp sends a test transaction to schedule a transaction on FlowTransactionScheduler
// at a given timestamp and returns the scheduled transaction ID.
func ScheduleTransactionAtTimestamp(
	timestamp int64,
	client *testnet.Client,
	sc *systemcontracts.SystemContracts,
) (uint64, error) {
	referenceBlock, err := client.GetLatestFinalizedBlockHeader(context.Background())
	if err != nil {
		return 0, fmt.Errorf("could not get latest block ID: %w", err)
	}

	flowTransactionScheduler := sdk.Address(sc.FlowTransactionScheduler.Address)
	flowToken := sdk.Address(sc.FlowToken.Address)
	fungibleToken := sdk.Address(sc.FungibleToken.Address)

	serviceAccountAddress := client.SDKServiceAddress()
	script := []byte(fmt.Sprintf(`
		import FlowTransactionScheduler from 0x%s
		import TestFlowTransactionSchedulerHandler from 0x%s
		import FlowToken from 0x%s
		import FungibleToken from 0x%s

		transaction(timestamp: UFix64) {

			prepare(account: auth(BorrowValue, SaveValue, IssueStorageCapabilityController, PublishCapability, GetStorageCapabilityController) &Account) {
				if !account.storage.check<@TestFlowTransactionSchedulerHandler.Handler>(from: TestFlowTransactionSchedulerHandler.HandlerStoragePath) {
					let handler <- TestFlowTransactionSchedulerHandler.createHandler()

					account.storage.save(<-handler, to: TestFlowTransactionSchedulerHandler.HandlerStoragePath)
					account.capabilities.storage.issue<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>(TestFlowTransactionSchedulerHandler.HandlerStoragePath)
				}

				let transactionCap = account.capabilities.storage
					.getControllers(forPath: TestFlowTransactionSchedulerHandler.HandlerStoragePath)[0]
					.capability as! Capability<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>
				
				let vault = account.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("Could not borrow FlowToken vault")
				
				let testData = "test data"
				let feeAmount = 1.0
				let effort = UInt64(1000)
				let priority = FlowTransactionScheduler.Priority.High

				let fees <- vault.withdraw(amount: feeAmount) as! @FlowToken.Vault
				
				let scheduledTransaction <- FlowTransactionScheduler.schedule(
					handlerCap: transactionCap,
					data: testData,
					timestamp: timestamp,
					priority: priority,
					executionEffort: effort,
					fees: <-fees
				)

				TestFlowTransactionSchedulerHandler.addScheduledTransaction(transaction: <-scheduledTransaction)
			}
		} 
	`, serviceAccountAddress.Hex(), flowTransactionScheduler.Hex(), flowToken.Hex(), fungibleToken.Hex()))

	timeArg, err := cadence.NewUFix64(fmt.Sprintf("%d.0", timestamp))
	if err != nil {
		return 0, fmt.Errorf("could not create time argument: %w", err)
	}

	tx := sdk.NewTransaction().
		SetScript(script).
		SetReferenceBlockID(referenceBlock.ID).
		SetProposalKey(serviceAccountAddress, 0, client.GetAndIncrementSeqNumber()).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	err = tx.AddArgument(timeArg)
	if err != nil {
		return 0, fmt.Errorf("could not add argument to transaction: %w", err)
	}

	return sendScheduledTransactionTx(client, tx)
}

// CancelTransactionByID sends a test transaction for canceling a scheduled transaction on FlowTransactionScheduler by ID.
func CancelTransactionByID(
	transactionID uint64,
	client *testnet.Client,
	sc *systemcontracts.SystemContracts,
) (uint64, error) {
	referenceBlock, err := client.GetLatestFinalizedBlockHeader(context.Background())
	if err != nil {
		return 0, fmt.Errorf("could not get latest block ID: %w", err)
	}

	flowTransactionScheduler := sdk.Address(sc.FlowTransactionScheduler.Address)
	flowToken := sdk.Address(sc.FlowToken.Address)
	fungibleToken := sdk.Address(sc.FungibleToken.Address)

	serviceAccountAddress := client.SDKServiceAddress()
	cancelTx := fmt.Sprintf(`
		import FlowTransactionScheduler from 0x%s
		import TestFlowTransactionSchedulerHandler from 0x%s
		import FlowToken from 0x%s
		import FungibleToken from 0x%s

		transaction(id: UInt64) {

			prepare(account: auth(BorrowValue, SaveValue, IssueStorageCapabilityController, PublishCapability, GetStorageCapabilityController) &Account) {

				let vault = account.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("Could not borrow FlowToken vault")

				vault.deposit(from: <-TestFlowTransactionSchedulerHandler.cancelTransaction(id: id))
			}
		} 
	`, serviceAccountAddress.Hex(), flowTransactionScheduler.Hex(), flowToken.Hex(), fungibleToken.Hex())

	tx := sdk.NewTransaction().
		SetScript([]byte(cancelTx)).
		SetReferenceBlockID(referenceBlock.ID).
		SetProposalKey(serviceAccountAddress, 0, client.GetAndIncrementSeqNumber()).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	err = tx.AddArgument(cadence.UInt64(transactionID))
	if err != nil {
		return 0, fmt.Errorf("could not add argument to transaction: %w", err)
	}

	return sendScheduledTransactionTx(client, tx)
}

// ExtractTransactionIDFromEvents extracts the scheduled transaction ID from the events of a transaction result.
func ExtractTransactionIDFromEvents(result *sdk.TransactionResult) uint64 {
	for _, event := range result.Events {
		if strings.Contains(string(event.Type), "FlowTransactionScheduler.Scheduled") ||
			strings.Contains(string(event.Type), "FlowTransactionScheduler.Canceled") ||
			strings.Contains(string(event.Type), "FlowTransactionScheduler.Executed") ||
			strings.Contains(string(event.Type), "FlowTransactionScheduler.PendingExecution") {

			if id := event.Value.SearchFieldByName("id"); id != nil {
				return uint64(id.(cadence.UInt64))
			}
		}
	}

	return 0
}

// DeployScheduledTransactionsTestContract deploys the test contract for scheduled transactions.
func DeployScheduledTransactionsTestContract(
	client *testnet.Client,
	sc *systemcontracts.SystemContracts,
) (sdk.Identifier, error) {
	referenceBlock, err := client.GetLatestFinalizedBlockHeader(context.Background())
	if err != nil {
		return sdk.Identifier{}, fmt.Errorf("could not get latest block ID: %w", err)
	}

	flowTransactionScheduler := sdk.Address(sc.FlowTransactionScheduler.Address)
	flowToken := sdk.Address(sc.FlowToken.Address)
	fungibleToken := sdk.Address(sc.FungibleToken.Address)

	testContract := TestFlowScheduledTransactionHandlerContract(flowTransactionScheduler, flowToken, fungibleToken)
	tx, err := client.DeployContract(context.Background(), referenceBlock.ID, testContract)
	if err != nil {
		return sdk.Identifier{}, fmt.Errorf("could not deploy test contract: %w", err)
	}

	res, err := client.WaitForExecuted(context.Background(), tx.ID())
	if err != nil {
		return sdk.Identifier{}, fmt.Errorf("could not wait for deploy transaction to be sealed: %w", err)
	}

	if res.Error != nil {
		return sdk.Identifier{}, fmt.Errorf("deploy transaction should not have error: %w", res.Error)
	}

	return tx.ID(), nil
}

func sendScheduledTransactionTx(client *testnet.Client, tx *sdk.Transaction) (uint64, error) {
	err := client.SignAndSendTransaction(context.Background(), tx)
	if err != nil {
		return 0, fmt.Errorf("could not send schedule transaction: %w", err)
	}

	// Wait for the transaction to be executed
	executedResult, err := client.WaitForExecuted(context.Background(), tx.ID())
	if err != nil {
		return 0, fmt.Errorf("could not wait for schedule transaction to be executed: %w", err)
	}

	if executedResult.Error != nil {
		return 0, fmt.Errorf("schedule transaction should not have error: %w", executedResult.Error)
	}

	transactionID := ExtractTransactionIDFromEvents(executedResult)
	if transactionID == 0 {
		return 0, fmt.Errorf("scheduled transaction ID should not be 0")
	}

	return transactionID, nil
}
