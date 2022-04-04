package wintermute

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

// Orchestrator encapsulates a stateful implementation of wintermute attack orchestrator logic.
type Orchestrator struct {
	sync.Mutex
	state        *attackState
	logger       zerolog.Logger
	network      insecure.AttackNetwork
	corruptedIds flow.IdentityList
	allIds       flow.IdentityList // identity of all nodes in the network (including non-corrupted ones)
}

func NewOrchestrator(allIds flow.IdentityList, corruptedIds flow.IdentityList, logger zerolog.Logger) *Orchestrator {
	o := &Orchestrator{
		logger:       logger,
		corruptedIds: corruptedIds,
		allIds:       allIds,
		state:        nil,
	}

	return o
}

func (o *Orchestrator) WithAttackNetwork(network insecure.AttackNetwork) {
	o.network = network
}

// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attacker instead of dispatching them to the network.
func (o *Orchestrator) HandleEventFromCorruptedNode(event *insecure.Event) error {
	o.Lock()
	defer o.Unlock()

	corruptedIdentity, ok := o.corruptedIds.ByNodeID(event.CorruptedId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", event.CorruptedId)
	}

	switch protocolEvent := event.FlowProtocolEvent.(type) {

	case *flow.ExecutionReceipt:
		if corruptedIdentity.Role != flow.RoleExecution {
			return fmt.Errorf("wrong sender role for execution receipt: %s", corruptedIdentity.Role.String())
		}

		if o.state != nil {
			// non-nil state means an execution result has already been corrupted.
			if protocolEvent.ExecutionResult.ID() == o.state.originalResult.ID() {
				// receipt contains the original result that has been corrupted.
				// corrupted result must have already been sent to this node, so
				// just discards it.
				return nil
			}

			err := o.network.Send(event)
			if err != nil {
				return fmt.Errorf("could not send rpc on channel: %w", err)
			}
			return nil
		}

		// replace honest receipt with corrupted receipt
		corruptedResult := o.corruptExecutionResult(protocolEvent)
		// save all corrupted chunks so can create result approvals for them
		// can just create result approvals here and save them

		corruptedExecutionIds := o.corruptedIds.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs()

		// sends corrupted execution receipt to all corrupted execution nodes.
		for _, corruptedExecutionId := range corruptedExecutionIds {
			// sets executor id of the result as the same corrupted execution node id that
			// is meant to send this message to the flow network.
			err := o.network.Send(&insecure.Event{
				CorruptedId:       corruptedExecutionId,
				Channel:           event.Channel,
				Protocol:          event.Protocol,
				TargetNum:         event.TargetNum,
				TargetIds:         event.TargetIds,
				FlowProtocolEvent: corruptedResult,
			})
			if err != nil {
				return fmt.Errorf("could not send rpc on channel: %w", err)
			}

			// saves state of attack for further replies
			o.state = &attackState{
				originalResult:    &protocolEvent.ExecutionResult,
				corruptedResult:   corruptedResult,
				originalChunkIds:  flow.GetIDs(protocolEvent.ExecutionResult.Chunks),
				corruptedChunkIds: flow.GetIDs(corruptedResult.Chunks),
			}
		}

	default:
		return fmt.Errorf("unexpected message type for wintermute attack orchestrator: %T", protocolEvent)
	}

	return nil
}

// TODO: how do we keep track of state between calls to HandleEventFromCorruptedNode()?
// there will be many events sent to the orchestrator and we need a way to co-ordinate all the event calls

//switch corruptedIdentity.Role {
//case flow.RoleVerification:
//	// if the event is a chunk data request for any of the chunks of the corrupted result, then:
//	// if coming from an honest verification node -> do nothing (wintermuting the honest verification node).
//	// if coming from a corrupted verification node -> create and send the result approval for the requested
//	// chunk
//	// request coming from verification node
//
//	request := event.FlowProtocolEvent.(*messages.ChunkDataRequest)
//	request.ChunkID.String()
//
//	// go through the saved list of corrupted execution receipt - there should only be 1 execution receipt
//	// go through the execution result (should only be 1) in that execution receipt
//	// go through all the chunks in that execution result and check for a match with "ChunkID flow.Identifier" from ChunkDataRequest
//	// if no matches across all chunks, then send (we'll go over next time)
//	// if there is match, fill in result approval (attestation part) and send to verification node
//
//	// we need to lookup the chunk with the requested chunk id
//	// if that chunk belongs to the corrupted result
//	// we need to fill in an approval for it.
//	approval := flow.ResultApproval{
//		Body: flow.ResultApprovalBody{
//			// attestation needs to be filled in
//			//
//			Attestation: flow.Attestation{
//				BlockID:           flow.Identifier{},
//				ExecutionResultID: flow.Identifier{},
//				ChunkIndex:        0,
//			},
//			ApproverID:           flow.Identifier{}, // should be filled by corrupted VN ID
//			AttestationSignature: nil,               // can be done later (at verification node side)
//			Spock:                nil,               // can be done later (at verification node side)
//		},
//		VerifierSignature: nil,
//	}
//	approval.ID()

// corruptExecutionResult creates a corrupted version of the input receipt by tampering its content so that
// the resulted corrupted version would not pass verification.
func (o *Orchestrator) corruptExecutionResult(receipt *flow.ExecutionReceipt) *flow.ExecutionResult {
	return &flow.ExecutionResult{
		PreviousResultID: receipt.ExecutionResult.PreviousResultID,
		BlockID:          receipt.ExecutionResult.BlockID,
		// replace all chunks with new ones to simulate chunk corruption
		Chunks:          unittest.ChunkListFixture(uint(len(receipt.ExecutionResult.Chunks)), receipt.ExecutionResult.BlockID),
		ServiceEvents:   receipt.ExecutionResult.ServiceEvents,
		ExecutionDataID: receipt.ExecutionResult.ExecutionDataID,
	}
}
