package flow

// SealingSegment is the chain segment such that the head (greatest
// height) is this snapshot's reference block and the tail (least height)
// is the most recently sealed block as of this snapshot (ie. the block
// referenced by LatestSeal). The segment is in ascending height order.
type SealingSegment struct {
	// Blocks the chain segment blocks
	Blocks []*Block

	// ExecutionResults
	ExecutionResults ExecutionResultList
	// ExecutionReceipts
	ExecutionReceipts ExecutionReceiptMetaList
}

// AddBlock appends block to Blocks
func (segment *SealingSegment) AddBlock(block *Block) {
	segment.Blocks = append(segment.Blocks, block)
}

// AddExecutionReceiptMeta adds receipt to ExecutionReceipts
func (segment *SealingSegment) AddExecutionReceiptMeta(receipt *ExecutionReceiptMeta) {
	segment.ExecutionReceipts = append(segment.ExecutionReceipts, receipt)
}

// AddExecutionResult adds result to ExecutionResults
func (segment *SealingSegment) AddExecutionResult(result *ExecutionResult) {
	segment.ExecutionResults = append(segment.ExecutionResults, result)
}

// NewSealingSegment returns SealingSegment
func NewSealingSegment() *SealingSegment {
	return &SealingSegment{
		Blocks:            make([]*Block, 0),
		ExecutionReceipts: make(ExecutionReceiptMetaList, 0),
		ExecutionResults:  make(ExecutionResultList, 0),
	}
}
