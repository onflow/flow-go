package wintermute

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/channels"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

// Orchestrator encapsulates a stateful implementation of wintermute attack orchestrator logic.
// The attack logic works as follows:
//  1. Orchestrator corrupts the result of the first incoming execution receipt from any of the corrupted execution node.
//  2. Orchestrator sends the corrupted execution result to all corrupted execution nodes.
//  3. If Orchestrator receives any chunk data pack request for a corrupted chunk from a corrupted verification node, it prompts the verifier to approve that chunk by sending it an attestation
//     for that corrupted chunk.
//  4. If Orchestrator receives any chunk data pack response from a corrupted execution node to an honest verification node, it drops the response
//     if it is for one of the corrupted chunks.
//  5. If Orchestrator receives a result approval for the original result (i.e., conflicting result with corrupted result),
//     from a corrupted VN, it drops it, hence no conflicting result with the corrupted result is getting approved by any
//     corrupted VN.
//  5. Any other incoming messages to the orchestrator are passed through, i.e., are sent as they are in the original Flow network without any tampering.
type Orchestrator struct {
	attackStateLock   sync.RWMutex // providing mutual exclusion for external reads to attack state.
	receiptHandleLock sync.Mutex   // ensuring at most one receipt is handled at a time, to avoid corrupting two concurrent receipts.

	logger zerolog.Logger
	state  *attackState

	corruptedNodeIds flow.IdentifierList // identifier of corrupted nodes.
	allNodeIds       flow.IdentityList   // identity of all nodes in the network (including non-corrupted ones)

	network insecure.OrchestratorNetwork
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(logger zerolog.Logger, corruptedNodeIds flow.IdentifierList, allIds flow.IdentityList) *Orchestrator {
	o := &Orchestrator{
		logger:           logger.With().Str("component", "wintermute-orchestrator").Logger(),
		corruptedNodeIds: corruptedNodeIds,
		allNodeIds:       allIds,
		state:            nil, // state is initialized to nil meaning no attack yet conducted.
	}

	return o
}

// Register sets the orchestrator network of the orchestrator.
func (o *Orchestrator) Register(network insecure.OrchestratorNetwork) {
	o.network = network
}

// HandleEgressEvent implements logic of processing the events received from a corrupted node.
//
// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
// the attack Orchestrator instead of dispatching them directly to the network.
// The Orchestrator completely determines what the corrupted conduit should send to the network.
func (o *Orchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := o.logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", event.TargetIds)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	switch event.FlowProtocolEvent.(type) {

	case *flow.ExecutionReceipt:
		// orchestrator received execution receipt from corrupted EN after EN executed a block.
		if err := o.handleExecutionReceiptEvent(event); err != nil {
			return fmt.Errorf("could not handle execution receipt event: %w", err)
		}
	case *messages.ChunkDataRequest:
		// orchestrator received chunk data request from corrupted VN after VN assigned a chunk, and VN about to request that chunk to verify it.
		if err := o.handleChunkDataPackRequestEvent(event); err != nil {
			return fmt.Errorf("could not handle chunk data pack request event: %w", err)
		}
	case *messages.ChunkDataResponse:
		// orchestrator received chunk data response from corrupted EN when EN wants to respond to a chunk data request from VN.
		if err := o.handleChunkDataPackResponseEvent(event); err != nil {
			return fmt.Errorf("could not handle chunk data pack response event: %w", err)
		}
	case *flow.ResultApproval:
		// orchestrator receives a result approval from corrupted VN. If it is an approval for the original result, it should
		// be wintermuted, i.e., a corrupted VN must not approve any conflicting result with the corrupted result (otherwise, it
		// causes a sealing halt at consensus nodes).
		if err := o.handleResultApprovalEvent(event); err != nil {
			return fmt.Errorf("could not handle result approval event: %w", err)
		}

	default:
		// Any other event is just passed through the network as it is.
		err := o.network.SendEgress(event)

		if err != nil {
			// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
			lg.Fatal().Err(err).Msg("egress event not passed through")
			return fmt.Errorf("could not send rpc egress event on channel: %w", err)
		}
		lg.Debug().Msg("egress event has passed through")
	}

	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
// Wintermute orchestrator doesn't corrupt ingress events, so it just passes them through.
func (o *Orchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := o.logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	err := o.network.SendIngress(event)

	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
		lg.Fatal().Err(err).Msg("ingress event not passed through")
		return fmt.Errorf("could not send rpc ingress event on channel: %w", err)
	}
	lg.Debug().Msg("ingress event has passed through")
	return nil
}

// corruptExecutionResult creates a corrupted version of the input receipt by tampering its content so that
// the resulted corrupted version would not pass verification.
func (o *Orchestrator) corruptExecutionResult(receipt *flow.ExecutionReceipt) *flow.ExecutionResult {
	receiptStartState := receipt.ExecutionResult.Chunks[0].StartState
	chunksNum := len(receipt.ExecutionResult.Chunks)

	result := &flow.ExecutionResult{
		PreviousResultID: receipt.ExecutionResult.PreviousResultID,
		BlockID:          receipt.ExecutionResult.BlockID,
		// replace all chunks with new ones to simulate chunk corruption
		Chunks: flow.ChunkList{
			unittest.ChunkFixture(receipt.ExecutionResult.BlockID, 0, unittest.WithChunkStartState(receiptStartState)),
		},
		ServiceEvents:   receipt.ExecutionResult.ServiceEvents,
		ExecutionDataID: receipt.ExecutionResult.ExecutionDataID,
	}

	if chunksNum > 1 {
		result.Chunks = append(result.Chunks, unittest.ChunkListFixture(uint(chunksNum-1), receipt.ExecutionResult.BlockID)...)
	}

	return result
}

// handleExecutionReceiptEvent processes incoming execution receipt event from a corrupted execution node.
// If no attack has already been conducted, it corrupts the result of receipt and sends it to all corrupted execution nodes.
// Otherwise, it just passes through the receipt to the sender.
func (o *Orchestrator) handleExecutionReceiptEvent(receiptEvent *insecure.EgressEvent) error {
	// ensuring at most one receipt is handled at a time, to avoid corrupting two concurrent receipts
	o.receiptHandleLock.Lock()
	defer o.receiptHandleLock.Unlock()

	ok := o.corruptedNodeIds.Contains(receiptEvent.CorruptOriginId)
	if !ok {
		return fmt.Errorf("sender of the event is not a corrupted node")
	}

	corruptedIdentity, ok := o.allNodeIds.ByNodeID(receiptEvent.CorruptOriginId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", receiptEvent.CorruptOriginId)
	}

	if corruptedIdentity.Role != flow.RoleExecution {
		return fmt.Errorf("wrong sender role for execution receipt: %s", corruptedIdentity.Role.String())
	}

	receipt, ok := receiptEvent.FlowProtocolEvent.(*flow.ExecutionReceipt)
	if !ok {
		return fmt.Errorf("protocol event is not an execution receipt: %T", receiptEvent.FlowProtocolEvent)
	}

	lg := o.logger.With().
		Hex("receipt_id", logging.ID(receipt.ID())).
		Hex("executor_id", logging.ID(receipt.ExecutorID)).
		Hex("result_id", logging.ID(receipt.ExecutionResult.ID())).
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("corrupted_id", logging.ID(receiptEvent.CorruptOriginId)).
		Str("protocol", insecure.ProtocolStr(receiptEvent.Protocol)).
		Str("channel", string(receiptEvent.Channel)).
		Uint32("targets_num", receiptEvent.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", receiptEvent.TargetIds)).Logger()

	if _, _, conducted := o.AttackState(); conducted {
		// an attack has already been conducted
		if receipt.ExecutionResult.ID() == o.state.originalResult.ID() {
			// receipt contains the original result that has been corrupted.
			// corrupted result must have already been sent to this node, so
			// just discard it.
			lg.Info().Msg("receipt event discarded")
			return nil
		}

		err := o.network.SendEgress(receiptEvent)
		if err != nil {
			return fmt.Errorf("could not send rpc on channel: %w", err)
		}
		lg.Info().Msg("receipt event passed through")
		return nil
	}

	// replace honest receipt with corrupted receipt
	corruptedResult := o.corruptExecutionResult(receipt)

	corruptedExecutionIds := o.allNodeIds.Filter(
		filter.And(filter.HasRole[flow.Identity](flow.RoleExecution),
			filter.HasNodeID[flow.Identity](o.corruptedNodeIds...))).NodeIDs()

	// sends corrupted execution result to all corrupted execution nodes.
	for _, corruptedExecutionId := range corruptedExecutionIds {
		// sets executor id of the result as the same corrupted execution node id that
		// is meant to send this message to the flow network.
		err := o.network.SendEgress(&insecure.EgressEvent{
			CorruptOriginId: corruptedExecutionId,
			Channel:         receiptEvent.Channel,
			Protocol:        receiptEvent.Protocol,
			TargetNum:       receiptEvent.TargetNum,
			TargetIds:       receiptEvent.TargetIds,

			// wrapping execution result in an execution receipt for sake of encoding and decoding.
			FlowProtocolEvent: &flow.ExecutionReceipt{ExecutionResult: *corruptedResult},
		})
		if err != nil {
			return fmt.Errorf("could not send rpc on channel: %w", err)
		}
		lg.Debug().
			Hex("corrupted_result_id", logging.ID(corruptedResult.ID())).
			Hex("corrupted_execution_id", logging.ID(corruptedExecutionId)).
			Msg("corrupted result successfully sent to corrupted execution node")
	}

	// saves state of attack for further replies
	o.updateAttackState(&receipt.ExecutionResult, corruptedResult)
	lg.Info().
		Hex("corrupted_result_id", logging.ID(corruptedResult.ID())).
		Msg("result successfully corrupted")

	return nil
}

// handleChunkDataPackRequestEvent processes a chunk data pack request event as follows:
// If request is for a corrupted chunk and comes from a corrupted verification node it is replied with an attestation for that
// chunk.
// Otherwise, it is passed through.
func (o *Orchestrator) handleChunkDataPackRequestEvent(chunkDataPackRequestEvent *insecure.EgressEvent) error {
	ok := o.corruptedNodeIds.Contains(chunkDataPackRequestEvent.CorruptOriginId)
	if !ok {
		return fmt.Errorf("sender of the event is not a corrupted node")
	}

	corruptedIdentity, ok := o.allNodeIds.ByNodeID(chunkDataPackRequestEvent.CorruptOriginId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", chunkDataPackRequestEvent.CorruptOriginId)
	}
	if corruptedIdentity.Role != flow.RoleVerification {
		return fmt.Errorf("wrong sender role for chunk data pack request: %s", corruptedIdentity.Role.String())
	}

	if _, _, conducted := o.AttackState(); conducted {
		// an attack has already been conducted
		sent, err := o.replyWithAttestation(chunkDataPackRequestEvent)
		if err != nil {
			return fmt.Errorf("could not reply with attestation: %w", err)
		}
		if sent {
			return nil
		}
	}

	// passing through the chunk data request unaltered because either:
	// 1) result corruption hasn't happened yet OR
	// 2) result corruption happened for a previous chunk and subsequent chunks will not be corrupted (since only the first chunk is corrupted)
	// Therefore, chunk data request is for an honest result, hence the corrupted verifier can follow
	// the protocol and send its honest chunk data request.
	err := o.network.SendEgress(chunkDataPackRequestEvent)
	if err != nil {
		return fmt.Errorf("could not send chunk data request: %w", err)
	}

	o.logger.Info().
		Hex("corrupted_id", logging.ID(chunkDataPackRequestEvent.CorruptOriginId)).
		Str("protocol", insecure.ProtocolStr(chunkDataPackRequestEvent.Protocol)).
		Str("channel", string(chunkDataPackRequestEvent.Channel)).
		Uint32("targets_num", chunkDataPackRequestEvent.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", chunkDataPackRequestEvent.TargetIds)).
		Msg("chunk data pack request event passed through")

	return nil
}

// handleChunkDataPackResponseEvent wintermutes the chunk data pack reply if it belongs to a corrupted result, and is meant to
// be sent to an honest verification node. Otherwise, it is passed through.
func (o *Orchestrator) handleChunkDataPackResponseEvent(chunkDataPackReplyEvent *insecure.EgressEvent) error {
	cdpRep := chunkDataPackReplyEvent.FlowProtocolEvent.(*messages.ChunkDataResponse)
	if _, _, conducted := o.AttackState(); conducted {
		// an attack has already been conducted
		lg := o.logger.With().
			Hex("chunk_id", logging.ID(cdpRep.ChunkDataPack.ChunkID)).
			Hex("sender_id", logging.ID(chunkDataPackReplyEvent.CorruptOriginId)).
			Hex("target_id", logging.ID(chunkDataPackReplyEvent.TargetIds[0])).Logger()

		// chunk data pack reply goes over a unicast, hence, we only check first target id.
		ok := o.corruptedNodeIds.Contains(chunkDataPackReplyEvent.TargetIds[0])
		if o.state.containsCorruptedChunkId(cdpRep.ChunkDataPack.ChunkID) {
			if !ok {
				// this is a chunk data pack response for a CORRUPTED chunk to an HONEST verification node
				// we WINTERMUTE it!
				// The orchestrator doesn't send anything back to the EN to send to the VN,
				// thereby indirectly causing the EN to wintermute the honest VN.
				lg.Info().Msg("wintermuted corrupted chunk data response to an honest verification node")
				return nil
			} else {
				// Illegal state! chunk data pack response for a CORRUPTED chunk to a CORRUPTED verification node.
				// Request must have never been reached to corrupted execution node, and must have been replaced with
				// an attestation.
				lg.Fatal().
					Msg("orchestrator received a chunk data response for corrupted chunk to a corrupted verification node")
			}
		}
	}

	// chunk data response is for an honest result (or before an attack has started), hence the corrupted execution node can follow the protocol and send (pass through) its honest response
	err := o.network.SendEgress(chunkDataPackReplyEvent)
	if err != nil {
		return fmt.Errorf("could not passed through chunk data reply: %w", err)
	}
	o.logger.Debug().
		Hex("corrupted_id", logging.ID(chunkDataPackReplyEvent.CorruptOriginId)).
		Hex("chunk_id", logging.ID(cdpRep.ChunkDataPack.ID())).
		Msg("chunk data pack response passed through")
	return nil
}

// handleResultApprovalEvent wintermutes the result approvals for the chunks of original result that are coming from
// corrupted verification nodes. Otherwise, it is passed through.
func (o *Orchestrator) handleResultApprovalEvent(resultApprovalEvent *insecure.EgressEvent) error {
	// non-nil state means a result has been corrupted, hence checking whether the approval
	// belongs to the chunks of the original (non-corrupted) result.
	approval := resultApprovalEvent.FlowProtocolEvent.(*flow.ResultApproval)
	lg := o.logger.With().
		Hex("result_id", logging.ID(approval.Body.ExecutionResultID)).
		Uint64("chunk_index", approval.Body.ChunkIndex).
		Hex("result_id", logging.ID(approval.Body.BlockID)).
		Hex("sender_id", logging.ID(resultApprovalEvent.CorruptOriginId)).
		Str("target_ids", fmt.Sprintf("%v", resultApprovalEvent.TargetIds)).Logger()

	if _, _, conducted := o.AttackState(); conducted {
		// an attack has already been conducted
		if o.state.originalResult.ID() == approval.Body.ExecutionResultID {
			lg.Info().Msg("wintermuting result approval for original un-corrupted execution result")
			return nil
		}
	}

	err := o.network.SendEgress(resultApprovalEvent)
	if err != nil {
		return fmt.Errorf("could not passed through result approval event %w", err)
	}
	lg.Info().Msg("result approval is passing through")
	return nil
}

// replyWithAttestation sends an attestation for the given chunk data pack request if it belongs to
// the corrupted result of orchestrator's state.
func (o *Orchestrator) replyWithAttestation(chunkDataPackRequestEvent *insecure.EgressEvent) (bool, error) {
	cdpReq := chunkDataPackRequestEvent.FlowProtocolEvent.(*messages.ChunkDataRequest)

	// a result corruption has already conducted
	if o.state.containsCorruptedChunkId(cdpReq.ChunkID) {
		// requested chunk belongs to corrupted result.
		corruptedChunkIndex, err := o.state.corruptedChunkIndexOf(cdpReq.ChunkID)
		if err != nil {
			return false, fmt.Errorf("could not find chunk index for corrupted chunk: %w", err)
		}

		attestation := &flow.Attestation{
			BlockID:           o.state.corruptedResult.BlockID,
			ExecutionResultID: o.state.corruptedResult.ID(),
			ChunkIndex:        corruptedChunkIndex,
		}

		// sends an attestation on behalf of verification node to all consensus nodes
		consensusIds := o.allNodeIds.Filter(filter.HasRole[flow.Identity](flow.RoleConsensus)).NodeIDs()
		err = o.network.SendEgress(&insecure.EgressEvent{
			CorruptOriginId: chunkDataPackRequestEvent.CorruptOriginId,
			Channel:         channels.PushApprovals,
			Protocol:        insecure.Protocol_PUBLISH,
			TargetNum:       0,
			TargetIds:       consensusIds,

			// wrapping attestation in a result approval for sake of encoding and decoding.
			FlowProtocolEvent: &flow.ResultApproval{Body: flow.ResultApprovalBody{Attestation: *attestation}},
		})
		if err != nil {
			return false, fmt.Errorf("could not send attestation for corrupted chunk: %w", err)
		}

		o.logger.Info().
			Hex("corrupted_id", logging.ID(chunkDataPackRequestEvent.CorruptOriginId)).
			Str("protocol", insecure.ProtocolStr(chunkDataPackRequestEvent.Protocol)).
			Str("channel", string(chunkDataPackRequestEvent.Channel)).
			Uint32("targets_num", chunkDataPackRequestEvent.TargetNum).
			Str("target_ids", fmt.Sprintf("%v", chunkDataPackRequestEvent.TargetIds)).
			Msg("chunk data pack request replied with attestation")

		return true, nil
	}

	return false, nil
}

// AttackState returns the corrupted and original execution results involved in this attack.
// Boolean return value determines whether attack conducted.
func (o *Orchestrator) AttackState() (flow.ExecutionResult, flow.ExecutionResult, bool) {
	o.attackStateLock.RLock()
	defer o.attackStateLock.RUnlock()

	if o.state == nil {
		// no attack yet conducted.
		return flow.ExecutionResult{}, flow.ExecutionResult{}, false
	}

	return *o.state.corruptedResult, *o.state.originalResult, true
}

func (o *Orchestrator) updateAttackState(originalResult *flow.ExecutionResult, corruptedResult *flow.ExecutionResult) {
	o.attackStateLock.Lock()
	defer o.attackStateLock.Unlock()

	if o.state != nil {
		// based on our testing assumptions, Wintermute attack must be conducted only once, extra attempts
		// can be due to a bug.
		panic("attempt on conducting an already conducted attack is not allowed")
	}

	o.state = &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}
}
