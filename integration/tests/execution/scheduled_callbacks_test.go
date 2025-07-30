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
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// Execute script to call getStatus(id: 0) on the contract
	chainID := s.net.Root().Header.ChainID
	chain := chainID.Chain()

	result, ok := s.getCallbackStatus(chain.ServiceAddress(), 0)
	s.T().Logf("result: %v", result)
	require.False(s.T(), ok)
	require.Nil(s.T(), result, "getStatus(0) should return nil for non-existent callback")

	// Wait for a block to be executed to ensure everything is processed
	blockB := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height)
	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockB.Header.ID()), s.exe1ID)
	s.T().Logf("got block result ID %v", erBlock.ExecutionResult.BlockID)
}

func (s *ScheduledCallbacksSuite) TestScheduleCallback_ScheduledAndExecuted() {
	// Get chain information
	chainID := s.net.Root().Header.ChainID
	sc := systemcontracts.SystemContractsForChain(chainID)
	serviceAddress := sc.FlowServiceAccount.Address

	// Wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// Deploy the test contract first
	s.deployTestContract(serviceAddress)

	// Wait for next height finalized before scheduling callback
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), s.BlockState.HighestFinalizedHeight())

	// Schedule a callback for 90 seconds in the future
	scheduleDelta := int64(90)
	futureTimestamp := time.Now().Unix() + scheduleDelta
	s.T().Logf("scheduling callback at timestamp: %v, current timestamp: %v", futureTimestamp, time.Now().Unix())
	callbackID := s.scheduleCallback(sc, futureTimestamp)
	s.T().Logf("scheduled callback with ID: %d", callbackID)

	const scheduledStatus = 0
	const executedStatus = 2

	// Check the status of the callback right after scheduling
	status, ok := s.getCallbackStatus(serviceAddress, callbackID)
	require.True(s.T(), ok, "callback status should not be nil after scheduling")
	require.Equal(s.T(), scheduledStatus, status, "status should be equal to scheduled")
	s.T().Logf("callback status after scheduling: %v", status)

	// Verify the callback is scheduled (not executed yet)
	executedCallbacks := s.getExecutedCallbacks(serviceAddress)
	require.NotContains(s.T(), executedCallbacks, callbackID, "callback should not be executed immediately")

	// Wait to ensure the callback has time to be executed
	s.T().Log("waiting for callback execution...")
	time.Sleep(time.Duration(scheduleDelta)*time.Second + 10)

	// Wait for blocks to be processed after the callback execution time
	blockC := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height+2)
	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockC.Header.ID()), s.exe1ID)
	s.T().Logf("got block result ID %v after waiting", erBlock.ExecutionResult.BlockID)

	// Check the status again - it should still exist but be marked as executed
	statusAfter, ok := s.getCallbackStatus(serviceAddress, callbackID)
	require.True(s.T(), ok, "callback status should not be nil after scheduling")
	require.Equal(s.T(), executedStatus, statusAfter, "status should be equal to executed")

	// Verify the callback was executed by checking our test contract
	executedCallbacksAfter := s.getExecutedCallbacks(serviceAddress)
	s.T().Logf("executed callbacks: %v", executedCallbacksAfter)
	require.Contains(s.T(), executedCallbacksAfter, callbackID, "callback should have been executed")
}

func (s *ScheduledCallbacksSuite) deployTestContract(serviceAddress flow.Address) {
	testContract := lib.TestFlowCallbackHandlerContract(sdk.Address(serviceAddress))
	tx, err := s.AccessClient().DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), testContract)

	require.NoError(s.T(), err, "could not deploy test contract")
	s.T().Logf("deployed test contract, tx ID: %v", tx.ID())

	_, err = s.AccessClient().WaitForExecuted(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for deploy transaction to be sealed")
}

func (s *ScheduledCallbacksSuite) scheduleCallback(sc *systemcontracts.SystemContracts, timestamp int64) uint64 {
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
					account.capabilities.storage.issue<auth(FlowCallbackScheduler.ExecuteCallback) &{FlowCallbackScheduler.CallbackHandler}>(TestFlowCallbackHandler.HandlerStoragePath)
				}

				let callbackCap = account.capabilities.storage
									.getControllers(forPath: TestFlowCallbackHandler.HandlerStoragePath)[0]
									.capability as! Capability<auth(FlowCallbackScheduler.ExecuteCallback) &{FlowCallbackScheduler.CallbackHandler}>
				
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
	`, sc.FlowServiceAccount.Address, sc.FlowCallbackScheduler.Address, sc.FlowToken.Address, sc.FungibleToken.Address)

	acc, err := s.AccessClient().GetAccount(sdk.Address(sc.FlowServiceAccount.Address))
	require.NoError(s.T(), err, "could not get account")

	refID, err := s.AccessClient().GetLatestBlockID(context.Background())
	require.NoError(s.T(), err, "could not get latest block ID")

	tx := sdk.NewTransaction().
		SetScript([]byte(scheduledTx)).
		SetReferenceBlockID(sdk.Identifier(refID)).
		SetProposalKey(sdk.Address(acc.Address), 0, uint64(acc.Keys[0].Index+1)). // todo fix
		SetPayer(sdk.Address(acc.Address)).
		AddAuthorizer(sdk.Address(acc.Address))

	timeArg, err := cadence.NewUFix64(fmt.Sprintf("%d.0", timestamp))
	require.NoError(s.T(), err, "could not create time argument")

	err = tx.AddArgument(timeArg)
	require.NoError(s.T(), err, "could not add argument to transaction")

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

func (s *ScheduledCallbacksSuite) getCallbackStatus(serviceAddress flow.Address, callbackID uint64) (int, bool) {
	getStatusScript := dsl.Main{
		Import: dsl.Import{
			Address: sdk.Address(serviceAddress),
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

func (s *ScheduledCallbacksSuite) getExecutedCallbacks(serviceAddress flow.Address) []uint64 {
	getExecutedScript := dsl.Main{
		Import: dsl.Import{
			Address: sdk.Address(serviceAddress),
			Names:   []string{"TestFlowCallbackHandler"},
		},
		ReturnType: "[UInt64]",
		Code:       "return TestFlowCallbackHandler.getExecutedCallbacks()",
	}

	result, err := s.AccessClient().ExecuteScript(context.Background(), getExecutedScript)
	require.NoError(s.T(), err, "could not execute getExecutedCallbacks script")

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

func (s *ScheduledCallbacksSuite) extractCallbackIDFromEvents(result *sdk.TransactionResult) uint64 {
	for _, event := range result.Events {
		if strings.Contains(string(event.Type), "FlowCallbackScheduler.CallbackScheduled") {
			id := event.Value.SearchFieldByName("id")
			if id != nil {
				return uint64(id.(cadence.UInt64))
			}
		}
	}
	return 0
}
