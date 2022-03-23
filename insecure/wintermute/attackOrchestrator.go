package wintermute

import (
	"fmt"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

// Orchestrator encapsulates a stateful implementation of wintermute attack orchestrator logic.
type Orchestrator struct {
	component.Component
	logger       zerolog.Logger
	network      insecure.AttackNetwork
	corruptedIds flow.IdentityList
	allIds       flow.IdentityList // identity of all nodes in the network (including non-corrupted ones)
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(allIds flow.IdentityList, corruptedIds flow.IdentityList, attackNetwork insecure.AttackNetwork, logger zerolog.Logger) *Orchestrator {
	o := &Orchestrator{
		logger:       logger,
		network:      attackNetwork,
		corruptedIds: corruptedIds,
		allIds:       allIds,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			o.start(ctx)

			ready()

			<-ctx.Done()
		}).Build()

	o.Component = cm

	return o
}

// start performs the startup of orchestrator components.
func (o *Orchestrator) start(ctx irrecoverable.SignalerContext) {
	o.network.Start(ctx)
}

// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
func (o *Orchestrator) HandleEventFromCorruptedNode(corruptedId flow.Identifier,
	channel network.Channel,
	event interface{},
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) error {

	corruptedIdentity, ok := o.corruptedIds.ByNodeID(corruptedId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", corruptedId)
	}

	// TODO: how do we keep track of state between calls to HandleEventFromCorruptedNode()?
	// there will be many events sent to the orchestrator and we need a way to co-ordinate all the event calls

	switch corruptedIdentity.Role {
	// this switch case should be from
	case flow.RoleExecution:
		// corrupt execution result
		// TODO: do we corrupt a single execution result or all of them?
		// TODO: how do we corrupt each execution result?
		// e.g. honestExecutionResult1.Chunks[0].CollectionIndex = 999
		// TODO: how do we allow unit tests to assert execution result(s) was corrupted? Return type is error

		// extract execution receipt so we can corrupt it
		executionReceipt := event.(*flow.ExecutionReceipt)

		// replace all chunks with new ones to simulate chunk corruption
		fmt.Println("before corruption", executionReceipt.ExecutionResult.ID())
		corruptReceipt := &flow.ExecutionReceipt{
			ExecutorID: executionReceipt.ExecutorID,
			ExecutionResult: flow.ExecutionResult{
				PreviousResultID: executionReceipt.ExecutionResult.PreviousResultID,
				BlockID:          executionReceipt.ExecutionResult.BlockID,
				Chunks:           unittest.ChunkListFixture(uint(len(executionReceipt.ExecutionResult.Chunks)), executionReceipt.ExecutionResult.BlockID),
				ServiceEvents:    executionReceipt.ExecutionResult.ServiceEvents,
				ExecutionDataID:  executionReceipt.ExecutionResult.ExecutionDataID,
			},
			ExecutorSignature: executionReceipt.ExecutorSignature,
			Spocks:            executionReceipt.Spocks,
		}
		//unittest.Wiafter ipts()
		fmt.Println("after corruption", executionReceipt.ExecutionResult.ID())

		// save all corrupted chunks so can create result approvals for them
		// can just create result approvals here and save them

		err := o.network.RpcUnicastOnChannel(corruptedId, channel, corruptReceipt, targetIds[0])
		if err != nil {
			return fmt.Errorf("could not send rpc on channel: %w", err)
		}

	// need to send corrupted message to 2nd EN as well

	// send corrupted result approvals to consensus node

	case flow.RoleVerification:
		// if the event is a chunk data request for any of the chunks of the corrupted result, then:
		// if coming from an honest verification node -> do nothing (wintermuting the honest verification node).
		// if coming from a corrupted verification node -> create and send the result approval for the requested
		// chunk
		// request coming from verification node

		request := event.(*messages.ChunkDataRequest)
		request.ChunkID.String()

		// go through the saved list of corrupted execution receipt - there should only be 1 execution receipt
		// go through the execution result (should only be 1) in that execution receipt
		// go through all the chunks in that execution result and check for a match with "ChunkID flow.Identifier" from ChunkDataRequest
		// if no matches across all chunks, then send (we'll go over next time)
		// if there is match, fill in result approval (attestation part) and send to verification node

		// we need to lookup the chunk with the requested chunk id
		// if that chunk belongs to the corrupted result
		// we need to fill in an approval for it.
		approval := flow.ResultApproval{
			Body: flow.ResultApprovalBody{
				// attestation needs to be filled in
				//
				Attestation: flow.Attestation{
					BlockID:           flow.Identifier{},
					ExecutionResultID: flow.Identifier{},
					ChunkIndex:        0,
				},
				ApproverID:           flow.Identifier{}, // should be filled by corrupte vn Id
				AttestationSignature: nil,               // can be done later (at verification node side)
				Spock:                nil,               // can be done later (at verification node side)
			},
			VerifierSignature: nil,
		}
		approval.ID()

	default:
		// TODO should we return an error when role is neither EN nor VN?
		panic("unexpected role for Wintermute attack")
	}

	return nil
}
