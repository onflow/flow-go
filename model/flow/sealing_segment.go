package flow

// SealingSegment is the chain segment such that the head (greatest
// height) is this snapshot's reference block and the tail (least height)
// is the most recently sealed block as of this snapshot (ie. the block
// referenced by LatestSeal). The segment is in ascending height order.
type SealingSegment struct {
	// Blocks the chain segment blocks
	Blocks []*Block

	// ExecutionReceipts map of ExecutionResult.ID -> full ExecutionReceipt
	ExecutionReceipts map[Identifier]*ExecutionReceipt
}

// AddBlock appends block to Blocks
func (segment *SealingSegment) AddBlock(block *Block) {
	segment.Blocks = append(segment.Blocks, block)
}

// AddFullExecutionReceipt adds full execution receipt to ExecutionReceipts using execution result id as the key
func (segment *SealingSegment) AddFullExecutionReceipt(receipt *ExecutionReceipt) {
	segment.ExecutionReceipts[receipt.ExecutionResult.ID()] = receipt
}

// ContainsExecutionResult returns true if result is in ExecutionReceipts
func (segment *SealingSegment) ContainsExecutionResult(resultID Identifier) bool {
	_, ok := segment.ExecutionReceipts[resultID]
	return ok
}

// NewSealingSegment returns SealingSegment
func NewSealingSegment() *SealingSegment {
	return &SealingSegment{
		Blocks:            make([]*Block, 0),
		ExecutionReceipts: make(map[Identifier]*ExecutionReceipt),
	}
}
