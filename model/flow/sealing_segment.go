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
	ErrSegmentMissingSeal    = fmt.Errorf("sealing segment failed sanity check: highest block in segment does not contain seal for lowest")
	ErrSegmentBlocksWrongLen = fmt.Errorf("sealing segment is required to have atleast 2 blocks")
)

type SealingSegmentBuilder struct {
	resultLookup    func(resultID Identifier) (*ExecutionResult, error)
	includedResults map[Identifier]*ExecutionResult
	blocks          []*Block
	results         []*ExecutionResult
}

// AddBlock appends block to blocks
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	resultsByID := block.Payload.Results.Lookup()
	for _, receipt := range block.Payload.Receipts {
		if _, ok := builder.includedResults[receipt.ResultID]; ok {
			continue
		}

		if _, ok := resultsByID[receipt.ResultID]; !ok {
			result, err := builder.resultLookup(receipt.ResultID)
			if err != nil {
				return fmt.Errorf("could not get execution result (%s): %w", receipt.ResultID, err)
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

	for _, seal := range segment.Highest().Payload.Seals {
		if seal.BlockID == segment.Lowest().ID() {
			return segment, nil
		}
	}

	return nil, ErrSegmentMissingSeal
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
