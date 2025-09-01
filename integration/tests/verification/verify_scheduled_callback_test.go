package verification

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
)

func TestVerifyScheduledCallback(t *testing.T) {
	suite.Run(t, new(VerifyScheduledCallbackSuite))
}

type VerifyScheduledCallbackSuite struct {
	Suite
}

func (s *VerifyScheduledCallbackSuite) TestVerifyScheduledCallback() {
	// Wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.HeaderBody.Height, blockA.ID())

	// Deploy the test contract first
	s.deployTestContract()

	// Wait for next height finalized before scheduling callback
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), s.BlockState.HighestFinalizedHeight())

	// Schedule a callback for 10 seconds in the future
	scheduleDelta := int64(10)
	futureTimestamp := time.Now().Unix() + scheduleDelta
	s.T().Logf("scheduling callback at timestamp: %v, current timestamp: %v", futureTimestamp, time.Now().Unix())
	callbackID := s.scheduleCallback(futureTimestamp)
	s.T().Logf("scheduled callback with ID: %d", callbackID)

	// make sure callback executed event was emitted
	sc := systemcontracts.SystemContractsForChain(s.net.Root().HeaderBody.ChainID)
	eventTypeString := fmt.Sprintf("A.%v.FlowCallbackScheduler.Executed", sc.FlowCallbackScheduler.Address)

	// wait for block that executed the scheduled callbacks to be sealed (plus some buffer)
	var sealedBlock *flow.Block
	require.Eventually(s.T(), func() bool {
		sealed, ok := s.BlockState.HighestSealed()
		require.True(s.T(), ok)
		sealedBlock = sealed
		// sealed timestamp /1000 to drop the ms, and +2 to add some buffer
		return uint64(sealed.Timestamp/1000) > uint64(futureTimestamp+5)
	}, 30*time.Second, 1000*time.Millisecond)

	events, err := s.AccessClient().GetEventsForHeightRange(context.Background(), eventTypeString, blockA.HeaderBody.Height, sealedBlock.Height)
	require.NoError(s.T(), err)

	eventCount := 0
	for _, event := range events {
		for _, e := range event.Events {
			fmt.Println("### event type: ", e.Type, "event index: ", e.EventIndex, "transaction ID: ", e.TransactionID)
			eventCount++
		}
	}

	require.Equal(s.T(), eventCount, 1, "expected 1 callback executed event")
}

func (s *VerifyScheduledCallbackSuite) scheduleCallback(timestamp int64) uint64 {
	chainID := s.net.Root().HeaderBody.ChainID
	sc := systemcontracts.SystemContractsForChain(chainID)

	scheduledTx := fmt.Sprintf(`
		import FlowCallbackScheduler from 0x%s
		import TestFlowCallbackHandler from 0x%s
		import FlowToken from 0x%s
		import FungibleToken from 0x%s

		transaction(timestamp: UFix64) {

			prepare(account: auth(BorrowValue, SaveValue, IssueStorageCapabilityController, PublishCapability, GetStorageCapabilityController) &Account) {
        		if !account.storage.check<@TestFlowCallbackHandler.Handler>(from: TestFlowCallbackHandler.HandlerStoragePath) {
            		let handler <- TestFlowCallbackHandler.createHandler()
				
					account.storage.save(<-handler, to: TestFlowCallbackHandler.HandlerStoragePath)
            		account.capabilities.storage.issue<auth(FlowCallbackScheduler.Execute) &{FlowCallbackScheduler.CallbackHandler}>(TestFlowCallbackHandler.HandlerStoragePath)
				}

				let callbackCap = account.capabilities.storage
					.getControllers(forPath: TestFlowCallbackHandler.HandlerStoragePath)[0]
					.capability as! Capability<auth(FlowCallbackScheduler.Execute) &{FlowCallbackScheduler.CallbackHandler}>
				
				let vault = account.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("Could not borrow FlowToken vault")
				
				let testData = "test data"
				let feeAmount = 1.0
				let effort = UInt64(1000)
				let priority = FlowCallbackScheduler.Priority.High

				let fees <- vault.withdraw(amount: feeAmount) as! @FlowToken.Vault
				
				let scheduledCallback = FlowCallbackScheduler.schedule(
					callback: callbackCap,
					data: testData,
					timestamp: timestamp,
					priority: priority,
					executionEffort: effort,
					fees: <-fees
				)

				TestFlowCallbackHandler.addScheduledCallback(callback: scheduledCallback)
			}
		} 
	`, sc.FlowCallbackScheduler.Address, s.AccessClient().SDKServiceAddress(), sc.FlowToken.Address, sc.FungibleToken.Address)

	timeArg, err := cadence.NewUFix64(fmt.Sprintf("%d.0", timestamp))
	require.NoError(s.T(), err, "could not create time argument")

	return s.sendCallbackTx([]byte(scheduledTx), []cadence.Value{timeArg})
}

func (s *VerifyScheduledCallbackSuite) sendCallbackTx(script []byte, args []cadence.Value) uint64 {
	header, err := s.AccessClient().GetLatestFinalizedBlockHeader(context.Background())
	require.NoError(s.T(), err, "could not get latest block ID")

	acc, err := s.AccessClient().GetAccountAtBlockHeight(context.Background(), s.AccessClient().SDKServiceAddress(), header.Height)
	require.NoError(s.T(), err, "could not get account")

	tx := sdk.NewTransaction().
		SetScript(script).
		SetReferenceBlockID(sdk.Identifier(header.ID)).
		SetProposalKey(sdk.Address(acc.Address), acc.Keys[0].Index, acc.Keys[0].SequenceNumber).
		SetPayer(sdk.Address(acc.Address)).
		AddAuthorizer(sdk.Address(acc.Address))

	for _, arg := range args {
		err = tx.AddArgument(arg)
		require.NoError(s.T(), err, "could not add argument to transaction")
	}

	err = s.AccessClient().SignAndSendTransaction(context.Background(), tx)
	require.NoError(s.T(), err, "could not send schedule transaction")

	// Wait for the transaction to be executed
	executedResult, err := s.AccessClient().WaitForExecuted(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for schedule transaction to be executed")
	require.NoError(s.T(), executedResult.Error, "schedule transaction should not have error")

	// Extract callback ID from events
	callbackID := s.extractCallbackIDFromEvents(executedResult)
	require.NotEqual(s.T(), callbackID, uint64(0), "callback ID should not be 0")

	return callbackID
}

func (s *VerifyScheduledCallbackSuite) extractCallbackIDFromEvents(result *sdk.TransactionResult) uint64 {
	for _, event := range result.Events {
		if strings.Contains(string(event.Type), "FlowCallbackScheduler.Scheduled") ||
			strings.Contains(string(event.Type), "FlowCallbackScheduler.Canceled") ||
			strings.Contains(string(event.Type), "FlowCallbackScheduler.Executed") ||
			strings.Contains(string(event.Type), "FlowCallbackScheduler.PendingExecution") {

			if id := event.Value.SearchFieldByName("id"); id != nil {
				return uint64(id.(cadence.UInt64))
			}
		}
	}

	return 0
}

func (s *VerifyScheduledCallbackSuite) deployTestContract() {
	chainID := s.net.Root().HeaderBody.ChainID
	sc := systemcontracts.SystemContractsForChain(chainID)

	testContract := lib.TestFlowCallbackHandlerContract(
		sdk.Address(sc.FlowCallbackScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
	)
	tx, err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), testContract)

	require.NoError(s.T(), err, "could not deploy test contract")
	s.T().Logf("deployed test contract, tx ID: %v", tx.ID())

	res, err := s.AccessClient().WaitForExecuted(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for deploy transaction to be sealed")
	require.NoError(s.T(), res.Error, "deploy transaction should not have error")
}
