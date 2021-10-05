// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/dgraph-io/badger/v2"
	"github.com/opentracing/opentracing-go"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	metrics      module.MempoolMetrics
	receipts     []flow.Identifier
	receiptIndex int
	idblocks     []flow.Identifier
	blockIndex   int
	tracer       module.Tracer
	db           *badger.DB
	state        protocol.MutableState
	seals        storage.Seals
	headers      storage.Headers
	index        storage.Index
	blocks       storage.Blocks
	resultsDB    storage.ExecutionResults
	receiptsDB   storage.ExecutionReceipts
	guarPool     mempool.Guarantees
	sealPool     mempool.IncorporatedResultSeals
	recPool      mempool.ExecutionTree
	cfg          Config
}

// NewBuilder creates a new block builder.
func NewBuilder(
	metrics module.MempoolMetrics,
	db *badger.DB,
	state protocol.MutableState,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	blocks storage.Blocks,
	resultsDB storage.ExecutionResults,
	receiptsDB storage.ExecutionReceipts,
	guarPool mempool.Guarantees,
	sealPool mempool.IncorporatedResultSeals,
	recPool mempool.ExecutionTree,
	tracer module.Tracer,
	options ...func(*Config),
) (*Builder, error) {

	blockTimer, err := blocktimer.NewBlockTimer(500*time.Millisecond, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("could not create default block timer: %w", err)
	}

	// initialize default config
	cfg := Config{
		blockTimer:        blockTimer,
		maxSealCount:      100,
		maxGuaranteeCount: 100,
		maxReceiptCount:   200,
		expiry:            flow.DefaultTransactionExpiry,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		metrics:    metrics,
		db:         db,
		tracer:     tracer,
		state:      state,
		headers:    headers,
		seals:      seals,
		index:      index,
		blocks:     blocks,
		resultsDB:  resultsDB,
		receiptsDB: receiptsDB,
		guarPool:   guarPool,
		sealPool:   sealPool,
		recPool:    recPool,
		cfg:        cfg,
	}

	err = b.repopulateExecutionTree()
	if err != nil {
		return nil, fmt.Errorf("could not repopulate execution tree: %w", err)
	}

	return b, nil
}

// BuildOn creates a new block header on top of the provided parent, using the
// given view and applying the custom setter function to allow the caller to
// make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	// since we don't know the blockID when building the block we track the
	// time indirectly and insert the span directly at the end

	startTime := time.Now()

	// get the collection guarantees to insert in the payload
	insertableGuarantees, err := b.getInsertableGuarantees(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not insert guarantees: %w", err)
	}

	// get the receipts to insert in the payload
	insertableReceipts, err := b.getInsertableReceipts(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not insert receipts: %w", err)
	}

	// get the seals to insert in the payload
	insertableSeals, err := b.getInsertableSeals(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not insert seals: %w", err)
	}

	// assemble the block proposal
	proposal, err := b.createProposal(parentID,
		insertableGuarantees,
		insertableSeals,
		insertableReceipts,
		setter)
	if err != nil {
		return nil, fmt.Errorf("could not assemble proposal: %w", err)
	}

	span, ctx, _ := b.tracer.StartBlockSpan(context.Background(), proposal.ID(), trace.CONBuilderBuildOn, opentracing.StartTime(startTime))
	defer span.Finish()

	err = b.state.Extend(ctx, proposal)
	if err != nil {
		return nil, fmt.Errorf("could not extend state with built proposal: %w", err)
	}

	return proposal.Header, nil
}

// repopulateExecutionTree restores latest state of execution tree mempool based on local chain state information.
// Repopulating of execution tree is split into two parts:
// 1) traverse backwards all finalized blocks starting from last finalized block till we reach last sealed block. [lastSealedHeight, lastFinalizedHeight]
// 2) traverse forward all unfinalized(pending) blocks starting from last finalized block.
// For each block that is being traversed we will collect execution results and add them to execution tree.
func (b *Builder) repopulateExecutionTree() error {
	finalizedSnapshot := b.state.Final()
	finalized, err := finalizedSnapshot.Head()
	if err != nil {
		return fmt.Errorf("could not retrieve finalized block: %w", err)
	}
	finalizedID := finalized.ID()

	// Get the latest sealed block on this fork, i.e. the highest
	// block for which there is a finalized seal.
	latestSeal, err := b.seals.ByBlockID(finalizedID)
	if err != nil {
		return fmt.Errorf("could not retrieve latest seal in fork with head %x: %w", finalizedID, err)
	}
	latestSealedBlockID := latestSeal.BlockID
	latestSealedBlock, err := b.headers.ByBlockID(latestSealedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve latest sealed block (%x): %w", latestSeal.BlockID, err)
	}
	sealedResult, err := b.resultsDB.ByID(latestSeal.ResultID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed result (%x): %w", latestSeal.ResultID, err)
	}

	// prune execution tree to minimum height (while the tree is still empty, for max efficiency)
	err = b.recPool.PruneUpToHeight(latestSealedBlock.Height)
	if err != nil {
		return fmt.Errorf("could not prune execution tree to height %d: %w", latestSealedBlock.Height, err)
	}

	// At initialization, the execution tree is empty. However, during normal operations, we
	// generally query the tree for "all receipts, whose results are derived from the latest
	// sealed and finalized result". This requires the execution tree to know what the latest
	// sealed and finalized result is, so we add it here.
	// Note: we only add the sealed and finalized result, without any Execution Receipts. This
	// is sufficient to create a vertex in the tree. Thereby, we can traverse the tree, starting
	// from the sealed and finalized result, to find derived results and their respective receipts.
	err = b.recPool.AddResult(sealedResult, latestSealedBlock)
	if err != nil {
		return fmt.Errorf("failed to add sealed result as vertex to ExecutionTree (%x): %w", latestSeal.ResultID, err)
	}

	// receiptCollector adds _all known_ receipts for the given block to the execution tree
	receiptCollector := func(header *flow.Header) error {
		receipts, err := b.receiptsDB.ByBlockID(header.ID())
		if err != nil {
			return fmt.Errorf("could not retrieve execution reciepts for block %x: %w", header.ID(), err)
		}
		for _, receipt := range receipts {
			_, err = b.recPool.AddReceipt(receipt, header)
			if err != nil {
				return fmt.Errorf("could not add receipt (%x) to execution tree: %w", receipt.ID(), err)
			}
		}
		return nil
	}

	// Traverse chain backwards and add all known receipts for any finalized, unsealed block to the execution tree.
	// Thereby, we add superset of all unsealed execution results to the execution tree.
	err = fork.TraverseBackward(b.headers, finalizedID, receiptCollector, fork.ExcludingBlock(latestSealedBlockID))
	if err != nil {
		return fmt.Errorf("failed to traverse unsealed, finalized blocks: %w", err)
	}

	// At this point execution tree is filled with all results for blocks (lastSealedBlock, lastFinalizedBlock].
	// Now, we add all known receipts for any valid block that descends from the latest finalized block:
	validPending, err := finalizedSnapshot.ValidDescendants()
	if err != nil {
		return fmt.Errorf("could not retrieve valid pending blocks from finalized snapshot: %w", err)
	}
	for _, blockID := range validPending {
		block, err := b.headers.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve header for unfinalized block %x: %w", blockID, err)
		}
		err = receiptCollector(block)
		if err != nil {
			return fmt.Errorf("failed to add receipts for unfinalized block %x at height %d: %w", blockID, block.Height, err)
		}
	}

	return nil
}

func compareIdentifiers(a flow.Identifier, b flow.Identifier) (bool, bool) {
	// retrieve the first 8 bytes, as slices for the encoding
	num1 := a[:]
	num2 := b[:]
	first := binary.BigEndian.Uint64(num1)
	second := binary.BigEndian.Uint64(num2)

	// compare the second set of 8 bytes in the 32-byte Identifier
	if first == second {
		num1 = a[8:]
		num2 = b[8:]
		first = binary.BigEndian.Uint64(num1)
		second = binary.BigEndian.Uint64(num2)

		if first == second {
			num1 = a[16:]
			num2 = b[16:]
			first = binary.BigEndian.Uint64(num1)
			second = binary.BigEndian.Uint64(num2)

			if first == second {
				num1 = a[24:]
				num2 = b[24:]
				first = binary.BigEndian.Uint64(num1)
				second = binary.BigEndian.Uint64(num2)

				if first == second {
					return true, false
				}

				return false, first < second
			}
			return false, first < second
		}
		return false, first < second
	}
	return false, first < second
}

func Search(arr []flow.Identifier, n flow.Identifier, arrayLen int) bool {
	l := 0
	r := arrayLen - 1

	for {
		h := int(uint(l+r) >> 1) // avoid overflow when computing h

		bEqual, bLess := compareIdentifiers(arr[h], n)

		if bLess {
			l = h + 1
		} else if !bEqual {
			// set the arrayLen to be half what it was
			r = h - 1
		} else {
			// found
			return true
		}

		if l > r {
			// not found
			return false
		}
	}
}

var mtx sync.Mutex

// getInsertableGuarantees returns the list of CollectionGuarantees that should
// be inserted in the next payload. It looks in the collection mempool and
// applies the following filters:
//
// 1) If it was already included in the fork, skip.
//
// 2) If it references an unknown block, skip.
//
// 3) If the referenced block has an expired height, skip.
//
// 4) Otherwise, this guarantee can be included in the payload.
func (b *Builder) getInsertableGuarantees(parentID flow.Identifier) ([]*flow.CollectionGuarantee, error) {

	// we look back only as far as the expiry limit for the current height we
	// are building for; any guarantee with a reference block before that can
	// not be included anymore anyway
	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}
	height := parent.Height + 1
	limit := height - uint64(b.cfg.expiry)
	if limit > height { // overflow check
		limit = 0
	}

	// look up the root height so we don't look too far back
	// initially this is the genesis block height (aka 0).
	var rootHeight uint64
	err = b.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root block height: %w", err)
	}
	if limit < rootHeight {
		limit = rootHeight
	}

	mtx.Lock()

	defer mtx.Unlock()

	// make a slice to track of the blocks
	// keeps track of the blocks from limit to parent
	b.idblocks = make([]flow.Identifier, uint(b.cfg.expiry))
	b.blockIndex = 0

	// receiptLookup keeps track of the receipts contained in blocks between
	// limit and parent
	b.receipts = make([]flow.Identifier, uint(b.cfg.expiry))
	b.receiptIndex = 0

	// loop through the fork backwards, from parent to limit (inclusive),
	// and keep track of blocks and collections visited on the way
	forkScanner := func(header *flow.Header) error {
		ancestorID := header.ID()

		b.idblocks[b.blockIndex] = ancestorID
		b.blockIndex++

		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}

		for _, collID := range index.CollectionIDs {
			b.receipts[b.receiptIndex] = collID
			b.receiptIndex++
		}

		return nil
	}

	err = fork.TraverseBackward(b.headers, parentID, forkScanner, fork.IncludingHeight(limit))
	if err != nil {
		return nil, fmt.Errorf("internal error building set of CollectionGuarantees on fork: %w", err)
	}

	// before sorting, resize the slice to the actual size
	(*reflect.SliceHeader)(unsafe.Pointer(&b.idblocks)).Len = b.blockIndex
	(*reflect.SliceHeader)(unsafe.Pointer(&b.receipts)).Len = b.receiptIndex

	// sort the blocks and receipts slices, for searching later
	sort.Slice(b.idblocks, func(p, q int) bool {
		_, bLess := compareIdentifiers(b.idblocks[p], b.idblocks[q])
		return bLess
	})
	lenReceipts := len(b.receipts)
	lenBlocks := len(b.idblocks)

	sort.Slice(b.receipts, func(p, q int) bool {
		_, bLess := compareIdentifiers(b.receipts[p], b.receipts[q])
		return bLess
	})

	// go through mempool and collect valid collections
	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		// add at most <maxGuaranteeCount> number of collection guarantees in a new block proposal
		// in order to prevent the block payload from being too big or computationally heavy for the
		// execution nodes
		if uint(len(guarantees)) >= b.cfg.maxGuaranteeCount {
			break
		}

		collID := guarantee.ID()

		// skip collections that are already included in a block on the fork

		if lenReceipts > 0 && Search(b.receipts, collID, lenReceipts) {
			continue
		}

		// skip collections for blocks that are not within the limit
		if lenBlocks == 0 || !Search(b.idblocks, guarantee.ReferenceBlockID, lenBlocks) {
			println("builder-debug\tblock not within limit")
			continue
		}

		guarantees = append(guarantees, guarantee)
	}

	return guarantees, nil
}

// getInsertableSeals returns the list of Seals from the mempool that should be
// inserted in the next payload.
// Per protocol definition, a specific result is only incorporated _once_ in each fork.
// Specifically, the result is incorporated in the block that contains a receipt committing
// to a result for the _first time_ in the respective fork.
// We can seal a result if and only if _all_ of the following conditions are satisfied:
//  (0) We have collected a sufficient number of approvals for each of the result's chunks.
//  (1) The result must have been previously incorporated in the fork, which we are extending.
//      Note: The protocol dictates that all incorporated results must be for ancestor blocks
//            in the respective fork. Hence, a result being incorporated in the fork, implies
//            that the result must be for a block in this fork.
//  (2) The result must be for an _unsealed_ block.
//  (3) The result's parent must have been previously sealed (either by a seal in an ancestor
//      block or by a seal included earlier in the block that we are constructing).
// To limit block size, we cap the number of seals to maxSealCount.
func (b *Builder) getInsertableSeals(parentID flow.Identifier) ([]*flow.Seal, error) {
	// get the latest seal in the fork, which we are extending and
	// the corresponding block, whose result is sealed
	// Note: the last seal might not be included in a finalized block yet
	lastSeal, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve latest seal in the fork, which we are extending: %w", err)
	}
	latestSealedBlockID := lastSeal.BlockID
	latestSealedBlock, err := b.headers.ByBlockID(latestSealedBlockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve sealed block %x: %w", lastSeal.BlockID, err)
	}
	latestSealedHeight := latestSealedBlock.Height

	// STEP I: Collect the seals for all results that satisfy (0), (1), and (2).
	//         The will give us a _superset_ of all seals that can be included.
	// Implementation:
	//  * We walk the fork backwards and check each block for incorporated results.
	//    - Therefore, all results that we encounter satisfy condition (1).
	//  * We only consider results, whose executed block has a height _strictly larger_
	//    than the lastSealedHeight.
	//    - Thereby, we guarantee that condition (2) is satisfied.
	//  * We only consider results for which we have a candidate seals in the sealPool.
	//    - Thereby, we guarantee that condition (0) is satisfied, because candidate seals
	//      are only generated and stored in the mempool once sufficient approvals are collected.
	// Furthermore, condition (2) imposes a limit on how far we have to walk back:
	//  * A result can only be incorporated in a child of the block that it computes.
	//    Therefore, we only have to inspect the results incorporated in unsealed blocks.
	sealsSuperset := make(map[uint64][]*flow.IncorporatedResultSeal) // map: executedBlock.Height -> candidate Seals
	sealCollector := func(header *flow.Header) error {
		blockID := header.ID()
		if blockID == parentID {
			// Important protocol edge case: There must be at least one block in between the block incorporating
			// a result and the block sealing the result. This is because we need the Source of Randomness for
			// the block that _incorporates_ the result, to compute the verifier assignment. Therefore, we require
			// that the block _incorporating_ the result has at least one child in the fork, _before_ we include
			// the seal. Thereby, we guarantee that a verifier assignment can be computed without needing
			// information from the block that we are just constructing. Hence, we don't consider results for
			// sealing that were incorporated in the immediate parent which we are extending.
			return nil
		}

		index, err := b.index.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve index for block %x: %w", blockID, err)
		}

		// enforce condition (1): only consider seals for results that are incorporated in the fork
		for _, resultID := range index.ResultIDs {
			result, err := b.resultsDB.ByID(resultID)
			if err != nil {
				return fmt.Errorf("could not retrieve execution result %x: %w", resultID, err)
			}

			// re-assemble the IncorporatedResult because we need its ID to
			// check if it is in the seal mempool.
			incorporatedResult := flow.NewIncorporatedResult(
				blockID,
				result,
			)

			// enforce condition (0): candidate seals are only constructed once sufficient
			// approvals have been collected. Hence, any incorporated result for which we
			// find a candidate seal satisfies condition (0)
			irSeal, ok := b.sealPool.ByID(incorporatedResult.ID())
			if !ok {
				continue
			}

			// enforce condition (2): the block is unsealed (in this fork) if and only if
			// its height is _strictly larger_ than the lastSealedHeight.
			executedBlock, err := b.headers.ByBlockID(incorporatedResult.Result.BlockID)
			if err != nil {
				return fmt.Errorf("could not get header of block %x: %w", incorporatedResult.Result.BlockID, err)
			}
			if executedBlock.Height <= latestSealedHeight {
				continue
			}

			// The following is a subtle but important protocol edge case: There can be multiple
			// candidate seals for the same block. We have to include all to guarantee sealing liveness!
			sealsSuperset[executedBlock.Height] = append(sealsSuperset[executedBlock.Height], irSeal)
		}

		return nil
	}
	err = fork.TraverseBackward(b.headers, parentID, sealCollector, fork.ExcludingBlock(latestSealedBlockID))
	if err != nil {
		return nil, fmt.Errorf("internal error traversing unsealed section of fork: %w", err)
	}
	// All the seals in sealsSuperset are for results that satisfy (0), (1), and (2).

	// STEP II: Select only the seals from sealsSuperset that also satisfy condition (3).
	// We do this by starting with the last sealed result in the fork. Then, we check whether we
	// have a seal for the child block (at latestSealedBlock.Height +1), which connects to the
	// sealed result. If we find such a seal, we can now consider the child block sealed.
	// We continue until we stop finding a seal for the child.
	seals := make([]*flow.Seal, 0, len(sealsSuperset))
	for {
		// cap the number of seals
		if uint(len(seals)) >= b.cfg.maxSealCount {
			break
		}

		// enforce condition (3):
		candidateSeal, ok := connectingSeal(sealsSuperset[latestSealedHeight+1], lastSeal)
		if !ok {
			break
		}
		seals = append(seals, candidateSeal)
		lastSeal = candidateSeal
		latestSealedHeight += 1
	}
	return seals, nil
}

// connectingSeal looks through `sealsForNextBlock`. It checks whether the
// sealed result directly descends from the lastSealed result.
func connectingSeal(sealsForNextBlock []*flow.IncorporatedResultSeal, lastSealed *flow.Seal) (*flow.Seal, bool) {
	for _, candidateSeal := range sealsForNextBlock {
		if candidateSeal.IncorporatedResult.Result.PreviousResultID == lastSealed.ResultID {
			return candidateSeal.Seal, true
		}
	}
	return nil, false
}

type InsertableReceipts struct {
	receipts []*flow.ExecutionReceiptMeta
	results  []*flow.ExecutionResult
}

// getInsertableReceipts constructs:
//  (i)  the meta information of the ExecutionReceipts (i.e. ExecutionReceiptMeta)
//       that should be inserted in the next payload
//  (ii) the ExecutionResults the receipts from step (i) commit to
//       (deduplicated w.r.t. the block under construction as well as ancestor blocks)
// It looks in the receipts mempool and applies the following filter:
//
// 1) If it doesn't correspond to an unsealed block on the fork, skip it.
//
// 2) If it was already included in the fork, skip it.
//
// 3) Otherwise, this receipt can be included in the payload.
//
// Receipts have to be ordered by block height.
func (b *Builder) getInsertableReceipts(parentID flow.Identifier) (*InsertableReceipts, error) {

	// Get the latest sealed block on this fork, ie the highest block for which
	// there is a seal in this fork. This block is not necessarily finalized.
	latestSeal, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", parentID, err)
	}
	sealedBlockID := latestSeal.BlockID

	// ancestors is used to keep the IDs of the ancestor blocks we iterate through.
	// We use it to skip receipts that are not for unsealed blocks in the fork.
	ancestors := make(map[flow.Identifier]struct{})

	// includedReceipts is a set of all receipts that are contained in unsealed blocks along the fork.
	includedReceipts := make(map[flow.Identifier]struct{})

	// includedResults is a set of all unsealed results that were incorporated into fork
	includedResults := make(map[flow.Identifier]struct{})

	// loop through the fork backwards, from parent to last sealed (including),
	// and keep track of blocks and receipts visited on the way.
	forkScanner := func(ancestor *flow.Header) error {
		ancestorID := ancestor.ID()
		ancestors[ancestorID] = struct{}{}

		index, err := b.index.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get payload index of block %x: %w", ancestorID, err)
		}
		for _, recID := range index.ReceiptIDs {
			includedReceipts[recID] = struct{}{}
		}
		for _, resID := range index.ResultIDs {
			includedResults[resID] = struct{}{}
		}

		return nil
	}
	err = fork.TraverseBackward(b.headers, parentID, forkScanner, fork.IncludingBlock(sealedBlockID))
	if err != nil {
		return nil, fmt.Errorf("internal error building set of CollectionGuarantees on fork: %w", err)
	}

	isResultForUnsealedBlock := isResultForBlock(ancestors)
	isReceiptUniqueAndUnsealed := isNoDupAndNotSealed(includedReceipts, sealedBlockID)
	// find all receipts:
	// 1) whose result connects all the way to the last sealed result
	// 2) is unique (never seen in unsealed blocks)
	receipts, err := b.recPool.ReachableReceipts(latestSeal.ResultID, isResultForUnsealedBlock, isReceiptUniqueAndUnsealed)
	// Occurrence of UnknownExecutionResultError:
	// Populating the execution with receipts from incoming blocks happens concurrently in
	// matching.Core. Hence, the following edge case can occur (rarely): matching.Core is
	// just in the process of populating the Execution Tree with the receipts from the
	// latest blocks, while the builder is already trying to build on top. In this rare
	// situation, the Execution Tree might not yet know the latest sealed result.
	// TODO: we should probably remove this edge case by _synchronously_ populating
	//       the Execution Tree in the Fork's finalizationCallback
	if err != nil && !mempool.IsUnknownExecutionResultError(err) {
		return nil, fmt.Errorf("failed to retrieve reachable receipts from memool: %w", err)
	}

	insertables := toInsertables(receipts, includedResults, b.cfg.maxReceiptCount)
	return insertables, nil
}

// toInsertables separates the provided receipts into ExecutionReceiptMeta and
// ExecutionResult. Results that are in includedResults are skipped.
// We also limit the number of receipts to maxReceiptCount.
func toInsertables(receipts []*flow.ExecutionReceipt, includedResults map[flow.Identifier]struct{}, maxReceiptCount uint) *InsertableReceipts {
	results := make([]*flow.ExecutionResult, 0)

	count := uint(len(receipts))
	// don't collect more than maxReceiptCount receipts
	if count > maxReceiptCount {
		count = maxReceiptCount
	}

	filteredReceipts := make([]*flow.ExecutionReceiptMeta, 0, count)

	for i := uint(0); i < count; i++ {
		receipt := receipts[i]
		meta := receipt.Meta()
		resultID := meta.ResultID
		if _, inserted := includedResults[resultID]; !inserted {
			results = append(results, &receipt.ExecutionResult)
			includedResults[resultID] = struct{}{}
		}

		filteredReceipts = append(filteredReceipts, meta)
	}

	return &InsertableReceipts{
		receipts: filteredReceipts,
		results:  results,
	}
}

// createProposal assembles a block with the provided header and payload
// information
func (b *Builder) createProposal(parentID flow.Identifier,
	guarantees []*flow.CollectionGuarantee,
	seals []*flow.Seal,
	insertableReceipts *InsertableReceipts,
	setter func(*flow.Header) error) (*flow.Block, error) {

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Guarantees: guarantees,
		Seals:      seals,
		Receipts:   insertableReceipts.receipts,
		Results:    insertableReceipts.results,
	}

	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}

	timestamp := b.cfg.blockTimer.Build(parent.Timestamp)

	// construct default block on top of the provided parent
	header := &flow.Header{
		ChainID:     parent.ChainID,
		ParentID:    parentID,
		Height:      parent.Height + 1,
		Timestamp:   timestamp,
		PayloadHash: payload.Hash(),

		// the following fields should be set by the custom function as needed
		// NOTE: we could abstract all of this away into an interface{} field,
		// but that would be over the top as we will probably always use hotstuff
		View:               0,
		ParentVoterIDs:     nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
		ProposerSigData:    nil,
	}

	// apply the custom fields setter of the consensus algorithm
	err = setter(header)
	if err != nil {
		return nil, fmt.Errorf("could not apply setter: %w", err)
	}

	proposal := &flow.Block{
		Header:  header,
		Payload: payload,
	}

	return proposal, nil
}

// isResultForBlock constructs a mempool.BlockFilter that accepts only blocks whose ID is part of the given set.
func isResultForBlock(blockIDs map[flow.Identifier]struct{}) mempool.BlockFilter {
	blockIdFilter := id.InSet(blockIDs)
	return func(h *flow.Header) bool {
		return blockIdFilter(h.ID())
	}
}

// isNoDupAndNotSealed constructs a mempool.ReceiptFilter for discarding receipts that
// * are duplicates
// * or are for the sealed block
func isNoDupAndNotSealed(includedReceipts map[flow.Identifier]struct{}, sealedBlockID flow.Identifier) mempool.ReceiptFilter {
	return func(receipt *flow.ExecutionReceipt) bool {
		if _, duplicate := includedReceipts[receipt.ID()]; duplicate {
			return false
		}
		if receipt.ExecutionResult.BlockID == sealedBlockID {
			return false
		}
		return true
	}
}
