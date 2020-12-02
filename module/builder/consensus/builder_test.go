package consensus

import (
	"math/rand"
	"os"
	"testing"

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

	// mempool written to by builder
	incorporatedResults []*flow.IncorporatedResult

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
	state    *protocol.State
	mutator  *protocol.Mutator
	headerDB *storage.Headers
	sealDB   *storage.Seals
	indexDB  *storage.Index
	blockDB  *storage.Blocks

	guarPool *mempool.Guarantees
	sealPool *mempool.IncorporatedResultSeals
	recPool  *mempool.Receipts
	resPool  *mempool.IncorporatedResults

	// tracking behaviour
	assembled  *flow.Payload     // built payload
	remCollIDs []flow.Identifier // guarantees removed from mempool
	remRecIDs  []flow.Identifier // receipts removed from mempool

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
			unittest.IncorporatedResult.WithIncorporatedBlockID(parentBlock.ID()),
			// For sealing phase 2, the value for IncorporatedBlockID is the block the result pertains to (here parentBlock).
			// In later development phases, we will change the logic such that IncorporatedBlockID references the
			// block which actually incorporates the result:
			//unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
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
	bs.incorporatedResults = nil

	// initialise the dbs
	bs.lastSeal = nil
	bs.headers = make(map[flow.Identifier]*flow.Header)
	bs.heights = make(map[uint64]*flow.Header)
	bs.index = make(map[flow.Identifier]*flow.Index)
	bs.blocks = make(map[flow.Identifier]*flow.Block)

	// initialize behaviour tracking
	bs.assembled = nil
	bs.remCollIDs = nil
	bs.remRecIDs = nil

	// insert the first block in our range
	first := bs.createAndRecordBlock(nil)
	bs.firstID = first.ID()
	firstResult := unittest.ExecutionResultFixture(unittest.WithBlock(first))
	firstSealedState, ok := firstResult.FinalStateCommitment()
	if !ok {
		panic("missing first execution result's final state commitment")
	}
	bs.lastSeal = &flow.Seal{
		BlockID:    first.ID(),
		ResultID:   firstResult.ID(),
		FinalState: firstSealedState,
	}
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

	bs.state = &protocol.State{}
	bs.mutator = &protocol.Mutator{}
	bs.state.On("Mutate").Return(bs.mutator)
	bs.mutator.On("Extend", mock.Anything).Run(func(args mock.Arguments) {
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
	bs.guarPool.On("Rem", mock.Anything).Run(func(args mock.Arguments) {
		collID := args.Get(0).(flow.Identifier)
		bs.remCollIDs = append(bs.remCollIDs, collID)
	}).Return(true)

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

	bs.recPool = &mempool.Receipts{}
	bs.recPool.On("Size").Return(uint(0))
	bs.recPool.On("All").Return(
		func() []*flow.ExecutionReceipt {
			return bs.pendingReceipts
		},
	)
	bs.recPool.On("Rem", mock.Anything).Run(func(args mock.Arguments) {
		recID := args.Get(0).(flow.Identifier)
		bs.remRecIDs = append(bs.remRecIDs, recID)
	}).Return(true)

	bs.resPool = &mempool.IncorporatedResults{}
	bs.resPool.On("Size").Return(uint(0))
	bs.resPool.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*flow.IncorporatedResult)
			bs.incorporatedResults = append(bs.incorporatedResults, res)
		},
	).Return(true)

	// initialize the builder
	bs.build = NewBuilder(
		noopMetrics,
		bs.db,
		bs.state,
		bs.headerDB,
		bs.sealDB,
		bs.indexDB,
		bs.guarPool,
		bs.sealPool,
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
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().Empty(bs.remCollIDs, "should not remove any valid guarantees")
}

func (bs *BuilderSuite) TestPayloadGuaranteeDuplicateFinalized() {

	// create some valid guarantees
	valid := unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(bs.finalID))

	// create some duplicate guarantees and add to random finalized block
	duplicated := unittest.CollectionGuaranteesFixture(12, unittest.WithCollRef(bs.finalID))
	for _, guarantee := range duplicated {
		finalizedID := bs.finalizedBlockIDs[rand.Intn(len(bs.finalizedBlockIDs))]
		index := bs.index[finalizedID]
		index.CollectionIDs = append(index.CollectionIDs, guarantee.ID())
		bs.index[finalizedID] = index
	}

	// add sixteen guarantees to the pool
	bs.pendingGuarantees = append(valid, duplicated...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(valid, bs.assembled.Guarantees, "should have valid guarantees from mempool in payload")
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().ElementsMatch(flow.GetIDs(duplicated), bs.remCollIDs, "should remove duplicate finalized guarantees from mempool")

}

func (bs *BuilderSuite) TestPayloadGuaranteeDuplicatePending() {

	// create some valid guarantees
	valid := unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(bs.finalID))

	// create some duplicate guarantees and add to random finalized block
	duplicated := unittest.CollectionGuaranteesFixture(12, unittest.WithCollRef(bs.finalID))
	for _, guarantee := range duplicated {
		pendingID := bs.pendingBlockIDs[rand.Intn(len(bs.pendingBlockIDs))]
		index := bs.index[pendingID]
		index.CollectionIDs = append(index.CollectionIDs, guarantee.ID())
		bs.index[pendingID] = index
	}

	// add sixteen guarantees to the pool
	bs.pendingGuarantees = append(valid, duplicated...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(valid, bs.assembled.Guarantees, "should have valid guarantees from mempool in payload")
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().Empty(bs.remCollIDs, "should not remove duplicate pending guarantees from mempool")
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
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().ElementsMatch(flow.GetIDs(unknown), bs.remCollIDs, "should remove guarantees with unknown reference from mempool")
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
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().ElementsMatch(flow.GetIDs(expired), bs.remCollIDs, "should remove guarantees with expired reference from mempool")
}

func (bs *BuilderSuite) TestPayloadSealAllValid() {

	// use valid chain of seals in mempool
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain, bs.assembled.Seals, "should have included valid chain of seals")
}

// TestPayloadSealOnlyFork verifies that builder only includes seals whose
//
// if an IncorporatedResultSeal is included in mempool, whose final state does not match the execution result
func (bs *BuilderSuite) TestPayloadSealOnlyFork() {
	// in the test setup, we already created a single fork
	//  [first] <- [F0] <- [F1] <- [F2] <- [F3] <- [A0] <- [A1] <- [A2] <- [A3]
	// Where block
	//   * [first] is sealed and finalized
	//   * [F0] ... [F3] are finalized but _not_ sealed
	//   * [A0] ... [A3] are _not_ finalized and _not_ sealed
	// We now create an additional fork:  [F3] <- [B0] <- [B1] <- ... <- [B7]
	var forkHead *flow.Block
	for f := 0; f < 100; f++ {
		forkHead = bs.blocks[bs.finalID]
		for i := 0; i < 8; i++ {
			forkHead = bs.createAndRecordBlock(forkHead)
			// Method createAndRecordBlock adds a seal for every block into the mempool.
		}
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

func (bs *BuilderSuite) TestPayloadSealCutoffChain() {

	// remove the seal at the start
	firstSeal := bs.irsList[0]
	delete(bs.irsMap, firstSeal.ID())
	bs.pendingSeals = bs.irsMap

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should have not included any chains from cutoff chain")
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
