package flow

import (
	"fmt"

	"golang.org/x/exp/slices"
)

// SealingSegment is the chain segment such that the last block (greatest height)
// is this snapshot's reference block and the first (least height) is the most
// recently sealed block as of this snapshot (ie. the block referenced by LatestSeal).
//
// In other words, the most recently incorporated seal as of the highest block
// references the lowest block. The highest block does not need to contain this seal.
//
// Example 1 - E seals A:
//
//	A <- B <- C <- D <- E(SA)
//
// The above sealing segment's last block (E) has a seal for block A, which is
// the first block of the sealing segment.
//
// Example 2 - E contains no seals, but latest seal prior to E seals A:
//
//	A <- B <- C <- D(SA) <- E
//
// Example 3 - E contains multiple seals
//
//	B <- C <- D <- E(SA, SB)
//
// MINIMALITY REQUIREMENT:
// Note that block B is the highest sealed block as of E. Therefore, the
// sealing segment's lowest block must be B. Essentially, this is a minimality
// requirement for the history: it shouldn't be longer than necessary. So
// extending the chain segment above to A <- B <- C <- D <- E(SA, SB) would
// _not_ yield a valid SealingSegment.
//
// ROOT SEALING SEGMENTS:
// Root sealing segments are sealing segments which contain the root block:
// * the root block is a self-sealing block with an empty payload
// * the root block must be the first block (least height) in the segment
// * no blocks in the segment may contain any seals (by the minimality requirement)
// * it is possible (but not necessary) for root sealing segments to contain only the root block
//
// Example 1 - one self-sealing root block
//
//	ROOT
//
// The above sealing segment is the form of sealing segments within root snapshots,
// for example those snapshots used to bootstrap a new network, or spork.
//
// Example 2 - one self-sealing root block followed by any number of seal-less blocks
//
//	ROOT <- A <- B
//
// All non-root sealing segments contain more than one block.
// Sealing segments are in ascending height order.
//
// In addition to storing the blocks within the sealing segment, as defined above,
// the SealingSegment structure also stores any resources which are referenced
// by blocks in the segment, but not included in the payloads of blocks within
// the segment. In particular:
// * results referenced by receipts within segment payloads
// * results referenced by seals within segment payloads
// * seals which represent the latest state commitment as of a segment block
type SealingSegment struct {
	// Blocks contain the chain segment blocks in ascending height order.
	Blocks []*Block

	// ExtraBlocks contain the chain extra blocks in ascending height order. These blocks
	// are connecting to `Blocks[0]` (the lowest block of sealing segment). Formally, let `l`
	// be the length of `ExtraBlocks`, then ExtraBlocks[l-1] is the _parent_ of `Blocks[0]`.
	// These extra blocks are included in order to ensure that a newly bootstrapped node
	// knows about all entities which might be referenced by blocks which extend from
	// the sealing segment.
	// ExtraBlocks are stored separately from Blocks, because not all node roles need
	// the same amount of history. Blocks represents the minimal possible required history;
	// ExtraBlocks represents additional role-required history.
	ExtraBlocks []*Block

	// ExecutionResults contain any results which are referenced by receipts
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

// AllBlocks returns all blocks within the sealing segment, including extra blocks, in ascending height order.
func (segment *SealingSegment) AllBlocks() []*Block {
	return append(segment.ExtraBlocks, segment.Blocks...)
}

// FinalizedSeal returns the seal that seals the lowest block.
// Per specification, this seal must be included in a SealingSegment.
// The SealingSegment must be validated.
// No errors are expected during normal operation.
func (segment *SealingSegment) FinalizedSeal() (*Seal, error) {
	if isRootSegment(segment.LatestSeals) {
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

// Validate validates the sealing segment structure and returns an error if
// the segment isn't valid. This is done by re-building the segment from scratch,
// re-using the validation logic already present in the SealingSegmentBuilder.
// The node logic requires a valid sealing segment to bootstrap.
// No errors are expected during normal operation.
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
	// extra blocks should be added in reverse order, starting from the highest one since they are sorted
	// in ascending order.
	for i := len(segment.ExtraBlocks) - 1; i >= 0; i-- {
		block := segment.ExtraBlocks[i]
		err := builder.AddExtraBlock(block)
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
)

// GetResultFunc is a getter function for results by ID.
// No errors are expected during normal operation.
type GetResultFunc func(resultID Identifier) (*ExecutionResult, error)

// GetSealByBlockIDFunc is a getter function for seals by block ID, returning
// the latest seals incorporated as of the given block.
// No errors are expected during normal operation.
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
	// extraBlocks included in sealing segment, must connect to the lowest block of segment
	// stored in descending order for simpler population logic
	extraBlocks []*Block
}

// AddBlock appends a block to the sealing segment under construction.
// No errors are expected during normal operation.
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	// sanity check: all blocks have to be added before adding extra blocks
	if len(builder.extraBlocks) > 0 {
		return fmt.Errorf("cannot add sealing segment block after extra block is added")
	}

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

// AddExtraBlock appends an extra block to sealing segment under construction.
// Extra blocks needs to be added in descending order and the first block must connect to the lowest block
// of sealing segment, this way they form a continuous chain.
// No errors are expected during normal operation.
func (builder *SealingSegmentBuilder) AddExtraBlock(block *Block) error {
	if len(builder.extraBlocks) == 0 {
		if len(builder.blocks) == 0 {
			return fmt.Errorf("cannot add extra blocks before adding lowest sealing segment block")
		}
		// first extra block has to match the lowest block of sealing segment
		if (block.Header.Height + 1) != builder.lowest().Header.Height {
			return fmt.Errorf("invalid extra block height (%d), doesn't connect to sealing segment: %w", block.Header.Height, ErrSegmentInvalidBlockHeight)
		}
	} else if (block.Header.Height + 1) != builder.extraBlocks[len(builder.extraBlocks)-1].Header.Height {
		return fmt.Errorf("invalid extra block height (%d), doesn't connect to last extra block: %w", block.Header.Height, ErrSegmentInvalidBlockHeight)
	}

	builder.extraBlocks = append(builder.extraBlocks, block)
	return nil
}

// AddExecutionResult adds result to executionResults
func (builder *SealingSegmentBuilder) addExecutionResult(result *ExecutionResult) {
	builder.results = append(builder.results, result)
}

// SealingSegment completes building the sealing segment, validating the segment
// constructed so far, and returning it as a SealingSegment if it is valid.
//
// All errors indicate the SealingSegmentBuilder internal state does not represent
// a valid sealing segment.
// No errors are expected during normal operation.
func (builder *SealingSegmentBuilder) SealingSegment() (*SealingSegment, error) {
	if err := builder.validateSegment(); err != nil {
		return nil, fmt.Errorf("failed to validate sealing segment: %w", err)
	}

	// SealingSegment must store extra blocks in ascending order, builder stores them in descending.
	// Apply a sort to reverse the slice and use correct ordering.
	slices.SortFunc(builder.extraBlocks, func(lhs, rhs *Block) bool {
		return lhs.Header.Height < rhs.Header.Height
	})

	return &SealingSegment{
		Blocks:           builder.blocks,
		ExtraBlocks:      builder.extraBlocks,
		ExecutionResults: builder.results,
		LatestSeals:      builder.latestSeals,
		FirstSeal:        builder.firstSeal,
	}, nil
}

// isValidHeight returns true iff block is exactly 1 height higher than the current highest block in the segment.
func (builder *SealingSegmentBuilder) isValidHeight(block *Block) bool {
	if builder.highest() == nil {
		return true
	}

	return block.Header.Height == builder.highest().Header.Height+1
}

// validateRootSegment will check that the current builder state represents a valid
// root sealing segment. In particular:
// * the root block must be the first block (least height) in the segment
// * no blocks in the segment may contain any seals (by the minimality requirement)
//
// All errors indicate an invalid root segment, and either a bug in SealingSegmentBuilder
// or a corrupted underlying protocol state.
// No errors are expected during normal operation.
func (builder *SealingSegmentBuilder) validateRootSegment() error {
	if len(builder.blocks) == 0 {
		return fmt.Errorf("root segment must have at least 1 block")
	}
	if len(builder.extraBlocks) > 0 {
		return fmt.Errorf("root segment cannot have extra blocks")
	}
	if builder.lowest().Header.View != 0 {
		return fmt.Errorf("root block has unexpected view (%d != 0)", builder.lowest().Header.View)
	}
	if len(builder.results) != 1 {
		return fmt.Errorf("expected %d results, got %d", 1, len(builder.results))
	}
	if builder.firstSeal == nil {
		return fmt.Errorf("firstSeal must not be nil for root segment")
	}
	if builder.results[0].BlockID != builder.lowest().ID() {
		return fmt.Errorf("result (block_id=%x) is not for root block (id=%x)", builder.results[0].BlockID, builder.lowest().ID())
	}
	if builder.results[0].ID() != builder.firstSeal.ResultID {
		return fmt.Errorf("firstSeal (result_id=%x) is not for root result (id=%x)", builder.firstSeal.ResultID, builder.results[0].ID())
	}
	if builder.results[0].BlockID != builder.firstSeal.BlockID {
		return fmt.Errorf("root seal (block_id=%x) references different block than root result (block_id=%x)", builder.firstSeal.BlockID, builder.results[0].BlockID)
	}
	for _, block := range builder.blocks {
		if len(block.Payload.Seals) > 0 {
			return fmt.Errorf("root segment cannot contain blocks with seals (minimality requirement) - block (height=%d,id=%x) has %d seals",
				block.Header.Height, block.ID(), len(block.Payload.Seals))
		}
	}
	return nil
}

// validateSegment will validate if builder satisfies conditions for a valid sealing segment.
// No errors are expected during normal operation.
func (builder *SealingSegmentBuilder) validateSegment() error {
	// sealing cannot be empty
	if len(builder.blocks) == 0 {
		return fmt.Errorf("expect at least 2 blocks in a sealing segment or 1 block in the case of root segments, but got an empty sealing segment: %w", ErrSegmentBlocksWrongLen)
	}

	if len(builder.extraBlocks) > 0 {
		if builder.extraBlocks[0].Header.Height+1 != builder.lowest().Header.Height {
			return fmt.Errorf("extra blocks don't connect to lowest block in segment")
		}
	}

	// if root sealing segment, use different validation
	if isRootSegment(builder.latestSeals) {
		err := builder.validateRootSegment()
		if err != nil {
			return fmt.Errorf("invalid root segment: %w", err)
		}
		return nil
	}

	// validate the latest seal is for the lowest block
	_, err := findLatestSealForLowestBlock(builder.blocks, builder.latestSeals)
	if err != nil {
		return fmt.Errorf("sealing segment missing seal (lowest block id: %x) (highest block id: %x) %v: %w", builder.lowest().ID(), builder.highest().ID(), err, ErrSegmentMissingSeal)
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
		extraBlocks:         make([]*Block, 0),
		results:             make(ExecutionResultList, 0),
	}
}

// findLatestSealForLowestBlock finds the seal for the lowest block.
// As a sanity check, the method confirms that this seal is the latest seal as of the highest block.
// In other words, this function checks that the sealing segment's history is minimal.
// Inputs:
//   - `blocks` is the continuous sequence of blocks that form the sealing segment
//   - `latestSeals` holds for each block the identifier of the latest seal included in the fork as of this block
//
// CAUTION: this method is only applicable for non-root sealing segments, where at least one block
// was sealed after the root block.
// Examples:
//
//	A <- B <- C <- D(seal_A)               ==> valid
//	A <- B <- C <- D(seal_A) <- E()        ==> valid
//	A <- B <- C <- D(seal_A,seal_B)        ==> invalid, because latest seal is B, but lowest block is A
//	A <- B <- C <- D(seal_X,seal_A)        ==> valid, because it's OK for block X to be unknown
//	A <- B <- C <- D(seal_A) <- E(seal_B)  ==> invalid, because latest seal is B, but lowest block is A
//	A(seal_A)                              ==> invalid, because this is impossible for non-root sealing segments
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

		// the latest seal must be found in a block that has a seal when traversing blocks
		// backwards from higher height to lower height.
		// otherwise, the sealing segment is invalid
		if len(block.Payload.Seals) > 0 {
			return nil, fmt.Errorf("invalid segment: segment's last block contain seal %v, but doesn't match latestSealID: %v",
				block.Payload.Seals[0].ID(), latestSealID)
		}
	}

	return nil, fmt.Errorf("invalid segment: seal %v not found", latestSealID)
}

// isRootSegment returns true if the input latestSeals map represents a root segment.
// The implementation makes use of the fact that root sealing segments uniquely
// have the same latest seal, for all blocks in the segment.
func isRootSegment(latestSeals map[Identifier]Identifier) bool {
	var rootSealID Identifier
	// set root seal ID to the latest seal value for any block in the segment
	for _, sealID := range latestSeals {
		rootSealID = sealID
		break
	}
	// then, verify all other blocks have the same latest seal
	for _, sealID := range latestSeals {
		if sealID != rootSealID {
			return false
		}
	}
	return true
}
