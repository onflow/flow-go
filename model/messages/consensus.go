package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// UntrustedExecutionResult is a duplicate of flow.ExecutionResult used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// flow.ExecutionResult type.
// Deprecated: Please update flow.ExecutionResult to use []flow.Chunk, then
// replace instances of this type with flow.ExecutionResult
type UntrustedExecutionResult struct {
	PreviousResultID flow.Identifier
	BlockID          flow.Identifier
	Chunks           []flow.Chunk
	ServiceEvents    flow.ServiceEventList
	ExecutionDataID  flow.Identifier
}

// ToInternal returns the internal representation of the type.
func (ur *UntrustedExecutionResult) ToInternal() *flow.ExecutionResult {
	result := flow.ExecutionResult{
		PreviousResultID: ur.PreviousResultID,
		BlockID:          ur.BlockID,
		Chunks:           make(flow.ChunkList, 0, len(ur.Chunks)),
		ServiceEvents:    ur.ServiceEvents,
		ExecutionDataID:  ur.ExecutionDataID,
	}
	for _, chunk := range ur.Chunks {
		chunk := chunk
		result.Chunks = append(result.Chunks, &chunk)
	}
	return &result
}

// UntrustedExecutionResultFromInternal converts the internal flow.ExecutionResult representation
// to the representation used in untrusted messages.
func UntrustedExecutionResultFromInternal(internal *flow.ExecutionResult) UntrustedExecutionResult {
	result := UntrustedExecutionResult{
		PreviousResultID: internal.PreviousResultID,
		BlockID:          internal.BlockID,
		ServiceEvents:    internal.ServiceEvents,
		ExecutionDataID:  internal.ExecutionDataID,
	}
	for _, chunk := range internal.Chunks {
		result.Chunks = append(result.Chunks, *chunk)
	}
	return result
}

// UntrustedBlockPayload is a duplicate of flow.Payload used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// flow.Payload type.
// Deprecated: Please update flow.Payload to use []flow.Guarantee etc., then
// replace instances of this type with flow.Payload
type UntrustedBlockPayload struct {
	Guarantees []flow.CollectionGuarantee
	Seals      []flow.Seal
	Receipts   []flow.ExecutionReceiptMeta
	Results    []UntrustedExecutionResult
}

func (up UntrustedBlockPayload) ToInternal() *flow.Payload {
	payload := &flow.Payload{}
	for _, guarantee := range up.Guarantees {
		guarantee := guarantee
		payload.Guarantees = append(payload.Guarantees, &guarantee)
	}
	for _, seal := range up.Seals {
		seal := seal
		payload.Seals = append(payload.Seals, &seal)
	}
	for _, receipt := range up.Receipts {
		receipt := receipt
		payload.Receipts = append(payload.Receipts, &receipt)
	}
	for _, result := range up.Results {
		result := result
		payload.Results = append(payload.Results, result.ToInternal())
	}
	return payload
}

type GenericToInternal[T flow.GenericPayload] interface {
	ToInternal() T
}

// GenericUntrustedBlock is a duplicate of flow.Block used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// flow.Block type.
// Deprecated: Please update flow.Payload to use []flow.Guarantee etc., then
// replace instances of this type with flow.Block
type GenericUntrustedBlock[TrustedPayload flow.GenericPayload] struct {
	Header  flow.Header
	Payload GenericToInternal[TrustedPayload]
}

// ToInternal returns the internal representation of the type.
func (ub *GenericUntrustedBlock[TrustedPayload]) ToInternal() *flow.GenericBlock[TrustedPayload] {
	return &flow.GenericBlock[TrustedPayload]{
		Header:  &ub.Header,
		Payload: ub.Payload.ToInternal(),
	}
}

type UntrustedBlock = GenericUntrustedBlock[*flow.Payload]

// UntrustedBlockFromInternal converts the internal flow.Block representation
// to the representation used in untrusted messages.
func UntrustedBlockFromInternal(flowBlock *flow.Block) UntrustedBlock {
	payload := UntrustedBlockPayload{}
	for _, guarantee := range flowBlock.Payload.Guarantees {
		payload.Guarantees = append(payload.Guarantees, *guarantee)
	}
	for _, seal := range flowBlock.Payload.Seals {
		payload.Seals = append(payload.Seals, *seal)
	}
	for _, receipt := range flowBlock.Payload.Receipts {
		payload.Receipts = append(payload.Receipts, *receipt)
	}
	for _, result := range flowBlock.Payload.Results {
		payload.Results = append(payload.Results, UntrustedExecutionResultFromInternal(result))
	}

	return UntrustedBlock{
		Header:  *flowBlock.Header,
		Payload: payload,
	}
}

type GenericBlockProposal[TrustedPayload flow.GenericPayload] struct {
	Block GenericUntrustedBlock[TrustedPayload]
}

// BlockProposal is part of the consensus protocol and represents the leader
// of a consensus round pushing a new proposal to the network.
type BlockProposal = GenericBlockProposal[*flow.Payload]

//func NewBlockProposal(internal *flow.GenericBlock[flow.GenericPayload]) *BlockProposal[flow.GenericPayload, GenericToInternal[flow.GenericPayload]] {
//	return &BlockProposal[flow.GenericPayload, GenericToInternal[flow.GenericPayload]]{
//		Block: internal,
//	}
//}

func NewBlockProposal(internal *flow.Block) *BlockProposal {
	return &BlockProposal{
		Block: UntrustedBlockFromInternal(internal),
	}
}

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}
