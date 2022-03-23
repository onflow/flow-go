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

func TestWintermuteOrchestrator(t *testing.T) {
	// create corrupted Wintermute VN, EN nodes
	//colludingVNIdentityList := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleVerification))
	//maliciousENIdentityList := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	//corruptedIdentityList := append(colludingVNIdentityList, maliciousENIdentityList...)

	// create all honest nodes
	//honestIdentityList := unittest.IdentityListFixture(5, unittest.WithAllRoles())

	// create list of all nodes - honest and malicious
	//allIdentityList := append(honestIdentityList, corruptedIdentityList...)

	// should be mocked network
	attackNetwork := &mockinsecure.AttackNetwork{}
	//attackNetwork2 *attacknetwork.AttackNetwork,

	//orchestrator := NewOrchestrator(allIdentityList, corruptedIdentityList, unittest.Logger())

	//attackNetwork.

	attackNetwork.On("Start", mock.AnythingOfType("*irrecoverable.signalerCtx")).Return().Once()
	//signalerCtx, _ := irrecoverable.WithSignaler(context.Background())
	//orchestrator.start(signalerCtx)

	//genesisFixture := unittest.GenesisFixture()

	//honestExecutionResult1 := unittest.ExecutionResultFixture(unittest.WithBlock(genesisFixture))
	//honestExecutionResult2 := unittest.ExecutionResultFixture(unittest.WithBlock(genesisFixture))
	//
	//require.Equal(t, honestExecutionResult1.ID(), honestExecutionResult2.ID())

	blockIDFixture := unittest.IdentifierFixture()
	honestExecutionResult1 := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(blockIDFixture))
	honestExecutionResult2 := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(blockIDFixture))
	require.Equal(t, honestExecutionResult1.ID(), honestExecutionResult2.ID())

	//honestExecutionResult1.ID() // use this to check if execution results are the same
	//honestExecutionReceipt1 := unittest.ExecutionReceiptFixture(unittest.WithResult(honestExecutionResult1))
	//honestExecutionReceipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(honestExecutionResult1))

	// corrupt execution result
	honestExecutionResult1.Chunks[0].CollectionIndex = 999

	//require.Equal(t, honestExecutionReceipt1, honestExecutionReceipt2)

	//if honestExecutionResult1 1 and honestExecutionResult1 2 are agreeging
	//orchestrator.HandleEventFromCorruptedNode(executionResultFixtureFromNode1)
	//orchestrator.HandleEventFromCorruptedNode(executionResultFixtureFromNode2)

	// orchestrator replaces honestExecutionResult1 1 and honestExecutionResult1 2 with corrupted agreeing ones and send them over network
	// to execution node 1 and execution 2, respectively.
	//attackNetwork.On("")
}

// if an execution receipt is coming from one corrupted EN, then orchestrator tampers with the receipt and generates a counterfeit receipt, and then
// enforces that corrupted EN to send this counterfeit receipt.
//
// if an execution receipt is coming from one corrupted EN, then orchestrator tampers with the receipt and generates a counterfeit receipt, and then
// enforces that corrupted EN to send this counterfeit receipt.
// The orchestrator then also enforces the second corrupted EN to send the same counterfeit result on its behalf to the network.
func TestWintermuteOrchestrator_CorruptSingleExecutionResult(t *testing.T) {
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

			//corruptedId, ok := args[0].(flow.Identifier)
			corruptedId := event.CorruptedId
			//require.True(t, ok)
			// make sure sender is a corrupted execution node.
			corruptedIdentity, ok := corruptedIdentityList.ByNodeID(corruptedId)
			require.True(t, ok)
			require.Equal(t, flow.RoleExecution, corruptedIdentity.Role)

			//channel, ok := args[1].(network.Channel)
			channel := event.Channel
			//require.True(t, ok)
			// make sure message being sent on correct channel
			require.Equal(t, engine.PushReceipts, channel)

			//event, ok := args[2].(*flow.ExecutionReceipt)
			corruptedReceipt, ok := event.FlowProtocolEvent.(*flow.ExecutionReceipt)
			require.True(t, ok)

			// make sure the original uncorrupted execution receipt is NOT sent to orchestrator
			require.NotEqual(t, completeExecutionReceipts[0].Receipts[0], corruptedReceipt)

			//receivedTargetIds, ok := args[3].(flow.Identifier)
			receivedTargetIds := event.TargetIds
			//require.True(t, ok)

			//// event should be dispatched to a corrupted execution node
			//corruptedIdentity, ok := corruptedIdentityList.ByNodeID(corruptedId)
			//require.True(t, ok)
			//require.Equal(t, flow.RoleExecution, corruptedIdentity.Role)

			require.Equal(t, engine.PushReceipts, channel)
			//require.Equal(t, engine.ConsensusCommittee, channel)
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

	//err = wintermuteOrchestrator.HandleEventFromCorruptedNode(corruptedEn1.NodeID,
	//	engine.PushReceipts,
	//	completeExecutionReceipts[0].Receipts[0], //single execution receipt
	//	insecure.Protocol_UNICAST,
	//	uint32(0),
	//	targetIdentities.NodeIDs()[0])
	require.NoError(t, err)
}

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
