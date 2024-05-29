package flow

import (
	"errors"
	"fmt"

	"golang.org/x/exp/slices"
)

// SealingSegment is a continuous segment of recently finalized blocks that contains enough history
// for a new node to execute its business logic normally. It is part of the data need to initialize
// a new node to join the network.
// DETAILED SPECIFICATION: ./sealing_segment.md
//
//	├═══════════┤  ├───────────────────────┤
//	 ExtraBlocks   ^          Blocks       ^
//	               B                      head
//
// Lets denote the highest block in the sealing segment as `head`. Per convention, `head` must be a
// finalized block. Consider the chain of blocks leading up to `head` (included). The highest block
// in chain leading up to `head` that is sealed, we denote as B.
// In other words, head is the last finalized block, and B is the last sealed block,
// block at height (B.Height + 1) is not sealed.
type SealingSegment struct {
	// Blocks contain the chain `B <- ... <- Head` in ascending height order.
	// Formally, Blocks contains exactly (not more!) the history to satisfy condition
	// (see sealing_segment.md for details):
	//   (i) The highest sealed block as of `head` needs to be included in the sealing segment.
	//       This is relevant if `head` does not contain any seals.
	Blocks []*Block

	// ExtraBlocks [optional] holds ancestors of `Blocks` in ascending height order.
	// Formally, ExtraBlocks contains at least the additional history to satisfy conditions
	// (see sealing_segment.md for details):
	//  (ii) All blocks that are sealed by `head`. This is relevant if `head` contains _multiple_ seals.
	// (iii) The sealing segment holds the history of all non-expired collection guarantees, i.e.
	//       limitHeight := max(blockSealedAtHead.Height - flow.DefaultTransactionExpiry, SporkRootBlockHeight)
	//       where blockSealedAtHead is the block sealed by `head` block.
	// (Potentially longer history is permitted)
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

	// ProtocolStateEntries contains every protocol state entry committed to
	// by any block in the SealingSegment (including ExtraBlocks).
	ProtocolStateEntries map[Identifier]*ProtocolStateEntryWrapper
}

// ProtocolStateEntryWrapper is a wrapper coupling two data sources.
// Conceptually, the SealingSegment stores one Protocol State Entry (aka `KVStoreEntry`)
// per unique ProtocolStateID field within the segment's blocks.
// Currently, although epoch data is conceptually a part of the protocol data entry associated
// with each block, it is stored separately as a matter of technical debt (only a hash commitment
// `RichProtocolStateEntry.ID()` is stored within the `KVStoreEntry`.
//
// Deprecated: avoid using this in new code; this is a temporary measure until epoch data is moved into protocol KV store
// TODO: move epoch data into the KVStore as part of a future upgrade
type ProtocolStateEntryWrapper struct {
	KVStore    PSKeyValueStoreData
	EpochEntry *RichProtocolStateEntry
}

// Highest is the highest block in the sealing segment and the reference block from snapshot that was
// used to produce this sealing segment.
func (segment *SealingSegment) Highest() *Block {
	return segment.Blocks[len(segment.Blocks)-1]
}

// Finalized returns the last finalized block, which is an alias of Highest
func (segment *SealingSegment) Finalized() *Block {
	return segment.Highest()
}

// Sealed returns the most recently sealed block based on head of sealing segment(highest block).
func (segment *SealingSegment) Sealed() *Block {
	return segment.Blocks[0]
}

// AllBlocks returns all blocks within the sealing segment, including extra blocks, in ascending height order.
func (segment *SealingSegment) AllBlocks() []*Block {
	return append(segment.ExtraBlocks, segment.Blocks...)
}

// IsSporkRoot returns true if this SealingSegment represents a spork root snapshot.
// The Flow protocol explicitly defines a spork root block (incl. also the network's
// genesis block) to be finalized and sealed and to have a specific execution state
// commitment attached. Mathematically, this is a protocol axiom, as no block exists
// that contains an execution result or seal for the spork root block (nor any children
// at the time of the spork that could possibly finalize the root block).
// Therefore, a spork root block is a degenerate sealing segment with a length of 1.
func (segment *SealingSegment) IsSporkRoot() bool {
	return len(segment.Blocks) == 1
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
	if seal.BlockID != segment.Sealed().ID() {
		return nil, fmt.Errorf("finalized seal should seal the lowest block %v, but actually is to seal %v",
			segment.Sealed().ID(), seal.BlockID)
	}
	return seal, nil
}

// LatestProtocolStateEntry returns the Protocol State entry corresponding to
// the highest block in the sealing segment. This represents the Dynamic Protocol State
// after applying all state changes sealed in `SealingSegment.Highest().Payload`.
// Caution: `segment` must be a valid SealingSegment.
func (segment *SealingSegment) LatestProtocolStateEntry() *ProtocolStateEntryWrapper {
	highest := segment.Highest()
	return segment.ProtocolStateEntries[highest.Payload.ProtocolStateID]
}

// Validate validates the sealing segment structure and returns an error if
// the segment isn't valid. This is done by re-building the segment from scratch,
// re-using the validation logic already present in the SealingSegmentBuilder.
// The node logic requires a valid sealing segment to bootstrap.
// Errors expected during normal operation:
//   - InvalidSealingSegmentError if `segment` is an invalid SealingSegment.
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
	getProtocolStateEntry := func(protocolStateID Identifier) (*ProtocolStateEntryWrapper, error) {
		entry, ok := segment.ProtocolStateEntries[protocolStateID]
		if !ok {
			return nil, fmt.Errorf("protocol state (id=%x) not found in segment", protocolStateID)
		}
		return entry, nil
	}

	builder := NewSealingSegmentBuilder(getResult, getSeal, getProtocolStateEntry)
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

// InvalidSealingSegmentError is returned either when building or validating a SealingSegment,
// when the segment is found to be invalid, or when attempting to add an entity to a segment
// under construction would cause the resulting SealingSegment to become invalid.
type InvalidSealingSegmentError struct {
	err error
}

func NewInvalidSealingSegmentError(msg string, args ...any) InvalidSealingSegmentError {
	return InvalidSealingSegmentError{
		err: fmt.Errorf(msg, args...),
	}
}

func (err InvalidSealingSegmentError) Error() string {
	return err.err.Error()
}

func (err InvalidSealingSegmentError) Unwrap() error {
	return err.err
}

// IsInvalidSealingSegmentError returns true if err is or wraps an instance of InvalidSealingSegmentError.
func IsInvalidSealingSegmentError(err error) bool {
	var invalidSealingSegmentError InvalidSealingSegmentError
	return errors.As(err, &invalidSealingSegmentError)
}

// GetResultFunc is a getter function for results by ID.
// No errors are expected during normal operation.
type GetResultFunc func(resultID Identifier) (*ExecutionResult, error)

// GetSealByBlockIDFunc is a getter function for seals by block ID, returning
// the latest seals incorporated as of the given block.
// No errors are expected during normal operation.
type GetSealByBlockIDFunc func(blockID Identifier) (*Seal, error)

// GetProtocolStateEntryFunc  is a getter function for protocol state entries
// No errors are expected during normal operation.
type GetProtocolStateEntryFunc func(protocolStateID Identifier) (*ProtocolStateEntryWrapper, error)

// SealingSegmentBuilder is a utility for incrementally building a sealing segment.
type SealingSegmentBuilder struct {
	// access to storage to read referenced by not included resources
	resultLookup        GetResultFunc
	sealByBlockIDLookup GetSealByBlockIDFunc
	protocolStateLookup GetProtocolStateEntryFunc
	// keep track of resources included in payloads
	includedResults map[Identifier]struct{}
	// resources to include in the sealing segment
	blocks               []*Block
	results              []*ExecutionResult
	latestSeals          map[Identifier]Identifier
	protocolStateEntries map[Identifier]*ProtocolStateEntryWrapper
	firstSeal            *Seal
	// extraBlocks included in sealing segment, must connect to the lowest block of segment
	// stored in descending order for simpler population logic
	extraBlocks []*Block
}

// AddBlock appends a block to the sealing segment under construction.
// Errors expected during normal operation:
//   - InvalidSealingSegmentError if the added block would cause an invalid resulting segment
func (builder *SealingSegmentBuilder) AddBlock(block *Block) error {
	// sanity check: all blocks have to be added before adding extra blocks
	if len(builder.extraBlocks) > 0 {
		return fmt.Errorf("cannot add sealing segment block after extra block is added")
	}

	// sanity check: block should be 1 height higher than current highest
	if !builder.isValidHeight(block) {
		return NewInvalidSealingSegmentError("invalid block height (%d)", block.Header.Height)
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
				return fmt.Errorf("could not look up seal: %w", err)
			}
			builder.firstSeal = seal
			// add first seal result ID here, since it isn't in payload
			missingResultIDs[seal.ResultID] = struct{}{}
		}
	}

	// index the latest seal for this block
	latestSeal, err := builder.sealByBlockIDLookup(blockID)
	if err != nil {
		return fmt.Errorf("could not look up seal: %w", err)
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
			return fmt.Errorf("could not look up result with id=%x: %w", resultID, err)
		}
		builder.addExecutionResult(result)
		builder.includedResults[resultID] = struct{}{}
	}

	// if the block commits to an unseen ProtocolStateID, add the corresponding data entry
	err = builder.addProtocolStateEntryIfUnseen(block.Payload.ProtocolStateID)
	if err != nil {
		return fmt.Errorf("could not check or add protocol state entry: %w", err)
	}

	builder.blocks = append(builder.blocks, block)
	return nil
}

// addProtocolStateEntryIfUnseen checks whether the given protocolStateID corresponds
// to a previously unseen protocol state entry. If it does, retrieves the state entry
// and persists it for inclusion in the resulting SealingSegment.
// No errors expected during normal operation.
func (builder *SealingSegmentBuilder) addProtocolStateEntryIfUnseen(protocolStateID Identifier) error {
	_, exists := builder.protocolStateEntries[protocolStateID]
	if exists {
		return nil
	}

	protocolStateEntry, err := builder.protocolStateLookup(protocolStateID)
	if err != nil {
		return fmt.Errorf("could not look up protocol state entry with id=%x: %w", protocolStateID, err)
	}
	builder.protocolStateEntries[protocolStateID] = protocolStateEntry
	return nil
}

// AddExtraBlock appends an extra block to sealing segment under construction.
// Extra blocks needs to be added in descending order and the first block must connect to the lowest block
// of sealing segment, this way they form a continuous chain.
// Errors expected during normal operation:
//   - InvalidSealingSegmentError if the added block would cause an invalid resulting segment
func (builder *SealingSegmentBuilder) AddExtraBlock(block *Block) error {
	if len(builder.extraBlocks) == 0 {
		if len(builder.blocks) == 0 {
			return fmt.Errorf("cannot add extra blocks before adding lowest sealing segment block")
		}
		// first extra block has to match the lowest block of sealing segment
		if (block.Header.Height + 1) != builder.lowest().Header.Height {
			return NewInvalidSealingSegmentError("invalid extra block height (%d), doesn't connect to sealing segment", block.Header.Height)
		}
	} else if (block.Header.Height + 1) != builder.extraBlocks[len(builder.extraBlocks)-1].Header.Height {
		return NewInvalidSealingSegmentError("invalid extra block height (%d), doesn't connect to last extra block", block.Header.Height)
	}

	// if the block commits to an unseen ProtocolStateID, add the corresponding data entry
	err := builder.addProtocolStateEntryIfUnseen(block.Payload.ProtocolStateID)
	if err != nil {
		return fmt.Errorf("could not check or add protocol state entry: %w", err)
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
// Errors expected during normal operation:
//   - InvalidSealingSegmentError if the added block would cause an invalid resulting segment
func (builder *SealingSegmentBuilder) SealingSegment() (*SealingSegment, error) {
	if err := builder.validateSegment(); err != nil {
		return nil, fmt.Errorf("failed to validate sealing segment: %w", err)
	}

	// SealingSegment must store extra blocks in ascending order, builder stores them in descending.
	// Apply a sort to reverse the slice and use correct ordering.
	slices.SortFunc(builder.extraBlocks, func(lhs, rhs *Block) int {
		return int(lhs.Header.Height) - int(rhs.Header.Height)
	})

	return &SealingSegment{
		Blocks:               builder.blocks,
		ExtraBlocks:          builder.extraBlocks,
		ExecutionResults:     builder.results,
		ProtocolStateEntries: builder.protocolStateEntries,
		LatestSeals:          builder.latestSeals,
		FirstSeal:            builder.firstSeal,
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
//   - the root block must be the first block (least height) in the segment
//   - no blocks in the segment may contain any seals (by the minimality requirement)
//
// Errors expected during normal operation:
//   - InvalidSealingSegmentError if the added block would cause an invalid resulting segment
func (builder *SealingSegmentBuilder) validateRootSegment() error {
	if len(builder.blocks) == 0 {
		return NewInvalidSealingSegmentError("root segment must have at least 1 block")
	}
	if len(builder.extraBlocks) > 0 {
		return NewInvalidSealingSegmentError("root segment cannot have extra blocks")
	}
	if builder.lowest().Header.View != 0 {
		return NewInvalidSealingSegmentError("root block has unexpected view (%d != 0)", builder.lowest().Header.View)
	}
	if len(builder.results) != 1 {
		return NewInvalidSealingSegmentError("expected %d results, got %d", 1, len(builder.results))
	}
	if builder.firstSeal == nil {
		return NewInvalidSealingSegmentError("firstSeal must not be nil for root segment")
	}
	if builder.results[0].BlockID != builder.lowest().ID() {
		return NewInvalidSealingSegmentError("result (block_id=%x) is not for root block (id=%x)", builder.results[0].BlockID, builder.lowest().ID())
	}
	if builder.results[0].ID() != builder.firstSeal.ResultID {
		return NewInvalidSealingSegmentError("firstSeal (result_id=%x) is not for root result (id=%x)", builder.firstSeal.ResultID, builder.results[0].ID())
	}
	if builder.results[0].BlockID != builder.firstSeal.BlockID {
		return NewInvalidSealingSegmentError("root seal (block_id=%x) references different block than root result (block_id=%x)", builder.firstSeal.BlockID, builder.results[0].BlockID)
	}
	for _, block := range builder.blocks {
		if len(block.Payload.Seals) > 0 {
			return NewInvalidSealingSegmentError("root segment cannot contain blocks with seals (minimality requirement) - block (height=%d,id=%x) has %d seals",
				block.Header.Height, block.ID(), len(block.Payload.Seals))
		}
	}
	return nil
}

// validateSegment will validate if builder satisfies conditions for a valid sealing segment.
// Errors expected during normal operation:
//   - InvalidSealingSegmentError if the added block would cause an invalid resulting segment
func (builder *SealingSegmentBuilder) validateSegment() error {
	// sealing cannot be empty
	if len(builder.blocks) == 0 {
		return NewInvalidSealingSegmentError("expect at least 2 blocks in a sealing segment or 1 block in the case of root segments, but got an empty sealing segment")
	}

	if len(builder.extraBlocks) > 0 {
		if builder.extraBlocks[0].Header.Height+1 != builder.lowest().Header.Height {
			return NewInvalidSealingSegmentError("extra blocks don't connect to lowest block in segment")
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
		return NewInvalidSealingSegmentError("sealing segment missing seal (lowest block id: %x) (highest block id: %x): %w", builder.lowest().ID(), builder.highest().ID(), err)
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
func NewSealingSegmentBuilder(resultLookup GetResultFunc, sealLookup GetSealByBlockIDFunc, protocolStateLookup GetProtocolStateEntryFunc) *SealingSegmentBuilder {
	return &SealingSegmentBuilder{
		resultLookup:         resultLookup,
		sealByBlockIDLookup:  sealLookup,
		protocolStateLookup:  protocolStateLookup,
		includedResults:      make(map[Identifier]struct{}),
		latestSeals:          make(map[Identifier]Identifier),
		protocolStateEntries: make(map[Identifier]*ProtocolStateEntryWrapper),
		blocks:               make([]*Block, 0, 10),
		extraBlocks:          make([]*Block, 0, DefaultTransactionExpiry),
		results:              make(ExecutionResultList, 0, 3),
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
// The node logic requires a valid sealing segment to bootstrap.
// No errors are expected during normal operations.
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
