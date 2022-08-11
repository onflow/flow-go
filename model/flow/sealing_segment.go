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
// Example 3 - E contains multiple seals
//    B <- C <- D <- E(SA, SB)
// Note that block B is the highest sealed block as of E. Therefore, the
// sealing segment's lowest block must be B. Essentially, this is a minimality
// requirement for the history: it shouldn't be longer than necessary. So
// extending the chain segment above to A <- B <- C <- D <- E(SA, SB) would
// _not_ yield a valid SealingSegment.
//
// Root sealing segments contain only one self-sealing block. All other sealing
// segments contain multiple blocks.
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
	// incorporated as of that block. Note: we store the seals' IDs here
	// (instead of the full seals), because the seals for all blocks, except for
	// the lowest one, are contained in the blocks of the sealing segment.
	LatestSeals map[Identifier]Identifier

	// FirstSeal contains the latest seal as of the first block in the segment.
	// Per convention, this field holds a seal that was included _prior_ to the
	// first block of the sealing segment. If the first block in the segment
	// contains a seal, then this field is `nil`.
	// This information is needed for the `Commit` method of protocol snapshot
	// to return the sealed state, when the first block contains no seal.
	FirstSeal *Seal
}

func (segment *SealingSegment) Highest() *Block {
	return segment.Blocks[len(segment.Blocks)-1]
}

func (segment *SealingSegment) Lowest() *Block {
	return segment.Blocks[0]
}

// FinalizedSeal returns the seal that seals the lowest block.
// Per specification, this seal must be included in a SealingSegment.
// Under normal operations (where only a validated SealingSegment
// is processed), no error returns are expected.
func (segment *SealingSegment) FinalizedSeal() (*Seal, error) {
	if isRootSegment(segment) {
		return segment.FirstSeal, nil
	}

	seal, err := findLatestSealForLowestBlock(segment.Blocks, segment.LatestSeals)
	if err != nil {
		return nil, err
	}

	// sanity check
	if seal.BlockID != segment.Lowest().ID() {
		return nil, fmt.Errorf("finalized seal should seal the lowest block %v, but actually is to seal %v",
			segment.Lowest().ID(), seal.BlockID)
	}
	return seal, nil
}

func isRootSegment(segment *SealingSegment) bool {
	return len(segment.Blocks) == rootSegmentBlocksLen
}

// Validate validates the sealing segment structure and returns an error if
// the segment isn't valid. This is done by re-building the segment from scratch,
// re-using the validation logic already present in the SealingSegmentBuilder.
// The node logic requires a valid sealing segment to bootstrap. There are no
// errors expected during normal operations.
func (segment *SealingSegment) Validate() error {

	// populate lookup of seals and results in the segment to satisfy builder
	seals := make(map[Identifier]*Seal)
	results := segment.ExecutionResults.Lookup()

	if segment.FirstSeal != nil {
		seals[segment.FirstSeal.ID()] = segment.FirstSeal
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
	sealByBlockIDLookup GetSealByBlockIDFunc
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
			seal, err := builder.sealByBlockIDLookup(blockID)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrSegmentSealLookup, err)
			}
			builder.firstSeal = seal
			// add first seal result ID here, since it isn't in payload
			missingResultIDs[seal.ResultID] = struct{}{}
		}
	}

	// index the latest seal for this block
	latestSeal, err := builder.sealByBlockIDLookup(blockID)
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

// isValidHeight returns true iff block is exactly 1 height higher than the current highest block in the segment
func (builder *SealingSegmentBuilder) isValidHeight(block *Block) bool {
	if builder.highest() == nil {
		return true
	}

	return block.Header.Height == builder.highest().Header.Height+1
}

// isValidRootSegment will check that the block in the root segment has a view of 0.
func (builder *SealingSegmentBuilder) isValidRootSegment() bool {
	return len(builder.blocks) == rootSegmentBlocksLen &&
		builder.highest().Header.View == rootSegmentBlockView &&
		len(builder.results) == rootSegmentBlocksLen && // root segment has only 1 result
		builder.firstSeal != nil && // first seal is the root seal itself and must exist
		builder.results[0].BlockID == builder.blocks[0].ID() && // root result matches the root block
		builder.results[0].ID() == builder.firstSeal.ResultID && // root seal matches the root result
		builder.results[0].BlockID == builder.firstSeal.BlockID // root seal seals the root block
}

// validateSegment will validate if builder satisfies conditions for a valid sealing segment.
func (builder *SealingSegmentBuilder) validateSegment() error {
	// sealing cannot be empty
	if len(builder.blocks) == 0 {
		return fmt.Errorf("expect at least 2 blocks in a sealing segment or 1 block in the case of root segments, but got an empty sealing segment: %w", ErrSegmentBlocksWrongLen)
	}

	// if root sealing segment skip seal sanity check
	if len(builder.blocks) == 1 {
		if !builder.isValidRootSegment() {
			return fmt.Errorf("root sealing segment block has the wrong view got (%d) expected (%d): %w", builder.highest().Header.View, rootSegmentBlockView, ErrSegmentInvalidRootView)
		}

		return nil
	}

	// validate the latest seal is for the lowest block
	_, err := findLatestSealForLowestBlock(builder.blocks, builder.latestSeals)
	if err != nil {
		return fmt.Errorf("sealing segment missing seal lowest (%x) highest (%x) %v: %w", builder.lowest().ID(), builder.highest().ID(), err, ErrSegmentMissingSeal)
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
		sealByBlockIDLookup: sealLookup,
		includedResults:     make(map[Identifier]struct{}),
		latestSeals:         make(map[Identifier]Identifier),
		blocks:              make([]*Block, 0),
		results:             make(ExecutionResultList, 0),
	}
}

// findLatestSealForLowestBlock finds the latest seal as of the highest block.
// As a sanity check, the method confirms that the latest seal is for lowest block, i.e.
// the sealing segment's history is minimal.
// Inputs:
//  * `blocks` is the continuous sequence of blocks that form the sealing segment
//  * `latestSeals` holds for each block the identifier of the latest seal included in the fork as of this block
// CAUTION: this method is only applicable for non-root sealing segments, where at least one block
// was sealed after the root block.
// Examples:
//  A <- B <- C <- D(seal_A)               ==> valid
//  A <- B <- C <- D(seal_A) <- E()        ==> valid
//  A <- B <- C <- D(seal_A,seal_B)        ==> invalid, because latest seal is B, but lowest block is A
//  A <- B <- C <- D(seal_X,seal_A)        ==> valid, because it's OK for block X to be unknown
//  A <- B <- C <- D(seal_A) <- E(seal_B)  ==> invalid, because latest seal is B, but lowest block is A
//  A(seal_A)                              ==> invalid, because this is impossible for non-root sealing segments
//
// The node logic requires a valid sealing segment to bootstrap. There are no
// errors expected during normal operations.
func findLatestSealForLowestBlock(blocks []*Block, latestSeals map[Identifier]Identifier) (*Seal, error) {
	lowestBlockID := blocks[0].ID()
	highestBlockID := blocks[len(blocks)-1].ID()

	// get the ID of the latest seal for highest block
	latestSealID := latestSeals[highestBlockID]

	// find the seal within the block payloads
	for i := len(blocks) - 1; i >= 0; i-- {
		block := blocks[i]
		// look for latestSealID in the payload
		for _, seal := range block.Payload.Seals {
			// if we found the latest seal, confirm it seals lowest
			if seal.ID() == latestSealID {
				if seal.BlockID == lowestBlockID {
					return seal, nil
				}
				return nil, fmt.Errorf("invalid segment: segment contain seal for block %v, but doesn't match lowest block %v",
					seal.BlockID, lowestBlockID)
			}
		}

		// the latest seal must be found in a block that has Seal when traversing blocks
		// backwards from higher height to lower height.
		// otherwise, the sealing segment is invalid
		if len(block.Payload.Seals) > 0 {
			return nil, fmt.Errorf("invalid segment: segment's last block contain seal %v, but doesn't match latestSealID: %v",
				block.Payload.Seals[0].ID(), latestSealID)
		}
	}

	return nil, fmt.Errorf("invalid segment: seal %v not found", latestSealID)
}
