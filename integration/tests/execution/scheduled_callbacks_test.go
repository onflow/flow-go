package execution

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
)

func TestScheduledCallbacks(t *testing.T) {
	suite.Run(t, new(ScheduledCallbacksSuite))
}

type ScheduledCallbacksSuite struct {
	Suite
}

func (s *ScheduledCallbacksSuite) TestScheduleCallback_DeployAndGetStatus() {
	// wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.HeaderBody.Height, blockA.ID())

	// Execute script to call getStatus(id: 10) on the contract
	result, ok := s.getCallbackStatus(10)
	s.T().Logf("result: %v, ok: %v", result, ok)
	require.False(s.T(), ok, "getStatus(10) should return false for non-existent callback")

	// Wait for a block to be executed to ensure everything is processed
	blockB := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.HeaderBody.Height)
	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockB.ID()), s.exe1ID)
	s.T().Logf("got block result ID %v", erBlock.ExecutionResult.BlockID)
}

func (s *ScheduledCallbacksSuite) TestScheduleCallback_ScheduledAndExecuted() {

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

	const scheduledStatus = 1
	const executedStatus = 2

	// Check the status of the callback right after scheduling
	status, ok := s.getCallbackStatus(callbackID)
	require.True(s.T(), ok, "callback status should not be nil after scheduling")
	require.Equal(s.T(), scheduledStatus, status, "status should be equal to scheduled")
	s.T().Logf("callback status after scheduling: %v", status)

	// Verify the callback is scheduled (not executed yet)
	executedCallbacks := s.getExecutedCallbacks()
	require.NotContains(s.T(), executedCallbacks, callbackID, "callback should not be executed immediately")

	// Wait to ensure the callback has time to be executed
	s.T().Log("waiting for callback execution...")
	time.Sleep(time.Duration(scheduleDelta)*time.Second + 2)

	// Wait for blocks to be processed after the callback execution time
	// blockC := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height+2)
	// erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockC.Header.ID()), s.exe1ID)
	// s.T().Logf("got block result ID %v after waiting", erBlock.ExecutionResult.BlockID)

	// Check the status again - it should still exist but be marked as executed
	statusAfter, ok := s.getCallbackStatus(callbackID)
	require.True(s.T(), ok, "callback status should not be nil after scheduling")
	require.Equal(s.T(), executedStatus, statusAfter, "status should be equal to executed")

	// Verify the callback was executed by checking our test contract
	executedCallbacksAfter := s.getExecutedCallbacks()
	s.T().Logf("executed callbacks: %v", executedCallbacksAfter)
	require.Len(s.T(), executedCallbacksAfter, 1, "should have exactly one executed callback")
	require.Contains(s.T(), executedCallbacksAfter, callbackID, "callback should have been executed")
}

func (s *ScheduledCallbacksSuite) TestScheduleCallback_ScheduleAndCancelCallback() {
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

	const scheduledStatus = 0
	const canceledStatus = 3

	// Wait fraction of the scheduled time
	s.T().Log("waiting for callback execution...")
	time.Sleep(time.Second + 2)

	// Check the status of the callback
	status, ok := s.getCallbackStatus(callbackID)
	require.True(s.T(), ok, "callback status should not be nil after scheduling")
	require.Equal(s.T(), scheduledStatus, status, "status should be equal to scheduled")
	s.T().Logf("callback status after scheduling: %v", status)

	// Verify the callback is scheduled (not executed yet)
	executedCallbacks := s.getExecutedCallbacks()
	require.NotContains(s.T(), executedCallbacks, callbackID, "callback should not be executed immediately")

	// Cancel the callback
	canceledID := s.cancelCallback(callbackID)
	require.Equal(s.T(), callbackID, canceledID, "canceled callback ID should be the same as scheduled")

	// Wait for callback scheduled time to make sure it was not executed
	time.Sleep(time.Duration(scheduleDelta) * time.Second)

	// Check the status of the callback
	status, ok = s.getCallbackStatus(callbackID)
	require.True(s.T(), ok, "callback status should not be nil after scheduling")
	require.Equal(s.T(), canceledStatus, status, "status should be equal to canceled")
}

func (s *ScheduledCallbacksSuite) deployTestContract() {
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

func (s *ScheduledCallbacksSuite) scheduleCallback(timestamp int64) uint64 {
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
				let effort = UInt64(10000)
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

func (s *ScheduledCallbacksSuite) cancelCallback(callbackID uint64) uint64 {
	chainID := s.net.Root().HeaderBody.ChainID
	sc := systemcontracts.SystemContractsForChain(chainID)

	cancelTx := fmt.Sprintf(`
		import FlowCallbackScheduler from 0x%s
		import TestFlowCallbackHandler from 0x%s
		import FlowToken from 0x%s
		import FungibleToken from 0x%s

		transaction(id: UInt64) {

			prepare(account: auth(BorrowValue, SaveValue, IssueStorageCapabilityController, PublishCapability, GetStorageCapabilityController) &Account) {

				let vault = account.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("Could not borrow FlowToken vault")

				vault.deposit(from: <-TestFlowCallbackHandler.cancelCallback(id: id))
			}
		} 
	`, sc.FlowCallbackScheduler.Address, s.AccessClient().SDKServiceAddress(), sc.FlowToken.Address, sc.FungibleToken.Address)

	return s.sendCallbackTx([]byte(cancelTx), []cadence.Value{cadence.UInt64(callbackID)})
}

func (s *ScheduledCallbacksSuite) getCallbackStatus(callbackID uint64) (int, bool) {
	getStatusScript := dsl.Main{
		Import: dsl.Import{
			Address: s.AccessClient().SDKServiceAddress(),
			Names:   []string{"FlowCallbackScheduler"},
		},
		ReturnType: "FlowCallbackScheduler.Status?",
		Code:       fmt.Sprintf("return FlowCallbackScheduler.getStatus(id: %d)", callbackID),
	}

	latest, err := s.AccessClient().GetLatestFinalizedBlockHeader(context.Background())
	require.NoError(s.T(), err, "could not get latest finalized block header")

	result, err := s.AccessClient().ExecuteScriptAtBlock(context.Background(), getStatusScript, latest.ID)
	require.NoError(s.T(), err, "could not execute getStatus script")

	optionalResult, ok := result.(cadence.Optional)
	require.True(s.T(), ok, "result should be a cadence.Optional")

	if optionalResult.Value == nil {
		return 0, false
	}

	enumValue, ok := optionalResult.Value.(cadence.Enum)
	require.True(s.T(), ok, "status should be a cadence.Enum")

	raw := enumValue.FieldsMappedByName()["rawValue"]
	val, ok := raw.(cadence.UInt8)
	require.True(s.T(), ok, "status should be a cadence.UInt8")

	return int(val), true
}

func (s *ScheduledCallbacksSuite) getExecutedCallbacks() []uint64 {
	getExecutedScript := dsl.Main{
		Import: dsl.Import{
			Address: s.AccessClient().SDKServiceAddress(),
			Names:   []string{"TestFlowCallbackHandler"},
		},
		ReturnType: "[UInt64]",
		Code:       "return TestFlowCallbackHandler.getExecutedCallbacks()",
	}

	latest, err := s.AccessClient().GetLatestFinalizedBlockHeader(context.Background())
	require.NoError(s.T(), err, "could not get latest finalized block header")

	result, err := s.AccessClient().ExecuteScriptAtBlock(context.Background(), getExecutedScript, latest.ID)
	require.NoError(s.T(), err, "could not execute getStatus script")

	// Convert cadence array to Go slice
	cadenceArray, ok := result.(cadence.Array)
	require.True(s.T(), ok, "result should be a cadence array")

	var executedIDs []uint64
	for _, value := range cadenceArray.Values {
		if id, ok := value.(cadence.UInt64); ok {
			executedIDs = append(executedIDs, uint64(id))
		}
	}

	return executedIDs
}

func (s *ScheduledCallbacksSuite) sendCallbackTx(script []byte, args []cadence.Value) uint64 {
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

func (s *ScheduledCallbacksSuite) extractCallbackIDFromEvents(result *sdk.TransactionResult) uint64 {
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
