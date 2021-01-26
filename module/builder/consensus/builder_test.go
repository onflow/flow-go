package consensus

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConsensusBuilder(t *testing.T) {
	suite.Run(t, new(BuilderSuite))
}

type BuilderSuite struct {
	suite.Suite

	// test helpers
	firstID           flow.Identifier                           // first block in the range we look at
	finalID           flow.Identifier                           // last finalized block
	parentID          flow.Identifier                           // parent block we build on
	finalizedBlockIDs []flow.Identifier                         // blocks between first and final
	pendingBlockIDs   []flow.Identifier                         // blocks between final and parent
	resultForBlock    map[flow.Identifier]*flow.ExecutionResult // map: BlockID -> Execution Result

	// used to populate and test the seal mempool
	chain   []*flow.Seal                                     // chain of seals starting first
	irsList []*flow.IncorporatedResultSeal                   // chain of IncorporatedResultSeals
	irsMap  map[flow.Identifier]*flow.IncorporatedResultSeal // index for irsList

	// mempools consumed by builder
	pendingGuarantees []*flow.CollectionGuarantee
	pendingReceipts   []*flow.ExecutionReceipt
	pendingSeals      map[flow.Identifier]*flow.IncorporatedResultSeal // storage for the seal mempool

	// storage for dbs
	headers map[flow.Identifier]*flow.Header
	heights map[uint64]*flow.Header
	index   map[flow.Identifier]*flow.Index
	blocks  map[flow.Identifier]*flow.Block

	lastSeal *flow.Seal

	// real dependencies
	dir      string
	db       *badger.DB
	sentinel uint64
	setter   func(*flow.Header) error

	// mocked dependencies
	state    *protocol.MutableState
	headerDB *storage.Headers
	sealDB   *storage.Seals
	indexDB  *storage.Index
	blockDB  *storage.Blocks

	guarPool *mempool.Guarantees
	sealPool *mempool.IncorporatedResultSeals
	recPool  *mempool.ReceiptsForest

	// tracking behaviour
	assembled *flow.Payload // built payload

	// component under test
	build *Builder
}

// createAndRecordBlock creates a new block chained to the previous block (if it
// is not nil). The new block contains a receipt for a result of the previous
// block, which is also used to create a seal for the previous block. The seal
// and the result are combined in an IncorporatedResultSeal which is a candidate
// for the seals mempool.
func (bs *BuilderSuite) createAndRecordBlock(parentBlock *flow.Block) *flow.Block {
	var block flow.Block
	if parentBlock == nil {
		block = unittest.BlockFixture()
	} else {
		block = unittest.BlockWithParentFixture(parentBlock.Header)
	}

	// if parentBlock is not nil, create a receipt for a result of the parentBlock
	// block, and add it to the payload. The corresponding IncorporatedResult
	// will be use to seal the parentBlock block, and to create an
	// IncorporatedResultSeal for the seal mempool.
	var incorporatedResultForPrevBlock *flow.IncorporatedResult
	if parentBlock != nil {
		previousResult, found := bs.resultForBlock[parentBlock.ID()]
		if !found {
			panic("missing execution result for parent")
		}
		receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(previousResult))
		block.Payload.Receipts = append(block.Payload.Receipts, receipt)

		incorporatedResultForPrevBlock = unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(previousResult),
			// ATTENTION: For sealing phase 2, the value for IncorporatedBlockID
			// is the block the result pertains to (here parentBlock).
			// In later development phases, we will change the logic such that
			// IncorporatedBlockID references the
			// block which actually incorporates the result:
			//unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID())
			unittest.IncorporatedResult.WithIncorporatedBlockID(parentBlock.ID()),
		)

		result := unittest.ExecutionResultFixture(
			unittest.WithBlock(&block),
			unittest.WithPreviousResult(*previousResult),
		)

		bs.resultForBlock[result.BlockID] = result
	}

	// record block in dbs
	bs.headers[block.ID()] = block.Header
	bs.heights[block.Header.Height] = block.Header
	bs.blocks[block.ID()] = &block
	bs.index[block.ID()] = block.Payload.Index()

	// seal the parentBlock block with the result included in this block. Do not
	// seal the first block because it is assumed that it is already sealed.
	if parentBlock != nil && parentBlock.ID() != bs.firstID {
		bs.chainSeal(incorporatedResultForPrevBlock)
	}

	return &block
}

// Create a seal for the result's block. The corresponding
// IncorporatedResultSeal, which ties the seal to the incorporated result it
// seals, is also recorded for future access.
func (bs *BuilderSuite) chainSeal(incorporatedResult *flow.IncorporatedResult) {
	incorporatedResultSeal := unittest.IncorporatedResultSeal.Fixture(
		unittest.IncorporatedResultSeal.WithResult(incorporatedResult.Result),
		unittest.IncorporatedResultSeal.WithIncorporatedBlockID(incorporatedResult.IncorporatedBlockID),
	)
	bs.chain = append(bs.chain, incorporatedResultSeal.Seal)
	bs.irsMap[incorporatedResultSeal.ID()] = incorporatedResultSeal
	bs.irsList = append(bs.irsList, incorporatedResultSeal)
}

func (bs *BuilderSuite) SetupTest() {

	// set up no-op dependencies
	noopMetrics := metrics.NewNoopCollector()
	noopTracer := trace.NewNoopTracer()

	// set up test parameters
	numFinalizedBlocks := 4
	numPendingBlocks := 4

	// reset test helpers
	bs.pendingBlockIDs = nil
	bs.finalizedBlockIDs = nil
	bs.resultForBlock = make(map[flow.Identifier]*flow.ExecutionResult)

	bs.chain = nil
	bs.irsMap = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
	bs.irsList = nil

	// initialize the pools
	bs.pendingGuarantees = nil
	bs.pendingSeals = nil
	bs.pendingReceipts = nil

	// initialise the dbs
	bs.lastSeal = nil
	bs.headers = make(map[flow.Identifier]*flow.Header)
	bs.heights = make(map[uint64]*flow.Header)
	bs.index = make(map[flow.Identifier]*flow.Index)
	bs.blocks = make(map[flow.Identifier]*flow.Block)

	// initialize behaviour tracking
	bs.assembled = nil

	// insert the first block in our range
	first := bs.createAndRecordBlock(nil)
	bs.firstID = first.ID()
	firstResult := unittest.ExecutionResultFixture(unittest.WithBlock(first))
	bs.lastSeal = unittest.Seal.Fixture(
		unittest.Seal.WithResult(firstResult),
	)
	bs.resultForBlock[firstResult.BlockID] = firstResult

	// insert the finalized blocks between first and final
	previous := first
	for n := 0; n < numFinalizedBlocks; n++ {
		finalized := bs.createAndRecordBlock(previous)
		bs.finalizedBlockIDs = append(bs.finalizedBlockIDs, finalized.ID())
		previous = finalized
	}

	// insert the finalized block with an empty payload
	final := bs.createAndRecordBlock(previous)
	bs.finalID = final.ID()

	// insert the pending ancestors with empty payload
	previous = final
	for n := 0; n < numPendingBlocks; n++ {
		pending := bs.createAndRecordBlock(previous)
		bs.pendingBlockIDs = append(bs.pendingBlockIDs, pending.ID())
		previous = pending
	}

	// insert the parent block with an empty payload
	parent := bs.createAndRecordBlock(previous)
	bs.parentID = parent.ID()

	// set up temporary database for tests
	bs.db, bs.dir = unittest.TempBadgerDB(bs.T())

	err := bs.db.Update(operation.InsertFinalizedHeight(final.Header.Height))
	bs.Require().NoError(err)
	err = bs.db.Update(operation.IndexBlockHeight(final.Header.Height, bs.finalID))
	bs.Require().NoError(err)

	err = bs.db.Update(operation.InsertRootHeight(13))
	bs.Require().NoError(err)

	err = bs.db.Update(operation.InsertSealedHeight(first.Header.Height))
	bs.Require().NoError(err)
	err = bs.db.Update(operation.IndexBlockHeight(first.Header.Height, first.ID()))
	bs.Require().NoError(err)

	bs.sentinel = 1337

	bs.setter = func(header *flow.Header) error {
		header.View = 1337
		return nil
	}

	bs.state = &protocol.MutableState{}
	bs.state.On("Extend", mock.Anything).Run(func(args mock.Arguments) {
		block := args.Get(0).(*flow.Block)
		bs.Assert().Equal(bs.sentinel, block.Header.View)
		bs.assembled = block.Payload
	}).Return(nil)

	// set up storage mocks for tests
	bs.sealDB = &storage.Seals{}
	bs.sealDB.On("ByBlockID", mock.Anything).Return(bs.lastSeal, nil)

	bs.headerDB = &storage.Headers{}
	bs.headerDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return bs.headers[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := bs.headers[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	bs.headerDB.On("ByHeight", mock.Anything).Return(
		func(height uint64) *flow.Header {
			return bs.heights[height]
		},
		func(height uint64) error {
			_, exists := bs.heights[height]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	bs.indexDB = &storage.Index{}
	bs.indexDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Index {
			return bs.index[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := bs.index[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	bs.blockDB = &storage.Blocks{}
	bs.blockDB.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Block {
			return bs.blocks[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := bs.blocks[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	bs.blockDB.On("Store", mock.Anything).Run(func(args mock.Arguments) {
		block := args.Get(0).(*flow.Block)
		bs.Assert().Equal(bs.sentinel, block.Header.View)
		bs.assembled = block.Payload
	}).Return(nil)

	// set up memory pool mocks for tests
	bs.guarPool = &mempool.Guarantees{}
	bs.guarPool.On("Size").Return(uint(0)) // only used by metrics
	bs.guarPool.On("All").Return(
		func() []*flow.CollectionGuarantee {
			return bs.pendingGuarantees
		},
	)

	bs.sealPool = &mempool.IncorporatedResultSeals{}
	bs.sealPool.On("Size").Return(uint(0)) // only used by metrics
	bs.sealPool.On("All").Return(
		func() []*flow.IncorporatedResultSeal {
			res := make([]*flow.IncorporatedResultSeal, 0, len(bs.pendingSeals))
			for _, ps := range bs.pendingSeals {
				res = append(res, ps)
			}
			return res
		},
	)
	bs.sealPool.On("ByID", mock.Anything).Return(
		func(id flow.Identifier) *flow.IncorporatedResultSeal {
			return bs.pendingSeals[id]
		},
		func(id flow.Identifier) bool {
			_, exists := bs.pendingSeals[id]
			return exists
		},
	)

	bs.recPool = &mempool.ReceiptsForest{}
	bs.recPool.On("Size").Return(uint(0))
	bs.recPool.On("All").Return(
		func() []*flow.ExecutionReceipt {
			return bs.pendingReceipts
		},
	)

	// initialize the builder
	bs.build = NewBuilder(
		noopMetrics,
		bs.db,
		bs.state,
		bs.headerDB,
		bs.sealDB,
		bs.indexDB,
		bs.blockDB,
		bs.guarPool,
		bs.sealPool,
		bs.recPool,
		noopTracer,
	)

	bs.build.cfg.expiry = 11

}

func (bs *BuilderSuite) TearDownTest() {
	err := bs.db.Close()
	bs.Assert().NoError(err)
	err = os.RemoveAll(bs.dir)
	bs.Assert().NoError(err)
}

func (bs *BuilderSuite) TestPayloadEmptyValid() {

	// we should build an empty block with default setup
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
}

func (bs *BuilderSuite) TestPayloadGuaranteeValid() {

	// add sixteen guarantees to the pool
	bs.pendingGuarantees = unittest.CollectionGuaranteesFixture(16, unittest.WithCollRef(bs.finalID))
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(bs.pendingGuarantees, bs.assembled.Guarantees, "should have guarantees from mempool in payload")
}

func (bs *BuilderSuite) TestPayloadGuaranteeDuplicate() {

	// create some valid guarantees
	valid := unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(bs.finalID))

	forkBlocks := append(bs.finalizedBlockIDs, bs.pendingBlockIDs...)

	// create some duplicate guarantees and add to random blocks on the fork
	duplicated := unittest.CollectionGuaranteesFixture(12, unittest.WithCollRef(bs.finalID))
	for _, guarantee := range duplicated {
		blockID := forkBlocks[rand.Intn(len(forkBlocks))]
		index := bs.index[blockID]
		index.CollectionIDs = append(index.CollectionIDs, guarantee.ID())
		bs.index[blockID] = index
	}

	// add sixteen guarantees to the pool
	bs.pendingGuarantees = append(valid, duplicated...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(valid, bs.assembled.Guarantees, "should have valid guarantees from mempool in payload")
}

func (bs *BuilderSuite) TestPayloadGuaranteeReferenceUnknown() {

	// create 12 valid guarantees
	valid := unittest.CollectionGuaranteesFixture(12, unittest.WithCollRef(bs.finalID))

	// create 4 guarantees with unknown reference
	unknown := unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(unittest.IdentifierFixture()))

	// add all guarantees to the pool
	bs.pendingGuarantees = append(valid, unknown...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(valid, bs.assembled.Guarantees, "should have valid from mempool in payload")
}

func (bs *BuilderSuite) TestPayloadGuaranteeReferenceExpired() {

	// create 12 valid guarantees
	valid := unittest.CollectionGuaranteesFixture(12, unittest.WithCollRef(bs.finalID))

	// create 4 expired guarantees
	header := unittest.BlockHeaderFixture()
	header.Height = bs.headers[bs.finalID].Height - 12
	bs.headers[header.ID()] = &header
	expired := unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(header.ID()))

	// add all guarantees to the pool
	bs.pendingGuarantees = append(valid, expired...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(valid, bs.assembled.Guarantees, "should have valid from mempool in payload")
}

func (bs *BuilderSuite) TestPayloadSealAllValid() {

	// use valid chain of seals in mempool
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain, bs.assembled.Seals, "should have included valid chain of seals")
}

// Test maxSealLimit is enforced
func (bs *BuilderSuite) TestPayloadSealLimit() {

	// use valid chain of seals in mempool
	bs.pendingSeals = bs.irsMap

	// change maxSealCount to one less than the number of items in the mempool
	limit := uint(len(bs.irsMap) - 1)
	bs.build.cfg.maxSealCount = limit

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Equal(bs.chain[:limit], bs.assembled.Seals, "should have excluded seals above maxSealCount")
}

// TestPayloadSealOnlyFork checks that the builder only includes seals corresponding
// to blocks on the current fork (and _not_ seals for sealable blocks on other forks)
func (bs *BuilderSuite) TestPayloadSealOnlyFork() {
	// in the test setup, we already created a single fork
	//  [first] <- [F0] <- [F1] <- [F2] <- [F3] <- [A0] <- [A1] <- [A2] <- [A3]
	// Where block
	//   * [first] is sealed and finalized
	//   * [F0] ... [F3] are finalized but _not_ sealed
	//   * [A0] ... [A3] are _not_ finalized and _not_ sealed
	// We now create an additional fork:  [F3] <- [B0] <- [B1] <- ... <- [B7]
	var forkHead *flow.Block
	forkHead = bs.blocks[bs.finalID]
	for i := 0; i < 8; i++ {
		forkHead = bs.createAndRecordBlock(forkHead)
		// Method createAndRecordBlock adds a seal for every block into the mempool.
	}

	bs.pendingSeals = bs.irsMap
	_, err := bs.build.BuildOn(forkHead.ID(), bs.setter)
	bs.Require().NoError(err)

	// expected seals: [F0] <- ... <- [F3] <- [B0] <- ... <- [B7]
	// Note: bs.chain contains seals for blocks  F0 ... F3 then A0 ... A3 and then B0 ... B7
	bs.Assert().Equal(12, len(bs.assembled.Seals), "unexpected number of seals")
	bs.Assert().ElementsMatch(bs.chain[:4], bs.assembled.Seals[:4], "should have included only valid chain of seals")
	bs.Assert().ElementsMatch(bs.chain[len(bs.chain)-8:], bs.assembled.Seals[4:], "should have included only valid chain of seals")

	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
}

// Test that seals for blocks that already have seals on the fork are rejected
func (bs *BuilderSuite) TestPayloadSealDuplicate() {

	// pretend that the first n blocks are already sealed
	n := 4
	lastSeal := bs.chain[n-1]
	// trick the builder into thinking this is the last seal on the fork
	mockSealDB := &storage.Seals{}
	mockSealDB.On("ByBlockID", mock.Anything).Return(lastSeal, nil)
	bs.build.seals = mockSealDB

	// submit all the seals (containing n duplicates)
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.chain[n:], bs.assembled.Seals, "should have rejected duplicate seals")
}

func (bs *BuilderSuite) TestPayloadSealCutoffChain() {

	// remove the seal at the start
	firstSeal := bs.irsList[0]
	delete(bs.irsMap, firstSeal.ID())
	bs.pendingSeals = bs.irsMap

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should not have included any seals from cutoff chain")
}

func (bs *BuilderSuite) TestPayloadSealBrokenChain() {

	// remove a seal in the middle
	seal := bs.irsList[3]
	delete(bs.irsMap, seal.ID())
	bs.pendingSeals = bs.irsMap

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain[:3], bs.assembled.Seals, "should have included only beginning of broken chain")
}

// Receipts for unknown blocks should not be inserted
func (bs *BuilderSuite) TestPayloadReceiptUnknownBlock() {

	// create a valid receipt for an unknown block
	pendingReceiptUnknownBlock := unittest.ExecutionReceiptFixture()
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceiptUnknownBlock)

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when pending receipts are for unknown blocks")
}

// Receipts for blocks that are not on the fork should be skipped
func (bs *BuilderSuite) TestPayloadReceiptForBlockNotInFork() {

	// create a valid receipt for a known, unsealed block, not on the fork
	first := bs.headers[bs.firstID]
	firstHeight := first.Height
	pendingReceipt := unittest.ExecutionReceiptFixture()
	bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{Height: firstHeight + 1000}
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceipt)

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when receipts correspond to blocks not on the fork")
}

// Receipts for sealed blocks on this fork should not be reinserted
func (bs *BuilderSuite) TestPayloadReceiptSealedAndFinalizedBlock() {

	// create a valid receipt for the first block, which is sealed and finalized
	sealedBlock := bs.blocks[bs.firstID]
	pendingReceiptSealedBlock := unittest.ReceiptForBlockFixture(sealedBlock)
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceiptSealedBlock)

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when pending receipts are for sealed blocks")
}

//
// [first] <- [F0] <- [F1] <- [F2] <- [F3] <- [F4] <- [A0] <- [A1] <- [A2] <- [A3] <- [A4]
//                                               |
//                                               +  <- [B0] <- [B1] <- [B2] <- [B3] <- [B4]
//
// Last sealed block on fork A: F0
// Last sealed block on fork B: `first`
//
// This scenario occurs if:
// - No block between F0 and F4 contains a seal for F0
// - No block between B0 and B4 contains a seal for F0
// - There is a block between A0 and A4 that contains a seal for F0
//
// Here we set it up by mocking the calls to sealDB.ByBlockID such that the
// last sealed block at A4 is F0, and the last sealed block at B4 is `first`.
//
// We test that we can Extend fork B with a receipt for block F0
func (bs *BuilderSuite) TestPayloadReceiptSealsOtherFork() {

	// create B fork
	var forkHead *flow.Block
	forkHead = bs.blocks[bs.finalID]
	for i := 0; i < 5; i++ {
		forkHead = bs.createAndRecordBlock(forkHead)
	}

	mockSealDB := &storage.Seals{}
	// set last seal on A fork to F0 (block A's id is  bs.parentID)
	mockSealDB.On("ByBlockID", bs.parentID).Return(bs.irsList[0].Seal, nil)
	// set last seal on B fork to `first`
	mockSealDB.On("ByBlockID", forkHead.ID()).Return(bs.lastSeal, nil)
	bs.build.seals = mockSealDB

	// Add a receipt for F0 in mempool and try to extend B fork
	pendingReceipt := unittest.ReceiptForBlockFixture(bs.blocks[bs.finalizedBlockIDs[0]])
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceipt)

	_, err := bs.build.BuildOn(forkHead.ID(), bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(bs.pendingReceipts, bs.assembled.Receipts, "should still include receipt if the corresponding block was sealed in another fork")
}

// Receipts that are already included in the fork should be skipped.
func (bs *BuilderSuite) TestPayloadReceiptAlreadyInFork() {

	// select a block at random in [finalized blocks] U [pending blocks], which
	// are unsealed blocks on the fork
	unsealedBlocks := append(bs.finalizedBlockIDs, bs.pendingBlockIDs...)
	index := rand.Intn(len(unsealedBlocks))
	block := bs.blocks[unsealedBlocks[index]]

	// try reinserting a receipt that was already contained in that block
	existingReceipt := block.Payload.Receipts[0]
	bs.pendingReceipts = append(bs.pendingReceipts, existingReceipt)

	// it should not be reinserted
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should not reinsert receipts that are already on the fork")
}

// Valid receipts should be inserted in the payload, sorted by block height.
func (bs *BuilderSuite) TestPayloadReceiptSorted() {

	// create valid receipts for known, unsealed blocks, that are in the fork.
	// each receipt has a unique result which is not in the fork yet, so every
	// result should be pushed to the mempool.
	receipts := []*flow.ExecutionReceipt{}
	var i uint64
	for i = 0; i < 5; i++ {
		blockOnFork := bs.blocks[bs.irsList[i].Seal.BlockID]
		pendingReceipt := unittest.ReceiptForBlockFixture(blockOnFork)
		receipts = append(receipts, pendingReceipt)
	}

	// shuffle receipts
	sr := make([]*flow.ExecutionReceipt, len(receipts))
	copy(sr, receipts)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(sr), func(i, j int) { sr[i], sr[j] = sr[j], sr[i] })

	bs.pendingReceipts = sr

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.assembled.Receipts, receipts, "payload should contain receipts ordered by block height")
}

// Payloads can contain multiple receipts for a given block.
func (bs *BuilderSuite) TestPayloadReceiptMultipleReceiptsWithDifferentResults() {

	// create MULTIPLE valid receipts for known, unsealed blocks
	receipts := []*flow.ExecutionReceipt{}

	var i uint64
	for i = 0; i < 5; i++ {
		blockOnFork := bs.blocks[bs.irsList[i].Seal.BlockID]
		pendingReceipt := unittest.ReceiptForBlockFixture(blockOnFork)
		receipts = append(receipts, pendingReceipt)

		// insert 3 receipts for the same block but different results
		for j := 0; j < 3; j++ {
			dupReceipt := unittest.ReceiptForBlockFixture(blockOnFork)
			receipts = append(receipts, dupReceipt)
		}
	}

	bs.pendingReceipts = receipts

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(receipts, bs.assembled.Receipts, "payload should contain all receipts for a given block")
}

// TestPayloadReceiptLimit tests that the builder does not include more receipts
// than the configured maxReceiptCount.
func (bs *BuilderSuite) TestPayloadReceiptLimit() {

	// populate the mempool with 5 valid receipts
	receipts := []*flow.ExecutionReceipt{}
	var i uint64
	for i = 0; i < 5; i++ {
		blockOnFork := bs.blocks[bs.irsList[i].Seal.BlockID]
		pendingReceipt := unittest.ReceiptForBlockFixture(blockOnFork)
		receipts = append(receipts, pendingReceipt)
	}
	bs.pendingReceipts = receipts

	// set maxReceiptCount to 3
	var limit uint = 3
	bs.build.cfg.maxReceiptCount = limit

	// ensure that only 3 of the 5 receipts were included
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.pendingReceipts[:limit], bs.assembled.Receipts, "should have excluded receipts above maxReceiptCount")
}
