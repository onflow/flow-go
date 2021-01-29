package consensus

import (
	"math/rand"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mempoolAPIs "github.com/onflow/flow-go/module/mempool"
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
	resultByID        map[flow.Identifier]*flow.ExecutionResult // map: result ID -> Execution Result

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
	resultDB *storage.ExecutionResults

	guarPool *mempool.Guarantees
	sealPool *mempool.IncorporatedResultSeals
	recPool  *mempool.ExecutionTree

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
		bs.resultByID[result.ID()] = result
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
	bs.resultByID = make(map[flow.Identifier]*flow.ExecutionResult)

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
	bs.resultByID[firstResult.ID()] = firstResult

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

	bs.resultDB = &storage.ExecutionResults{}
	bs.resultDB.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return bs.resultByID[resultID]
		},
		func(resultID flow.Identifier) error {
			_, exists := bs.resultByID[resultID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

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

	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe() // used for metrics only
	bs.recPool.On("AddResult", mock.Anything, mock.Anything).Return(nil)
	bs.recPool.On("ReachableReceipts", mock.Anything, mock.Anything, mock.Anything).Return(
		func(resultID flow.Identifier, blockFilter mempoolAPIs.BlockFilter, receiptFilter mempoolAPIs.ReceiptFilter) []*flow.ExecutionReceipt {
			return bs.pendingReceipts
		},
		nil,
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
		bs.resultDB,
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

// TestPayloadReceipts_TraverseExecutionTreeFromLastSealedResult tests the receipt selection:
// Expectation: Builder should trigger ExecutionTree to search Execution Tree from
//              last sealed result on respective fork.
// We test with the following main chain tree
//                                                ┌-[X0] <- [X1{seals ..F4}]
//                                                v
// [lastSeal] <- [F0] <- [F1] <- [F2] <- [F3] <- [F4] <- [A0] <- [A1{seals ..F2}] <- [A2] <- [A3]
// Where
// * blocks [lastSeal], [F1], ... [F4], [A0], ... [A4], are created by BuilderSuite
// * latest sealed block for a specific fork is provided by test-local seals storage mock
func (bs *BuilderSuite) TestPayloadReceipts_TraverseExecutionTreeFromLastSealedResult() {
	bs.build.cfg.expiry = 4 // reduce expiry so collection dedup algorithm doesn't walk past  [lastSeal]
	x0 := bs.createAndRecordBlock(bs.blocks[bs.finalID])
	x1 := bs.createAndRecordBlock(x0)

	// set last sealed blocks:
	f2 := bs.blocks[bs.finalizedBlockIDs[2]]
	f2eal := unittest.Seal.Fixture(unittest.Seal.WithResult(bs.resultForBlock[f2.ID()]))
	f4Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(bs.resultForBlock[bs.finalID]))
	bs.sealDB = &storage.Seals{}
	bs.build.seals = bs.sealDB

	// reset receipts mempool to verify calls made by Builder
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
	bs.build.recPool = bs.recPool

	// building on top of X0: latest finalized block in fork is [lastSeal]; expect search to start with sealed result
	bs.sealDB.On("ByBlockID", x0.ID()).Return(bs.lastSeal, nil)
	bs.recPool.On("AddResult", bs.resultByID[bs.lastSeal.ResultID], bs.blocks[bs.lastSeal.BlockID].Header).Return(nil).Once()
	bs.recPool.On("ReachableReceipts", bs.lastSeal.ResultID, mock.Anything, mock.Anything).Return([]*flow.ExecutionReceipt{}, nil).Once()
	_, err := bs.build.BuildOn(x0.ID(), bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())

	// building on top of X1: latest finalized block in fork is [F4]; expect search to start with sealed result
	bs.sealDB.On("ByBlockID", x1.ID()).Return(f4Seal, nil)
	bs.recPool.On("AddResult", bs.resultByID[f4Seal.ResultID], bs.blocks[bs.finalID].Header).Return(nil).Once()
	bs.recPool.On("ReachableReceipts", f4Seal.ResultID, mock.Anything, mock.Anything).Return([]*flow.ExecutionReceipt{}, nil).Once()
	_, err = bs.build.BuildOn(x1.ID(), bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())

	// building on top of A3 (with ID bs.parentID): latest finalized block in fork is [F4]; expect search to start with sealed result
	bs.sealDB.On("ByBlockID", bs.parentID).Return(f2eal, nil)
	bs.recPool.On("AddResult", bs.resultByID[f2eal.ResultID], f2.Header).Return(nil).Once()
	bs.recPool.On("ReachableReceipts", f2eal.ResultID, mock.Anything, mock.Anything).Return([]*flow.ExecutionReceipt{}, nil).Once()
	_, err = bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())
}

// TestPayloadReceipts_IncludeOnlyReceiptsForCurrentFork tests the receipt selection:
// In this test, we check that the Builder provides a BlockFilter which only allows
// blocks on the fork, which we are extending. We construct the following chain tree:
//       ┌--[X1]   ┌-[Y2]                                             ┌-- [A6]
//       v         v                                                  v
// <- [Final] <- [*B1*] <- [*B2*] <- [*B3*] <- [*B4{seals B1}*] <- [*B5*] <- ░newBlock░
//                           ^
//                           └-- [C3] <- [C4]
//                                  ^--- [D4]
// Expectation: BlockFilter should pass blocks marked with star: B1, ... ,B5
//              All other blocks should be filtered out.
// Context:
// While the receipt selection itself is performed by the ExecutionTree, the Builder
// controls the selection by providing suitable BlockFilter and ReceiptFilter.
func (bs *BuilderSuite) TestPayloadReceipts_IncludeOnlyReceiptsForCurrentFork() {
	b1 := bs.createAndRecordBlock(bs.blocks[bs.finalID])
	b2 := bs.createAndRecordBlock(b1)
	b3 := bs.createAndRecordBlock(b2)
	b4 := bs.createAndRecordBlock(b3)
	b5 := bs.createAndRecordBlock(b4)

	x1 := bs.createAndRecordBlock(bs.blocks[bs.finalID])
	y2 := bs.createAndRecordBlock(b1)
	a6 := bs.createAndRecordBlock(b5)

	c3 := bs.createAndRecordBlock(b2)
	c4 := bs.createAndRecordBlock(c3)
	d4 := bs.createAndRecordBlock(c3)

	// set last sealed blocks:
	b1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(bs.resultForBlock[b1.ID()]))
	bs.sealDB = &storage.Seals{}
	bs.sealDB.On("ByBlockID", b5.ID()).Return(b1Seal, nil)
	bs.build.seals = bs.sealDB

	// setup mock to test the BlockFilter provided by Builder
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
	bs.recPool.On("AddResult", bs.resultByID[b1Seal.ResultID], b1.Header).Return(nil).Once()
	bs.recPool.On("ReachableReceipts", b1Seal.ResultID, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			blockFilter := args[1].(mempoolAPIs.BlockFilter)
			for _, h := range []*flow.Header{b1.Header, b2.Header, b3.Header, b4.Header, b5.Header} {
				assert.True(bs.T(), blockFilter(h))
			}
			for _, h := range []*flow.Header{bs.blocks[bs.finalID].Header, x1.Header, y2.Header, a6.Header, c3.Header, c4.Header, d4.Header} {
				assert.False(bs.T(), blockFilter(h))
			}
		}).Return([]*flow.ExecutionReceipt{}, nil).Once()
	bs.build.recPool = bs.recPool

	_, err := bs.build.BuildOn(b5.ID(), bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())
}

// TestPayloadReceipts_SkipDuplicatedReceipts tests the receipt selection:
// Expectation: we check that the Builder provides a ReceiptFilter which
//              filters out duplicated receipts.
// Comment:
// While the receipt selection itself is performed by the ExecutionTree, the Builder
// controls the selection by providing suitable BlockFilter and ReceiptFilter.
func (bs *BuilderSuite) TestPayloadReceipts_SkipDuplicatedReceipts() {
	// setup mock to test the ReceiptFilter provided by Builder
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
	bs.recPool.On("AddResult", bs.resultByID[bs.lastSeal.ResultID], bs.blocks[bs.lastSeal.BlockID].Header).Return(nil).Once()
	bs.recPool.On("ReachableReceipts", bs.lastSeal.ResultID, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			receiptFilter := args[2].(mempoolAPIs.ReceiptFilter)
			// verify that all receipts already included in blocks are filtered out:
			for _, block := range bs.blocks {
				for _, rcpt := range block.Payload.Receipts {
					assert.False(bs.T(), receiptFilter(rcpt))
				}
			}
			// Verify that receipts for unsealed blocks, which are _not_ already incorporated are accepted:
			for _, block := range bs.blocks {
				if block.ID() != bs.firstID { // block with ID bs.firstID is already sealed
					rcpt := unittest.ReceiptForBlockFixture(block)
					assert.True(bs.T(), receiptFilter(rcpt))
				}
			}
		}).Return([]*flow.ExecutionReceipt{}, nil).Once()
	bs.build.recPool = bs.recPool

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())
}

// TestPayloadReceipts_SkipReceiptsForSealedBlock tests the receipt selection:
// Expectation: we check that the Builder provides a ReceiptFilter which
//              filters out _any_ receipt for the sealed block.
// Comment:
// While the receipt selection itself is performed by the ExecutionTree, the Builder
// controls the selection by providing suitable BlockFilter and ReceiptFilter.
func (bs *BuilderSuite) TestPayloadReceipts_SkipReceiptsForSealedBlock() {
	// setup mock to test the ReceiptFilter provided by Builder
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
	bs.recPool.On("AddResult", bs.resultByID[bs.lastSeal.ResultID], bs.blocks[bs.lastSeal.BlockID].Header).Return(nil).Once()
	bs.recPool.On("ReachableReceipts", bs.lastSeal.ResultID, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			receiptFilter := args[2].(mempoolAPIs.ReceiptFilter)

			// receipt for sealed block committing to same result as the sealed result
			rcpt := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.firstID]))
			assert.False(bs.T(), receiptFilter(rcpt))

			// receipt for sealed block committing to different result as the sealed result
			rcpt = unittest.ReceiptForBlockFixture(bs.blocks[bs.firstID])
			assert.False(bs.T(), receiptFilter(rcpt))
		}).Return([]*flow.ExecutionReceipt{}, nil).Once()
	bs.build.recPool = bs.recPool

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())
}

// TestPayloadReceipts_BlockLimit tests that the builder does not include more
// receipts than the configured maxReceiptCount.
func (bs *BuilderSuite) TestPayloadReceipts_BlockLimit() {

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

// TestPayloadReceipts_AsProvidedByReceiptForest tests the receipt selection.
// Expectation: Builder should embed the Receipts as provided by the ExecutionTree
func (bs *BuilderSuite) TestPayloadReceipts_AsProvidedByReceiptForest() {
	var expectedReceipts []*flow.ExecutionReceipt
	for i := 0; i < 10; i++ {
		expectedReceipts = append(expectedReceipts, unittest.ExecutionReceiptFixture())
	}
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
	bs.recPool.On("AddResult", mock.Anything, mock.Anything).Return(nil).Maybe()
	bs.recPool.On("ReachableReceipts", mock.Anything, mock.Anything, mock.Anything).Return(expectedReceipts, nil).Once()
	bs.build.recPool = bs.recPool

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(expectedReceipts, bs.assembled.Receipts, "should include receipts as returned by ExecutionTree")
	bs.recPool.AssertExpectations(bs.T())
}
