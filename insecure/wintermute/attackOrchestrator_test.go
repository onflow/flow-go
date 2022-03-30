package wintermute_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/insecure/wintermute"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestOrchestrator_HandleEventFromCorruptedNode_SingleExecutionReceipt tests that the orchestrator corrupts the execution receipt if the receipt is
// from a corrupted execution node.
// If an execution receipt is coming from a corrupt execution node,
// then orchestrator tampers with the receipt and generates a counterfeit receipt, and then
// enforces all corrupted execution nodes to send that counterfeit receipt on their behalf in the flow network.
func TestOrchestrator_HandleEventFromCorruptedNode_SingleExecutionReceipt(t *testing.T) {
	rootStateFixture, allIdentityList, corruptedIdentityList := bootstrapWintermuteFlowSystem(t)

	receipt := unittest.ExecutionReceiptFixture()

	corruptedExecutionNodes := corruptedIdentityList.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs()

	// identities of nodes who are expected targets of an execution receipt.
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	corruptedReceiptsSentWG := mockAttackNetworkForCorruptedExecutionResult(t,
		mockAttackNetwork,
		receipt,
		receiptTargetIds.NodeIDs(),
		corruptedExecutionNodes)

	wintermuteOrchestrator := wintermute.NewOrchestrator(allIdentityList, corruptedIdentityList, unittest.Logger())
	event := &insecure.Event{
		CorruptedId:       corruptedExecutionNodes[0],
		Channel:           engine.PushReceipts,
		Protocol:          insecure.Protocol_UNICAST,
		TargetIds:         receiptTargetIds.NodeIDs(),
		FlowProtocolEvent: receipt,
	}

	// register mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)
	err = wintermuteOrchestrator.HandleEventFromCorruptedNode(event)
	require.NoError(t, err)

	// waits till corrupted receipts dictated to all execution nodes.
	unittest.RequireReturnsBefore(t,
		corruptedReceiptsSentWG.Wait,
		1*time.Second,
		"orchestrator could not send corrupted receipts on time")
}

func TestOrchestrator_HandleEventFromCorruptedNode_MultipleExecutionReceipt(t *testing.T) {
	rootStateFixture, allIdentityList, corruptedIdentityList := bootstrapWintermuteFlowSystem(t)
	corruptedExecutionIds := flow.IdentifierList(corruptedIdentityList.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs())
	// identities of nodes who are expected targets of an execution receipt.
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	corruptedEnId1 := corruptedExecutionIds[0]
	receipt1 := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(corruptedEnId1))
	event1 := &insecure.Event{
		CorruptedId:       corruptedEnId1,
		Channel:           engine.PushReceipts,
		Protocol:          insecure.Protocol_UNICAST,
		TargetIds:         receiptTargetIds.NodeIDs(),
		FlowProtocolEvent: receipt1,
	}

	corruptedEnId2 := corruptedExecutionIds[1]
	receipt2 := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(corruptedEnId2))
	event2 := &insecure.Event{
		CorruptedId:       corruptedEnId2,
		Channel:           engine.PushReceipts,
		Protocol:          insecure.Protocol_UNICAST,
		TargetIds:         receiptTargetIds.NodeIDs(),
		FlowProtocolEvent: receipt2,
	}

	mockAttackNetwork := &mockinsecure.AttackNetwork{}

	wintermuteOrchestrator := wintermute.NewOrchestrator(allIdentityList, corruptedIdentityList, unittest.Logger())

	receivedEventList := make([]*insecure.Event, 0)
	corruptedReceiptsRcvWG := &sync.WaitGroup{}
	corruptedReceiptsRcvWG.Add(3)
	mockAttackNetwork.
		On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct

			// extract Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			// make sure sender is a corrupted execution node.
			ok = corruptedExecutionIds.Contains(event.CorruptedId)
			require.True(t, ok)

			// makes sure sender is unique
			receivedEventList = append(receivedEventList, event)

			corruptedReceiptsRcvWG.Done()
		}).Return(nil)

	// register mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)

	corruptedReceiptsSentWG := sync.WaitGroup{}
	corruptedReceiptsSentWG.Add(2)
	go func() {
		err = wintermuteOrchestrator.HandleEventFromCorruptedNode(event1)
		require.Equal(t, event1.CorruptedId, receipt1.ExecutorID)
		require.NoError(t, err)

		corruptedReceiptsSentWG.Done()
	}()

	go func() {
		err = wintermuteOrchestrator.HandleEventFromCorruptedNode(event2)
		require.Equal(t, event2.CorruptedId, receipt2.ExecutorID)
		require.NoError(t, err)

		corruptedReceiptsSentWG.Done()
	}()

	// waits till corrupted receipts dictated to all execution nodes.
	unittest.RequireReturnsBefore(t,
		corruptedReceiptsSentWG.Wait,
		1*time.Second,
		"orchestrator could not send corrupted receipts on time")

	// waits till corrupted receipts dictated to all execution nodes.
	unittest.RequireReturnsBefore(t,
		corruptedReceiptsRcvWG.Wait,
		1*time.Second,
		"orchestrator could not receive corrupted receipts on time")

	// checks one receipt gets corrupted and sent to both corrupted execution nodes
	receivedExecutionReceiptEventsSanityCheck(
		t,
		receivedEventList,
		corruptedExecutionIds,
		flow.IdentifierList{receipt1.ID(), receipt2.ID()})
}

// TestHandleEventFromCorruptedNode_HonestVN tests that honest VN will be ignored when they send a chunk data request
func TestHandleEventFromCorruptedNode_HonestVN(t *testing.T) {

}

// TestHandleEventFromCorruptedNode_CorruptVN tests that orchestrator sends the result approval for the corrupted
// execution result if the chunk data request is coming from a corrupt VN
func TestHandleEventFromCorruptedNode_CorruptVN(t *testing.T) {

}

// helper functions

func bootstrapWintermuteFlowSystem(t *testing.T) (*enginemock.StateFixture, flow.IdentityList, flow.IdentityList) {
	// creates identities to bootstrap system with
	corruptedVnIds := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleVerification))
	corruptedEnIds := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	identities := unittest.CompleteIdentitySet(append(corruptedVnIds, corruptedEnIds...)...)
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)))    // one honest execution node
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))) // one honest verification node

	// bootstraps the system
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	stateFixture := testutil.CompleteStateFixture(t, metrics.NewNoopCollector(), trace.NewNoopTracer(), rootSnapshot)

	return stateFixture, identities, append(corruptedEnIds, corruptedVnIds...)
}

func mockAttackNetworkForCorruptedExecutionResult(
	t *testing.T,
	attackNetwork *mockinsecure.AttackNetwork,
	receipt *flow.ExecutionReceipt,
	receiptTargetIds flow.IdentifierList,
	corruptedExecutionIds flow.IdentifierList) *sync.WaitGroup {

	wg := &sync.WaitGroup{}

	// expecting to receive a corrupted receipt from each of corrupted execution nodes.
	wg.Add(corruptedExecutionIds.Len())
	seen := make(map[flow.Identifier]struct{})

	attackNetwork.
		On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct

			// extract Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			// make sure sender is a corrupted execution node.
			ok = corruptedExecutionIds.Contains(event.CorruptedId)
			require.True(t, ok)

			// makes sure sender is unique
			_, ok = seen[event.CorruptedId]
			require.False(t, ok)
			seen[event.CorruptedId] = struct{}{}

			// make sure message being sent on correct channel
			require.Equal(t, engine.PushReceipts, event.Channel)

			corruptedResult, ok := event.FlowProtocolEvent.(*flow.ExecutionResult)
			require.True(t, ok)

			// make sure the original uncorrupted execution receipt is NOT sent to orchestrator
			require.NotEqual(t, receipt.ExecutionResult, corruptedResult)
			require.ElementsMatch(t, receiptTargetIds, event.TargetIds)

			wg.Done()
		}).Return(nil)

	return wg
}

func receivedExecutionReceiptEventsSanityCheck(
	t *testing.T,
	events []*insecure.Event,
	corruptedExecutionIds flow.IdentifierList,
	originalReceiptIds flow.IdentifierList) {

	corruptedResults := make(map[flow.Identifier]flow.IdentifierList)
	seenReceipts := flow.IdentifierList{}
	for _, submittedEvent := range events {
		switch event := submittedEvent.FlowProtocolEvent.(type) {
		case *flow.ExecutionReceipt:
			seenReceipts.Union(flow.IdentifierList{event.ID()})
		case *flow.ExecutionResult:
			resultId := event.ID()
			if corruptedResults[resultId] == nil {
				corruptedResults[resultId] = flow.IdentifierList{}
			}
			corruptedResults[event.ID()].Union(flow.IdentifierList{submittedEvent.CorruptedId})
		}
	}

	// there must be only one corrupted result during a wintermute attack, and
	// that corrupted result must be dictated to all corrupted execution ids.
	require.Len(t, corruptedResults, 1)
	for _, corruptedIds := range corruptedResults {
		require.ElementsMatch(t, corruptedExecutionIds, corruptedIds)
	}
}
