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
	Guarantees      []flow.CollectionGuarantee
	Seals           []flow.Seal
	Receipts        []flow.ExecutionReceiptMeta
	Results         []UntrustedExecutionResult
	ProtocolStateID flow.Identifier
}

// UntrustedBlock is a duplicate of flow.Block used within
// untrusted messages. It exists only to provide a memory-safe structure for
// decoding messages and should be replaced in the future by updating the core
// flow.Block type.
// Deprecated: Please update flow.Payload to use []flow.Guarantee etc., then
// replace instances of this type with flow.Block
type UntrustedBlock struct {
	Header  flow.Header
	Payload UntrustedBlockPayload
}

// ToInternal returns the internal representation of the type.
func (ub *UntrustedBlock) ToInternal() *flow.Block {
	block := flow.Block{
		Header: &ub.Header,
		Payload: &flow.Payload{
			ProtocolStateID: ub.Payload.ProtocolStateID,
		},
	}
	for _, guarantee := range ub.Payload.Guarantees {
		guarantee := guarantee
		block.Payload.Guarantees = append(block.Payload.Guarantees, &guarantee)
	}
	for _, seal := range ub.Payload.Seals {
		seal := seal
		block.Payload.Seals = append(block.Payload.Seals, &seal)
	}
	for _, receipt := range ub.Payload.Receipts {
		receipt := receipt
		block.Payload.Receipts = append(block.Payload.Receipts, &receipt)
	}
	for _, result := range ub.Payload.Results {
		result := result
		block.Payload.Results = append(block.Payload.Results, result.ToInternal())
	}

	return &block
}

// UntrustedBlockFromInternal converts the internal flow.Block representation
// to the representation used in untrusted messages.
func UntrustedBlockFromInternal(flowBlock *flow.Block) UntrustedBlock {
	block := UntrustedBlock{
		Header: *flowBlock.Header,
		Payload: UntrustedBlockPayload{
			ProtocolStateID: flowBlock.Payload.ProtocolStateID,
		},
	}
	for _, guarantee := range flowBlock.Payload.Guarantees {
		block.Payload.Guarantees = append(block.Payload.Guarantees, *guarantee)
	}
	for _, seal := range flowBlock.Payload.Seals {
		block.Payload.Seals = append(block.Payload.Seals, *seal)
	}
	for _, receipt := range flowBlock.Payload.Receipts {
		block.Payload.Receipts = append(block.Payload.Receipts, *receipt)
	}
	for _, result := range flowBlock.Payload.Results {
		block.Payload.Results = append(block.Payload.Results, UntrustedExecutionResultFromInternal(result))
	}
	return block
}

// BlockProposal is part of the consensus protocol and represents the leader
// of a consensus round pushing a new proposal to the network.
type BlockProposal struct {
	Block UntrustedBlock
}

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

// TimeoutObject is part of the consensus protocol and represents a consensus node
// timing out in given round. Contains a sequential number for deduplication purposes.
type TimeoutObject struct {
	TimeoutTick uint64
	View        uint64
	NewestQC    *flow.QuorumCertificate
	LastViewTC  *flow.TimeoutCertificate
	SigData     []byte
}
