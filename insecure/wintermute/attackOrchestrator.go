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
// Otherwise, it just bounces back the receipt to the sender.
func (o *Orchestrator) handleExecutionReceiptEvent(receiptEvent *insecure.Event) error {
	corruptedIdentity, ok := o.corruptedIds.ByNodeID(receiptEvent.CorruptedId)
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

	if o.state != nil {
		// non-nil state means an execution result has already been corrupted.
		if receipt.ExecutionResult.ID() == o.state.originalResult.ID() {
			// receipt contains the original result that has been corrupted.
			// corrupted result must have already been sent to this node, so
			// just discards it.
			return nil
		}

		err := o.network.Send(receiptEvent)
		if err != nil {
			return fmt.Errorf("could not send rpc on channel: %w", err)
		}
		return nil
	}

	// replace honest receipt with corrupted receipt
	corruptedResult := o.corruptExecutionResult(receipt)

	corruptedExecutionIds := o.corruptedIds.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs()

	// sends corrupted execution receipt to all corrupted execution nodes.
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

		// saves state of attack for further replies
		o.state = &attackState{
			originalResult:         &receipt.ExecutionResult,
			corruptedResult:        corruptedResult,
			originalChunkIds:       flow.GetIDs(receipt.ExecutionResult.Chunks),
			corruptedChunkIds:      flow.GetIDs(corruptedResult.Chunks),
			corruptedChunkIndexMap: chunkIndexMap(corruptedResult.Chunks),
		}
	}

	return nil
}

// handleChunkDataPackRequestEvent processes a chunk data pack request event as follows:
// If request is for a corrupted chunk and comes from a corrupted verification node it is replied wtih an attestation for that
// chunk.
// Otherwise, it is bounced back.
func (o *Orchestrator) handleChunkDataPackRequestEvent(chunkDataPackRequestEvent *insecure.Event) error {
	corruptedIdentity, ok := o.corruptedIds.ByNodeID(chunkDataPackRequestEvent.CorruptedId)
	if !ok {
		return fmt.Errorf("could not find corrupted identity for: %x", chunkDataPackRequestEvent.CorruptedId)
	}
	if corruptedIdentity.Role != flow.RoleVerification {
		return fmt.Errorf("wrong sender role for chunk data pack request: %s", corruptedIdentity.Role.String())
	}

	if o.state != nil {
		return o.replyWithAttestation(chunkDataPackRequestEvent)
	}

	// no result corruption yet conducted, hence bouncing back the chunk data request.
	err := o.network.Send(chunkDataPackRequestEvent)
	if err != nil {
		return fmt.Errorf("could not send chunk data request: %w", err)
	}
	return nil
}

// handleChunkDataPackResponseEvent Wintermutes the chunk data pack reply if it belongs to a corrupted result, and is meant to
// be sent to an honest verification node. Otherwise, it bounces it back.
func (o *Orchestrator) handleChunkDataPackResponseEvent(chunkDataPackReplyEvent *insecure.Event) error {
	if o.state != nil {
		cdpRep := chunkDataPackReplyEvent.FlowProtocolEvent.(*messages.ChunkDataResponse)

		// chunk data pack reply goes over a unicast, hence, we only check first target id.
		_, ok := o.corruptedIds.ByNodeID(chunkDataPackReplyEvent.TargetIds[0])
		if o.state.corruptedChunkIds.Contains(cdpRep.ChunkDataPack.ChunkID) {
			if !ok {
				// this is a chunk data pack response for a CORRUPTED chunk to an HONEST verification node
				// we WINTERMUTE it!
				return nil
			} else {
				// Illegal state! chunk data pack response for a CORRUPTED chunk to a CORRUPTED verification node.
				// Request must have never been reached to corrupted execution node, and must have been replaced with
				// an attestation.
				o.logger.Fatal().
					Hex("chunk_id", logging.ID(cdpRep.ChunkDataPack.ChunkID)).
					Hex("sender_id", logging.ID(chunkDataPackReplyEvent.CorruptedId)).
					Hex("target_id", logging.ID(chunkDataPackReplyEvent.TargetIds[0])).
					Msg("orchestrator received a chunk data response for corrupted chunk to a corrupted verification node")
			}
		}
	}

	// no result corruption yet conducted, hence bouncing back the chunk data request.
	err := o.network.Send(chunkDataPackReplyEvent)
	if err != nil {
		return fmt.Errorf("could not bounce back chunk data reply: %w", err)
	}
	return nil
}

// replyWithAttestation sends an attestation for the given chunk data pack request if it belongs to
// the corrupted result of orchestrator's state.
func (o *Orchestrator) replyWithAttestation(chunkDataPackRequestEvent *insecure.Event) error {
	cdpReq := chunkDataPackRequestEvent.FlowProtocolEvent.(*messages.ChunkDataRequest)

	// a result corruption has already conducted
	if o.state.corruptedChunkIds.Contains(cdpReq.ChunkID) {
		// requested chunk belongs to corrupted result.
		attestation := &flow.Attestation{
			BlockID:           o.state.corruptedResult.BlockID,
			ExecutionResultID: o.state.corruptedResult.ID(),
			ChunkIndex:        o.state.corruptedChunkIndexMap[cdpReq.ChunkID],
		}

		// sends an attestation for the corrupted chunk to corrupted verification node.
		err := o.network.Send(&insecure.Event{
			CorruptedId:       chunkDataPackRequestEvent.CorruptedId,
			Channel:           chunkDataPackRequestEvent.Channel,
			Protocol:          chunkDataPackRequestEvent.Protocol,
			TargetNum:         chunkDataPackRequestEvent.TargetNum,
			TargetIds:         chunkDataPackRequestEvent.TargetIds,
			FlowProtocolEvent: attestation,
		})
		if err != nil {
			return fmt.Errorf("could not send attestation for corrupted chunk: %w", err)
		}
	}

	return nil
}

// chunkIndexMap returns the map from chunks to indices.
func chunkIndexMap(chunkList flow.ChunkList) map[flow.Identifier]uint64 {
	cm := make(map[flow.Identifier]uint64)
	for _, chunk := range chunkList {
		cm[chunk.ID()] = chunk.Index
	}

	return cm
}
