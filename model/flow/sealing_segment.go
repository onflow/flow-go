package flow

import (
	"fmt"
)

// SealingSegment is the chain segment such that the last block (greatest
// height) is this snapshot's reference block and the first (least height)
// is the most recently sealed block as of this snapshot (ie. the block
// referenced by LatestSeal).
// In other words, the last block contains a seal for the first block.
// For instance:
//   A <- B <- C <- D <- E (seal_A)
// The above sealing segment's last block (E) has a seal for block A, which is
// the first block of the sealing segment.
//
// It is guaranteed there are at least 2 blocks in a SealingSegment
// The segment is in ascending height order.
type SealingSegment struct {
	// Blocks the chain segment blocks
	Blocks []*Block

	// Due to decoupling of execution receipts it's possible that blocks from sealing segment will be referring
	// execution results incorporated in blocks that aren't part of the segment.
	// ExecutionResults will contain those results.
	ExecutionResults ExecutionResultList
}

func (segment *SealingSegment) Highest() *Block {
	return segment.Blocks[len(segment.Blocks)-1]
}

func (segment *SealingSegment) Lowest() *Block {
	return segment.Blocks[0]
}

var (
	ErrSegmentMissingSeal        = fmt.Errorf("sealing segment failed sanity check: highest block in segment does not contain seal for lowest")
	ErrSegmentBlocksWrongLen     = fmt.Errorf("sealing segment failed sanity check: must have atleast 2 blocks")
	ErrSegmentInvalidBlockHeight = fmt.Errorf("sealing segment failed sanity check: blocks must be in ascending order")
	ErrSegmentResultLookup = fmt.Errorf("failed to lookup execution result")
)

type SealingSegmentBuilder struct {
	resultLookup    func(resultID Identifier) (*ExecutionResult, error)
	includedResults map[Identifier]*ExecutionResult
	blocks          []*Block
	results         []*ExecutionResult
}

// AddBlock appends block to blocks
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	//sanity check: block should be 1 height higher than current highest
	if !builder.isValidHeight(block) {
		 return fmt.Errorf("invalid block height (%x): %w", block.Header.Height, ErrSegmentInvalidBlockHeight)
	}

	// cache results in included results
	// they could be referenced in a future block in the segment
	for _, result := range block.Payload.Results.Lookup() {
		builder.includedResults[result.ID()] = result
	}

	for _, receipt := range block.Payload.Receipts {
		if _, ok := builder.includedResults[receipt.ResultID]; !ok {
			result, err := builder.resultLookup(receipt.ResultID)
			if err != nil {
				return fmt.Errorf("%w: (%x) %v", ErrSegmentResultLookup,receipt.ResultID, err)
			}

			builder.addExecutionResult(result)
			builder.includedResults[receipt.ResultID] = result
		}
	}
	builder.blocks = append(builder.blocks, block)
	return nil
}

// AddExecutionResult adds result to executionResults
func (builder *SealingSegmentBuilder) addExecutionResult(result *ExecutionResult) {
	builder.results = append(builder.results, result)
}

// SealingSegment will check if the highest block has a seal for the lowest block in the segment
func (builder *SealingSegmentBuilder) SealingSegment() (*SealingSegment, error) {
	segment := &SealingSegment{
		Blocks:           builder.blocks,
		ExecutionResults: builder.results,
	}

	if len(segment.Blocks) < 2 {
		return nil, fmt.Errorf("expect at least 2 blocks in a sealing segment, but actually got %v: %w", len(segment.Blocks), ErrSegmentBlocksWrongLen)
	}

	if !builder.hasValidSeal() {
		return nil, fmt.Errorf("sealing segment missing seal lowest (%x) highest (%x): %w", segment.Lowest().ID(), segment.Highest().ID(), ErrSegmentMissingSeal)
	}

	return segment, nil
}

// isValidHeight returns true block is exactly 1 height higher than the current highest block in the segment
func (builder *SealingSegmentBuilder) isValidHeight(block *Block) bool {
	if builder.highest() == nil {
		return true
	}

	return block.Header.Height == builder.highest().Header.Height+1
}

// hasValidSeal returns true if highest block in the segment contains a seal for the lowest block
func (builder *SealingSegmentBuilder) hasValidSeal() bool {
	for _, seal := range builder.highest().Payload.Seals {
		if seal.BlockID == builder.lowest().ID() {
			return true
		}
	}

	return false
}

// highest returns highest block in segment
func (builder *SealingSegmentBuilder) highest() *Block {
	if len(builder.blocks) == 0 {
		return nil
	}

	return builder.blocks[len(builder.blocks)-1]
}

// lowest returns lowest block in segment
func (builder *SealingSegmentBuilder) lowest() *Block {
	return builder.blocks[0]
}

// NewSealingSegmentBuilder returns *SealingSegmentBuilder
func NewSealingSegmentBuilder(resultLookup func(resultID Identifier) (*ExecutionResult, error)) *SealingSegmentBuilder {
	return &SealingSegmentBuilder{
		resultLookup:    resultLookup,
		includedResults: make(map[Identifier]*ExecutionResult),
		blocks:          make([]*Block, 0),
		results:         make(ExecutionResultList, 0),
	}
}
