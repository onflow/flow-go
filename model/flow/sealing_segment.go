package flow

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

// AddBlock appends block to Blocks
func (segment *SealingSegment) AddBlock(block *Block) {
	segment.Blocks = append(segment.Blocks, block)
}

// AddExecutionResult adds result to ExecutionResults
func (segment *SealingSegment) AddExecutionResult(result *ExecutionResult) {
	segment.ExecutionResults = append(segment.ExecutionResults, result)
}

// NewSealingSegment returns SealingSegment
func NewSealingSegment() *SealingSegment {
	return &SealingSegment{
		Blocks:           make([]*Block, 0),
		ExecutionResults: make(ExecutionResultList, 0),
	}
}
