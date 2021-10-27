package flow

// SealingSegment is the chain segment such that the head (greatest
// height) is this snapshot's reference block and the tail (least height)
// is the most recently sealed block as of this snapshot (ie. the block
// referenced by LatestSeal). The segment is in ascending height order.
type SealingSegment struct {
	// Blocks the chain segment blocks
	Blocks []*Block

	// ExecutionResults execution results referenced in segment block receipts
	// but missing from the referencing block
	//ExecutionResults map[Identifier]*ExecutionResult
	ExecutionResults map[Identifier]*ExecutionResult
}

// AddBlock appends block to Blocks
func (segment *SealingSegment) AddBlock(block *Block) {
	segment.Blocks = append(segment.Blocks, block)
}

// AddExecutionResult adds execution result to ExecutionResults using id as the key
func (segment *SealingSegment) AddExecutionResult(result *ExecutionResult) {
	segment.ExecutionResults[result.ID()] = result
}

// ContainsExecutionResult returns true if result is in ExecutionResults
func (segment *SealingSegment) ContainsExecutionResult(resultID Identifier) bool {
	_, ok := segment.ExecutionResults[resultID]
	return ok
}

// NewSealingSegment returns SealingSegment
func NewSealingSegment() *SealingSegment {
	return &SealingSegment{
		Blocks:           make([]*Block, 0),
		ExecutionResults: make(map[Identifier]*ExecutionResult),
	}
}
