package flow

import (
	"fmt"
)

// SealingSegment is the chain segment such that the head (greatest
// height) is this snapshot's reference block and the tail (least height)
// is the most recently sealed block as of this snapshot (ie. the block
// referenced by LatestSeal). The segment is in ascending height order.
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
	ErrSegmentMissingSeal = fmt.Errorf("sealing segment failed sanity check: highest block in segment does not contain seal for lowest")
	ErrSegmentBlocksEmpty = fmt.Errorf("invalid sealing segment with 0 blocks")
)

type SealingSegmentBuilder struct {
	resultLookup    func(resultID Identifier) (*ExecutionResult, error)
	includedResults map[Identifier]*ExecutionResult
	blocks          []*Block
	results         []*ExecutionResult
}

// AddBlock appends block to blocks
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	for _, receipt := range block.Payload.Receipts {
		if _, ok := builder.includedResults[receipt.ResultID]; ok {
			continue
		}

		resultsByID := block.Payload.Results.Lookup()
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

	// invalid empty sealing segment blocks
	if len(segment.Blocks) == 0 {
		return nil, ErrSegmentBlocksEmpty
	}

	// segment with 1 block
	if len(segment.Blocks) == 1 {
		return segment, nil
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
