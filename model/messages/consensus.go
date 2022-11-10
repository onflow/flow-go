package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

type UntrustedExecutionResult struct {
	PreviousResultID flow.Identifier
	BlockID          flow.Identifier
	Chunks           []flow.Chunk
	ServiceEvents    flow.ServiceEventList
	ExecutionDataID  flow.Identifier
}

func (ur UntrustedExecutionResult) ToFlowResult() *flow.ExecutionResult {
	result := flow.ExecutionResult{
		PreviousResultID: ur.PreviousResultID,
		BlockID:          ur.PreviousResultID,
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

func UntrustedExecutionResultFromInternal(flowResult *flow.ExecutionResult) UntrustedExecutionResult {
	result := UntrustedExecutionResult{
		PreviousResultID: flowResult.PreviousResultID,
		BlockID:          flowResult.BlockID,
		ServiceEvents:    flowResult.ServiceEvents,
		ExecutionDataID:  flowResult.ExecutionDataID,
	}
	for _, chunk := range flowResult.Chunks {
		result.Chunks = append(result.Chunks, *chunk)
	}
	return result
}

type UntrustedBlockPayload struct {
	Guarantees []flow.CollectionGuarantee
	Seals      []flow.Seal
	Receipts   []flow.ExecutionReceiptMeta
	Results    []UntrustedExecutionResult
}

type UntrustedBlock struct {
	Header  flow.Header
	Payload UntrustedBlockPayload
}

func (ub UntrustedBlock) ToInternal() *flow.Block {
	block := flow.Block{
		Header:  &ub.Header,
		Payload: &flow.Payload{},
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
		block.Payload.Results = append(block.Payload.Results, result.ToFlowResult())
	}

	return &block
}

func UntrustedBlockFromInternal(flowBlock *flow.Block) UntrustedBlock {
	block := UntrustedBlock{
		Header: *flowBlock.Header,
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
