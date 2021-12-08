package flow

import (
	"fmt"
)

const (
	// expected length of a root sealing segment
	rootSegmentBlocksLen = 1

	// expected view of the block in a root sealing segment
	rootSegmentBlockView = 0
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
	ErrSegmentResultLookup       = fmt.Errorf("failed to lookup execution result")
	ErrInvalidRootSegmentView    = fmt.Errorf("invalid root sealing segment block view")
)

type SealingSegmentBuilder struct {
	resultLookup    func(resultID Identifier) (*ExecutionResult, error)
	includedResults map[Identifier]struct{}
	blocks          []*Block
	results         []*ExecutionResult
}

// AddBlock appends block to blocks
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	//sanity check: block should be 1 height higher than current highest
	if !builder.isValidHeight(block) {
		return fmt.Errorf("invalid block height (%d): %w", block.Header.Height, ErrSegmentInvalidBlockHeight)
	}

	// cache results in included results
	// they could be referenced in a future block in the segment
	for _, result := range block.Payload.Results {
		builder.includedResults[result.ID()] = struct{}{}
	}

	for _, receipt := range block.Payload.Receipts {
		if _, ok := builder.includedResults[receipt.ResultID]; !ok {
			result, err := builder.resultLookup(receipt.ResultID)
			if err != nil {
				return fmt.Errorf("%w: (%x) %v", ErrSegmentResultLookup, receipt.ResultID, err)
			}

			builder.addExecutionResult(result)
			builder.includedResults[receipt.ResultID] = struct{}{}
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
	if err := builder.validateSegment(); err != nil {
		return nil, fmt.Errorf("failed to validate sealing segment: %w", err)
	}

	return &SealingSegment{builder.blocks, builder.results}, nil
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
	lowestID := builder.lowest().ID()
	highest := builder.highest()

	// due to the fact that lowest is not always sealed
	// by highest, if highest does not have any seals check
	// that a valid ancestor does.
	if len(highest.Payload.Seals) == 0 {
		for i := len(builder.blocks) - 2; i >= 0; i-- {
			// get first block that contains any seal
			block := builder.blocks[i]
			if len(block.Payload.Seals) == 0 {
				continue
			}

			// check if block seals lowest
			for _, seal := range block.Payload.Seals {
				if seal.BlockID == lowestID {
					return true
				}
			}

			return false
		}
	}

	// highest contains seals check if seal for lowest exists
	for _, seal := range highest.Payload.Seals {
		if seal.BlockID == lowestID {
			return true
		}
	}

	return false
}

// isValidRootSegment will check that the block in the root segment has a view of 0
func (builder *SealingSegmentBuilder) isValidRootSegment() bool {
	return len(builder.blocks) == rootSegmentBlocksLen && builder.highest().Header.View == rootSegmentBlockView
}

// validateSegment will validate if builder satisfies conditions for a valid sealing segment
func (builder *SealingSegmentBuilder) validateSegment() error {
	// sealing cannot be empty
	if len(builder.blocks) < 1 {
		return fmt.Errorf("expect at least 2 blocks in a sealing segment or 1 block in the case of root segments, but actually got %v: %w", len(builder.blocks), ErrSegmentBlocksWrongLen)
	}

	// if root sealing segment skip seal sanity check
	if len(builder.blocks) == 1 {
		if !builder.isValidRootSegment() {
			return fmt.Errorf("root sealing segment block has the wrong view got (%d) expected (%d): %w", builder.highest().Header.View, rootSegmentBlockView, ErrInvalidRootSegmentView)
		}

		return nil
	}

	if !builder.hasValidSeal() {
		return fmt.Errorf("sealing segment missing seal lowest (%x) highest (%x): %w", builder.lowest().ID(), builder.highest().ID(), ErrSegmentMissingSeal)
	}

	return nil
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
		includedResults: make(map[Identifier]struct{}),
		blocks:          make([]*Block, 0),
		results:         make(ExecutionResultList, 0),
	}
}
