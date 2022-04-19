package wintermute

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

// Orchestrator encapsulates a stateful implementation of wintermute attack orchestrator logic.
// The attack logic works as follows:
// 1. Orchestrator corrupts the result of the first incoming execution receipt from any of the corrupted execution node.
// 2. Orchestrator sends the corrupted execution result to all corrupted execution nodes.
// 3. If Orchestrator receives any chunk data pack request for a corrupted chunk from a corrupted verification node, it replies it with an attestation
//    for that chunk.
// 4. If Orchestrator receives any chunk data pack response from a corrupted execution node to an honest verification node, it drops the response
//    if it is for one of the corrupted chunks.
// 5. Any other incoming messages to the orchestrator are passed through, i.e., are sent as they are in the original Flow network without any tampering.
type Orchestrator struct {
	sync.Mutex
	logger zerolog.Logger
	state  *attackState

	corruptedIds flow.IdentifierList // identifier of corrupted nodes.
	allIds       flow.IdentityList   // identity of all nodes in the network (including non-corrupted ones)

	network insecure.AttackNetwork
}

func NewOrchestrator(logger zerolog.Logger, corruptedIds flow.IdentifierList, allIds flow.IdentityList) *Orchestrator {
	o := &Orchestrator{
		logger:       logger.With().Str("component", "wintermute-orchestrator").Logger(),
		corruptedIds: corruptedIds,
		allIds:       allIds,
		state:        nil, // state is initialized to nil meaning no attack yet conducted.
	}

	return o
}

// WithAttackNetwork sets the attack network of the orchestrator.
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

	switch protocolEvent := event.FlowProtocolEvent.(type) {

	case *flow.ExecutionReceipt:
		if err := o.handleExecutionReceiptEvent(event); err != nil {
			return fmt.Errorf("could not handle execution receipt event: %w", err)
		}
	case *messages.ChunkDataRequest:
		if err := o.handleChunkDataPackRequestEvent(event); err != nil {
			return fmt.Errorf("could not handle chunk data pack request event: %w", err)
		}
	case *messages.ChunkDataResponse:
		// orchestrator received chunk data response from EN when EN wants to respond to a chunk data request from VN.
		if err := o.handleChunkDataPackResponseEvent(event); err != nil {
			return fmt.Errorf("could not handle chunk data pack response event: %w", err)
		}
	default:
		return fmt.Errorf("unexpected message type for wintermute attack orchestrator: %T", protocolEvent)
	}

	return nil
}

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

// handleExecutionReceiptEvent processes incoming execution receipt event from a corrupted execution node.
// If no attack has already been conducted, it corrupts the result of receipt and sends it to all corrupted execution nodes.
// Otherwise, it just passes through the receipt to the sender.
func (o *Orchestrator) handleExecutionReceiptEvent(receiptEvent *insecure.Event) error {
	ok := o.corruptedIds.Contains(receiptEvent.CorruptedId)
	if !ok {
		return fmt.Errorf("sender of the event is not a corrupted node")
	}

	corruptedIdentity, ok := o.allIds.ByNodeID(receiptEvent.CorruptedId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", receiptEvent.CorruptedId)
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
		Hex("corrupted_id", logging.ID(receiptEvent.CorruptedId)).
		Str("protocol", insecure.ProtocolStr(receiptEvent.Protocol)).
		Str("channel", string(receiptEvent.Channel)).
		Uint32("targets_num", receiptEvent.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", receiptEvent.TargetIds)).Logger()

	if o.state != nil {
		// non-nil state means an execution result has already been corrupted.
		if receipt.ExecutionResult.ID() == o.state.originalResult.ID() {
			// receipt contains the original result that has been corrupted.
			// corrupted result must have already been sent to this node, so
			// just discard it.
			lg.Info().Msg("receipt event discarded")
			return nil
		}

		err := o.network.Send(receiptEvent)
		if err != nil {
			return fmt.Errorf("could not send rpc on channel: %w", err)
		}

		lg.Info().Msg("receipt event passed through")
		return nil
	}

	// replace honest receipt with corrupted receipt
	corruptedResult := o.corruptExecutionResult(receipt)

	corruptedExecutionIds := o.allIds.Filter(
		filter.And(filter.HasRole(flow.RoleExecution),
			filter.HasNodeID(o.corruptedIds...))).NodeIDs()

	// sends corrupted execution result to all corrupted execution nodes.
	for _, corruptedExecutionId := range corruptedExecutionIds {
		// sets executor id of the result as the same corrupted execution node id that
		// is meant to send this message to the flow network.
		err := o.network.Send(&insecure.Event{
			CorruptedId:       corruptedExecutionId,
			Channel:           receiptEvent.Channel,
			Protocol:          receiptEvent.Protocol,
			TargetNum:         receiptEvent.TargetNum,
			TargetIds:         receiptEvent.TargetIds,
			FlowProtocolEvent: corruptedResult,
		})
		if err != nil {
			return fmt.Errorf("could not send rpc on channel: %w", err)
		}
	}

	// saves state of attack for further replies
	o.state = &attackState{
		originalResult:  &receipt.ExecutionResult,
		corruptedResult: corruptedResult,
	}
	lg.Info().
		Hex("corrupted_result_id", logging.ID(corruptedResult.ID())).
		Msg("result successfully corrupted")

	return nil
}

// handleChunkDataPackRequestEvent processes a chunk data pack request event as follows:
// If request is for a corrupted chunk and comes from a corrupted verification node it is replied with an attestation for that
// chunk.
// Otherwise, it is passed through.
func (o *Orchestrator) handleChunkDataPackRequestEvent(chunkDataPackRequestEvent *insecure.Event) error {
	ok := o.corruptedIds.Contains(chunkDataPackRequestEvent.CorruptedId)
	if !ok {
		return fmt.Errorf("sender of the event is not a corrupted node")
	}

	corruptedIdentity, ok := o.allIds.ByNodeID(chunkDataPackRequestEvent.CorruptedId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", chunkDataPackRequestEvent.CorruptedId)
	}
	if corruptedIdentity.Role != flow.RoleVerification {
		return fmt.Errorf("wrong sender role for chunk data pack request: %s", corruptedIdentity.Role.String())
	}

	if o.state != nil {
		sent, err := o.replyWithAttestation(chunkDataPackRequestEvent)
		if err != nil {
			return fmt.Errorf("could not reply with attestation: %w", err)
		}
		if sent {
			return nil
		}
	}

	// no result corruption yet conducted, hence bouncing back the chunk data request.
	err := o.network.Send(chunkDataPackRequestEvent)
	if err != nil {
		return fmt.Errorf("could not send chunk data request: %w", err)
	}

	o.logger.Info().
		Hex("corrupted_id", logging.ID(chunkDataPackRequestEvent.CorruptedId)).
		Str("protocol", insecure.ProtocolStr(chunkDataPackRequestEvent.Protocol)).
		Str("channel", string(chunkDataPackRequestEvent.Channel)).
		Uint32("targets_num", chunkDataPackRequestEvent.TargetNum).
		Str("target_ids", fmt.Sprintf("%v", chunkDataPackRequestEvent.TargetIds)).
		Msg("chunk data pack request event passed through")

	return nil
}

// handleChunkDataPackResponseEvent wintermutes the chunk data pack reply if it belongs to a corrupted result, and is meant to
// be sent to an honest verification node. Otherwise, it is passed through.
func (o *Orchestrator) handleChunkDataPackResponseEvent(chunkDataPackReplyEvent *insecure.Event) error {
	if o.state != nil {
		cdpRep := chunkDataPackReplyEvent.FlowProtocolEvent.(*messages.ChunkDataResponse)

		lg := o.logger.With().
			Hex("chunk_id", logging.ID(cdpRep.ChunkDataPack.ChunkID)).
			Hex("sender_id", logging.ID(chunkDataPackReplyEvent.CorruptedId)).
			Hex("target_id", logging.ID(chunkDataPackReplyEvent.TargetIds[0])).Logger()

		// chunk data pack reply goes over a unicast, hence, we only check first target id.
		ok := o.corruptedIds.Contains(chunkDataPackReplyEvent.TargetIds[0])
		if o.state.containsCorruptedChunkId(cdpRep.ChunkDataPack.ChunkID) {
			if !ok {
				// this is a chunk data pack response for a CORRUPTED chunk to an HONEST verification node
				// we WINTERMUTE it!
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

	// no result corruption yet conducted, hence bouncing back the chunk data request.
	err := o.network.Send(chunkDataPackReplyEvent)
	if err != nil {
		return fmt.Errorf("could not passed through chunk data reply: %w", err)
	}
	return nil
}

// replyWithAttestation sends an attestation for the given chunk data pack request if it belongs to
// the corrupted result of orchestrator's state.
func (o *Orchestrator) replyWithAttestation(chunkDataPackRequestEvent *insecure.Event) (bool, error) {
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

		// sends an attestation for the corrupted chunk to corrupted verification node.
		err = o.network.Send(&insecure.Event{
			CorruptedId:       chunkDataPackRequestEvent.CorruptedId,
			Channel:           chunkDataPackRequestEvent.Channel,
			Protocol:          chunkDataPackRequestEvent.Protocol,
			TargetNum:         chunkDataPackRequestEvent.TargetNum,
			TargetIds:         chunkDataPackRequestEvent.TargetIds,
			FlowProtocolEvent: attestation,
		})
		if err != nil {
			return false, fmt.Errorf("could not send attestation for corrupted chunk: %w", err)
		}

		o.logger.Info().
			Hex("corrupted_id", logging.ID(chunkDataPackRequestEvent.CorruptedId)).
			Str("protocol", insecure.ProtocolStr(chunkDataPackRequestEvent.Protocol)).
			Str("channel", string(chunkDataPackRequestEvent.Channel)).
			Uint32("targets_num", chunkDataPackRequestEvent.TargetNum).
			Str("target_ids", fmt.Sprintf("%v", chunkDataPackRequestEvent.TargetIds)).
			Msg("chunk data pack request event passed through")

		return true, nil
	}

	return false, nil
}
