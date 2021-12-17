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
//
// In other words, the most recently incorporated seal as of the highest block
// references the lowest block. The highest block does not need to contain this
// seal.
//
// Example 1 - E seals A:
//   A <- B <- C <- D <- E(SA)
// The above sealing segment's last block (E) has a seal for block A, which is
// the first block of the sealing segment.
//
// Example 2 - E contains no seals, but latest seal prior to E seals A:
//   A <- B <- C <- D(SA) <- E
//
// It is guaranteed there are at least 2 blocks in a SealingSegment, unless
// the sealing segment is for a root snapshot, which contains only the root block.
// The segment is in ascending height order.
//
// In addition to storing the blocks within the sealing segment, as defined above,
// the SealingSegment structure also stores any resources which are referenced
// by blocks in the segment, but not included in the payloads of blocks within
// the segment. In particular:
// - results referenced by receipts within segment payloads
// - results referenced by seals within segment payloads
// - seals which represent the latest state commitment as of a segment block
//
type SealingSegment struct {
	// Blocks contains the chain segment blocks.
	Blocks []*Block

	// ExecutionResults contains any results which are referenced by receipts
	// or seals in the sealing segment, but not included in any segment block
	// payloads.
	//
	// Due to decoupling of execution receipts from execution results,
	// it's possible that blocks from the sealing segment will be referring to
	// execution results incorporated in blocks that aren't part of the segment.
	ExecutionResults ExecutionResultList

	// LatestSeals is a mapping from block ID to the ID of the latest seal
	// incorporated as of that block.
	LatestSeals map[Identifier]Identifier

	// FirstSeal contains the latest seal as of the first block in the segment.
	// It is needed for the `Commit` method of protocol snapshot to return the
	// sealed state, when the first block contains no seal. 
	// If the first block in the segment contains seal, then this field is `nil`
	FirstSeal *Seal
}

func (segment *SealingSegment) Highest() *Block {
	return segment.Blocks[len(segment.Blocks)-1]
}

func (segment *SealingSegment) Lowest() *Block {
	return segment.Blocks[0]
}

// Validate validates the sealing segment structure and returns an error if
// the segment isn't valid. This is done by re-building the segment from scratch,
// re-using the validation logic already present in the SealingSegmentBuilder.
func (segment *SealingSegment) Validate() error {

	// populate lookup of seals and results in the segment to satisfy builder
	seals := make(map[Identifier]*Seal)
	results := make(map[Identifier]*ExecutionResult)

	if segment.FirstSeal != nil {
		seals[segment.FirstSeal.ID()] = segment.FirstSeal
	}
	for _, result := range segment.ExecutionResults {
		results[result.ID()] = result
	}
	for _, block := range segment.Blocks {
		for _, result := range block.Payload.Results {
			results[result.ID()] = result
		}
		for _, seal := range block.Payload.Seals {
			seals[seal.ID()] = seal
		}
	}

	getResult := func(resultID Identifier) (*ExecutionResult, error) {
		result, ok := results[resultID]
		if !ok {
			return nil, fmt.Errorf("result (id=%x) not found in segment", resultID)
		}
		return result, nil
	}
	getSeal := func(blockID Identifier) (*Seal, error) {
		sealID, ok := segment.LatestSeals[blockID]
		if !ok {
			return nil, fmt.Errorf("seal for block (id=%x) not found in segment", blockID)
		}
		seal, ok := seals[sealID]
		if !ok {
			return nil, fmt.Errorf("seal (id=%x) not found in segment at block %x", sealID, blockID)
		}
		return seal, nil
	}

	builder := NewSealingSegmentBuilder(getResult, getSeal)
	for _, block := range segment.Blocks {
		err := builder.AddBlock(block)
		if err != nil {
			return fmt.Errorf("invalid segment: %w", err)
		}
	}
	_, err := builder.SealingSegment()
	if err != nil {
		return fmt.Errorf("invalid segment: %w", err)
	}
	return nil
}

var (
	ErrSegmentMissingSeal        = fmt.Errorf("sealing segment failed sanity check: missing seal referenced by segment")
	ErrSegmentBlocksWrongLen     = fmt.Errorf("sealing segment failed sanity check: non-root sealing segment must have at least 2 blocks")
	ErrSegmentInvalidBlockHeight = fmt.Errorf("sealing segment failed sanity check: blocks must be in ascending order")
	ErrSegmentResultLookup       = fmt.Errorf("failed to lookup execution result")
	ErrSegmentSealLookup         = fmt.Errorf("failed to lookup seal")
	ErrSegmentInvalidRootView    = fmt.Errorf("invalid root sealing segment block view")
)

// GetResultFunc is a getter function for results by ID.
type GetResultFunc func(resultID Identifier) (*ExecutionResult, error)

// GetSealByBlockIDFunc is a getter function for seals by block ID, returning
// the latest seals incorporated as of the given block.
type GetSealByBlockIDFunc func(blockID Identifier) (*Seal, error)

// SealingSegmentBuilder is a utility for incrementally building a sealing segment.
type SealingSegmentBuilder struct {
	// access to storage to read referenced by not included resources
	resultLookup        GetResultFunc
	sealByBLockIDLookup GetSealByBlockIDFunc
	// keep track of resources included in payloads
	includedResults map[Identifier]struct{}
	// resources to include in the sealing segment
	blocks      []*Block
	results     []*ExecutionResult
	latestSeals map[Identifier]Identifier
	firstSeal   *Seal
}

// AddBlock appends a block to the sealing segment under construction.
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	// sanity check: block should be 1 height higher than current highest
	if !builder.isValidHeight(block) {
		return fmt.Errorf("invalid block height (%d): %w", block.Header.Height, ErrSegmentInvalidBlockHeight)
	}
	blockID := block.ID()

	// a block might contain receipts or seals that refer to results that are included in blocks
	// whose height is below the first block of the segment.
	// In order to include those missing results into the segment, we construct a list of those
	// missing result IDs referenced by this block
	missingResultIDs := make(map[Identifier]struct{})

	// for the first (lowest) block, if it contains no seal, store the latest
	// seal incorporated prior to the first block
	if len(builder.blocks) == 0 {
		if len(block.Payload.Seals) == 0 {
			seal, err := builder.sealByBLockIDLookup(blockID)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrSegmentSealLookup, err)
			}
			builder.firstSeal = seal
			// add first seal result ID here, since it isn't in payload
			missingResultIDs[seal.ResultID] = struct{}{}
		}
	}

	// index the latest seal for this block
	latestSeal, err := builder.sealByBLockIDLookup(blockID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSegmentSealLookup, err)
	}
	builder.latestSeals[blockID] = latestSeal.ID()

	// cache included results and seals
	// they could be referenced in a future block in the segment
	for _, result := range block.Payload.Results {
		builder.includedResults[result.ID()] = struct{}{}
	}

	for _, receipt := range block.Payload.Receipts {
		if _, ok := builder.includedResults[receipt.ResultID]; !ok {
			missingResultIDs[receipt.ResultID] = struct{}{}
		}
	}
	for _, seal := range block.Payload.Seals {
		if _, ok := builder.includedResults[seal.ResultID]; !ok {
			missingResultIDs[seal.ResultID] = struct{}{}
		}
	}

	// add the missing results
	for resultID := range missingResultIDs {
		result, err := builder.resultLookup(resultID)

		if err != nil {
			return fmt.Errorf("%w: (%x) %v", ErrSegmentResultLookup, resultID, err)
		}
		builder.addExecutionResult(result)
		builder.includedResults[resultID] = struct{}{}
	}

	builder.blocks = append(builder.blocks, block)
	return nil
}

// AddExecutionResult adds result to executionResults
func (builder *SealingSegmentBuilder) addExecutionResult(result *ExecutionResult) {
	builder.results = append(builder.results, result)
}

// SealingSegment completes building the sealing segment, validating the segment
// constructed so far, and returning it as a SealingSegment if it is valid.
func (builder *SealingSegmentBuilder) SealingSegment() (*SealingSegment, error) {
	if err := builder.validateSegment(); err != nil {
		return nil, fmt.Errorf("failed to validate sealing segment: %w", err)
	}

	return &SealingSegment{
		Blocks:           builder.blocks,
		ExecutionResults: builder.results,
		LatestSeals:      builder.latestSeals,
		FirstSeal:        builder.firstSeal,
	}, nil
}

// isValidHeight returns true block is exactly 1 height higher than the current highest block in the segment
func (builder *SealingSegmentBuilder) isValidHeight(block *Block) bool {
	if builder.highest() == nil {
		return true
	}

	return block.Header.Height == builder.highest().Header.Height+1
}

// hasValidSeal returns true if the latest seal as of highest is for lowest.
func (builder *SealingSegmentBuilder) hasValidSeal() bool {
	lowestID := builder.lowest().ID()

	// due to the fact that lowest is not always sealed by highest,
	// if highest does not have any seals check that a valid ancestor does.
	for i := len(builder.blocks) - 1; i >= 0; i-- {

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

	return false
}

// isValidRootSegment will check that the block in the root segment has a view of 0.
func (builder *SealingSegmentBuilder) isValidRootSegment() bool {
	return len(builder.blocks) == rootSegmentBlocksLen && builder.highest().Header.View == rootSegmentBlockView
}

// validateSegment will validate if builder satisfies conditions for a valid sealing segment.
func (builder *SealingSegmentBuilder) validateSegment() error {
	// sealing cannot be empty
	if len(builder.blocks) < 1 {
		return fmt.Errorf("expect at least 2 blocks in a sealing segment or 1 block in the case of root segments, but actually got %v: %w", len(builder.blocks), ErrSegmentBlocksWrongLen)
	}

	// if root sealing segment skip seal sanity check
	if len(builder.blocks) == 1 {
		if !builder.isValidRootSegment() {
			return fmt.Errorf("root sealing segment block has the wrong view got (%d) expected (%d): %w", builder.highest().Header.View, rootSegmentBlockView, ErrSegmentInvalidRootView)
		}

		return nil
	}

	if !builder.hasValidSeal() {
		return fmt.Errorf("sealing segment missing seal lowest (%x) highest (%x): %w", builder.lowest().ID(), builder.highest().ID(), ErrSegmentMissingSeal)
	}

	return nil
}

// highest returns the highest block in segment.
func (builder *SealingSegmentBuilder) highest() *Block {
	if len(builder.blocks) == 0 {
		return nil
	}

	return builder.blocks[len(builder.blocks)-1]
}

// lowest returns the lowest block in segment.
func (builder *SealingSegmentBuilder) lowest() *Block {
	return builder.blocks[0]
}

// NewSealingSegmentBuilder returns *SealingSegmentBuilder
func NewSealingSegmentBuilder(resultLookup GetResultFunc, sealLookup GetSealByBlockIDFunc) *SealingSegmentBuilder {
	return &SealingSegmentBuilder{
		resultLookup:        resultLookup,
		sealByBLockIDLookup: sealLookup,
		includedResults:     make(map[Identifier]struct{}),
		latestSeals:         make(map[Identifier]Identifier),
		blocks:              make([]*Block, 0),
		results:             make(ExecutionResultList, 0),
	}
}
