package wintermute

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	verificationtest "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHandleEventFromCorruptedNode_CorruptEN tests that the orchestrator corrupts the execution receipt if the receipt is from a corrupt EN
// If an execution receipt is coming from a corrupt EN, then orchestrator tampers with the receipt and generates a counterfeit receipt, and then
// enforces that corrupt EN to send this counterfeit receipt.
// The orchestrator then also enforces the second corrupted EN to send the same counterfeit result on its behalf to the network.
func TestHandleEventFromCorruptedNode_CorruptEN(t *testing.T) {
	rootStateFixture, allIdentityList, corruptedIdentityList := bootstrapWintermuteFlowSystem(t)

	// generates a chain of blocks in the form of rootHeader <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
	// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
	// Container blocks only contain receipts of their preceding reference blocks. But they do not
	// hold any guarantees.
	rootHeader, err := rootStateFixture.State.Final().Head()
	require.NoError(t, err)

	completeExecutionReceipts := verificationtest.CompleteExecutionReceiptChainFixture(t, rootHeader, 1, verificationtest.WithExecutorIDs(corruptedIdentityList.NodeIDs()))

	corruptedEn1 := corruptedIdentityList.Filter(filter.HasRole(flow.RoleExecution))[0]
	targetIdentities, err := rootStateFixture.State.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	wintermuteOrchestrator := NewOrchestrator(allIdentityList, corruptedIdentityList, unittest.Logger())

	mockAttackNetwork.
		On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct

			// extract Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			// make sure sender is a corrupted execution node.
			corruptedIdentity, ok := corruptedIdentityList.ByNodeID(event.CorruptedId)
			require.True(t, ok)
			require.Equal(t, flow.RoleExecution, corruptedIdentity.Role)

			// make sure message being sent on correct channel
			require.Equal(t, engine.PushReceipts, event.Channel)

			corruptedReceipt, ok := event.FlowProtocolEvent.(*flow.ExecutionReceipt)
			require.True(t, ok)

			// make sure the original uncorrupted execution receipt is NOT sent to orchestrator
			require.NotEqual(t, completeExecutionReceipts[0].Receipts[0], corruptedReceipt)

			receivedTargetIds := event.TargetIds

			require.ElementsMatch(t, targetIdentities.NodeIDs(), receivedTargetIds)

			require.NotEqual(t, completeExecutionReceipts[0].ContainerBlock.Payload.Results[0], corruptedReceipt.ExecutionResult)
		}).Return(nil)

	event := &insecure.Event{
		CorruptedId:       corruptedEn1.NodeID,
		Channel:           engine.PushReceipts,
		Protocol:          insecure.Protocol_UNICAST,
		TargetNum:         uint32(0),
		TargetIds:         targetIdentities.NodeIDs(),
		FlowProtocolEvent: completeExecutionReceipts[0].Receipts[0],
	}

	// register mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)
	err = wintermuteOrchestrator.HandleEventFromCorruptedNode(event)
	require.NoError(t, err)
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
