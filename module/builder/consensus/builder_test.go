package consensus

import (
	"math/rand"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mempoolAPIs "github.com/onflow/flow-go/module/mempool"
	mempoolImpl "github.com/onflow/flow-go/module/mempool/consensus"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	realproto "github.com/onflow/flow-go/state/protocol"
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
	firstID           flow.Identifier                               // first block in the range we look at
	finalID           flow.Identifier                               // last finalized block
	parentID          flow.Identifier                               // parent block we build on
	finalizedBlockIDs []flow.Identifier                             // blocks between first and final
	pendingBlockIDs   []flow.Identifier                             // blocks between final and parent
	resultForBlock    map[flow.Identifier]*flow.ExecutionResult     // map: BlockID -> Execution Result
	resultByID        map[flow.Identifier]*flow.ExecutionResult     // map: result ID -> Execution Result
	receiptsByID      map[flow.Identifier]*flow.ExecutionReceipt    // map: receipt ID -> ExecutionReceipt
	receiptsByBlockID map[flow.Identifier]flow.ExecutionReceiptList // map: block ID -> flow.ExecutionReceiptList

	// used to populate and test the seal mempool
	chain   []*flow.Seal                                     // chain of seals starting first
	irsList []*flow.IncorporatedResultSeal                   // chain of IncorporatedResultSeals
	irsMap  map[flow.Identifier]*flow.IncorporatedResultSeal // index for irsList

	// mempools consumed by builder
	pendingGuarantees []*flow.CollectionGuarantee
	pendingReceipts   []*flow.ExecutionReceipt
	pendingSeals      map[flow.Identifier]*flow.IncorporatedResultSeal // storage for the seal mempool

	// storage for dbs
	headers       map[flow.Identifier]*flow.Header
	index         map[flow.Identifier]*flow.Index
	blocks        map[flow.Identifier]*flow.Block
	blockChildren map[flow.Identifier][]flow.Identifier // ids of children blocks

	lastSeal *flow.Seal

	// real dependencies
	dir      string
	db       *badger.DB
	sentinel uint64
	setter   func(*flow.Header) error

	// mocked dependencies
	state      *protocol.MutableState
	headerDB   *storage.Headers
	sealDB     *storage.Seals
	indexDB    *storage.Index
	blockDB    *storage.Blocks
	resultDB   *storage.ExecutionResults
	receiptsDB *storage.ExecutionReceipts

	guarPool *mempool.Guarantees
	sealPool *mempool.IncorporatedResultSeals
	recPool  *mempool.ExecutionTree

	// tracking behaviour
	assembled *flow.Payload // built payload

	// component under test
	build *Builder
}

func (bs *BuilderSuite) storeBlock(block *flow.Block) {
	bs.headers[block.ID()] = block.Header
	bs.blocks[block.ID()] = block
	bs.index[block.ID()] = block.Payload.Index()
	bs.blockChildren[block.Header.ParentID] = append(bs.blockChildren[block.Header.ParentID], block.ID())
	for _, result := range block.Payload.Results {
		bs.resultByID[result.ID()] = result
	}
}

// createAndRecordBlock creates a new block chained to the previous block.
// The new block contains a receipt for a result of the previous
// block, which is also used to create a seal for the previous block. The seal
// and the result are combined in an IncorporatedResultSeal which is a candidate
// for the seals mempool.
func (bs *BuilderSuite) createAndRecordBlock(parentBlock *flow.Block, candidateSealForParent bool) *flow.Block {
	block := unittest.BlockWithParentFixture(parentBlock.Header)

	// Create a receipt for a result of the parentBlock block,
	// and add it to the payload. The corresponding IncorporatedResult will be used to
	// seal the parentBlock, and to create an IncorporatedResultSeal for the seal mempool.
	var incorporatedResultForPrevBlock *flow.IncorporatedResult
	previousResult, found := bs.resultForBlock[parentBlock.ID()]
	if !found {
		panic("missing execution result for parent")
	}
	receipt := unittest.ExecutionReceiptFixture(unittest.WithResult(previousResult))
	block.Payload.Receipts = append(block.Payload.Receipts, receipt.Meta())
	block.Payload.Results = append(block.Payload.Results, &receipt.ExecutionResult)

	incorporatedResultForPrevBlock = unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithResult(previousResult),
		unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
	)

	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(&block),
		unittest.WithPreviousResult(*previousResult),
	)

	bs.resultForBlock[result.BlockID] = result
	bs.resultByID[result.ID()] = result
	bs.receiptsByID[receipt.ID()] = receipt
	bs.receiptsByBlockID[receipt.ExecutionResult.BlockID] = append(bs.receiptsByBlockID[receipt.ExecutionResult.BlockID], receipt)

	// record block in dbs
	bs.storeBlock(&block)

	if candidateSealForParent {
		// seal the parentBlock block with the result included in this block.
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

// SetupTest constructs the following chain of blocks:
//    [first] <- [F0] <- [F1] <- [F2] <- [F3] <- [final] <- [A0] <- [A1] <- [A2] <- [A3] <- [parent]
// Where block
//   * [first] is sealed and finalized
//   * [F0] ... [F4] and [final] are finalized, unsealed blocks with candidate seals are included in mempool
//   * [A0] ... [A2] are non-finalized, unsealed blocks with candidate seals are included in mempool
//   * [A3] and [parent] are non-finalized, unsealed blocks _without_ candidate seals
// Each block incorporates the result for its immediate parent.
//
// Note: In the happy path, the blocks [A3] and [parent] will not have candidate seal for the following reason:
// For the verifiers to start checking a result R, they need a source of randomness for the block _incorporating_
// result R. The result for block [A3] is incorporated in [parent], which does _not_ have a child yet.
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
	bs.receiptsByID = make(map[flow.Identifier]*flow.ExecutionReceipt)
	bs.receiptsByBlockID = make(map[flow.Identifier]flow.ExecutionReceiptList)

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
	//bs.heights = make(map[uint64]*flow.Header)
	bs.index = make(map[flow.Identifier]*flow.Index)
	bs.blocks = make(map[flow.Identifier]*flow.Block)
	bs.blockChildren = make(map[flow.Identifier][]flow.Identifier)

	// initialize behaviour tracking
	bs.assembled = nil

	// Construct the [first] block:
	first := unittest.BlockFixture()
	bs.storeBlock(&first)
	bs.firstID = first.ID()
	firstResult := unittest.ExecutionResultFixture(unittest.WithBlock(&first))
	bs.lastSeal = unittest.Seal.Fixture(unittest.Seal.WithResult(firstResult))
	bs.resultForBlock[firstResult.BlockID] = firstResult
	bs.resultByID[firstResult.ID()] = firstResult

	// Construct finalized blocks [F0] ... [F4]
	previous := &first
	for n := 0; n < numFinalizedBlocks; n++ {
		finalized := bs.createAndRecordBlock(previous, n > 0) // Do not construct candidate seal for [first], as it is already sealed
		bs.finalizedBlockIDs = append(bs.finalizedBlockIDs, finalized.ID())
		previous = finalized
	}

	// Construct the last finalized block [final]
	final := bs.createAndRecordBlock(previous, true)
	bs.finalID = final.ID()

	// Construct the pending (i.e. unfinalized) ancestors [A0], ..., [A3]
	previous = final
	for n := 0; n < numPendingBlocks; n++ {
		pending := bs.createAndRecordBlock(previous, true)
		bs.pendingBlockIDs = append(bs.pendingBlockIDs, pending.ID())
		previous = pending
	}

	// Construct [parent] block; but do _not_ add candidate seal for its parent
	parent := bs.createAndRecordBlock(previous, false)
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
	bs.state.On("Final").Return(func() realproto.Snapshot {
		if block, ok := bs.blocks[bs.finalID]; ok {
			snapshot := unittest.StateSnapshotForKnownBlock(block.Header, nil)
			snapshot.On("ValidDescendants").Return(bs.blockChildren[bs.finalID], nil)
			return snapshot
		}
		return unittest.StateSnapshotForUnknownBlock()
	})

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

	bs.receiptsDB = &storage.ExecutionReceipts{}
	bs.receiptsDB.On("ByID", mock.Anything).Return(
		func(receiptID flow.Identifier) *flow.ExecutionReceipt {
			return bs.receiptsByID[receiptID]
		},
		func(receiptID flow.Identifier) error {
			_, exists := bs.receiptsByID[receiptID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	bs.receiptsDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) flow.ExecutionReceiptList {
			return bs.receiptsByBlockID[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := bs.receiptsByBlockID[blockID]
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
	bs.recPool.On("PruneUpToHeight", mock.Anything).Return(nil).Maybe()
	bs.recPool.On("Size").Return(uint(0)).Maybe() // used for metrics only
	bs.recPool.On("AddResult", mock.Anything, mock.Anything).Return(nil).Maybe()
	bs.recPool.On("AddReceipt", mock.Anything, mock.Anything).Return(false, nil).Maybe()
	bs.recPool.On("ReachableReceipts", mock.Anything, mock.Anything, mock.Anything).Return(
		func(resultID flow.Identifier, blockFilter mempoolAPIs.BlockFilter, receiptFilter mempoolAPIs.ReceiptFilter) []*flow.ExecutionReceipt {
			return bs.pendingReceipts
		},
		nil,
	)

	// initialize the builder
	bs.build, err = NewBuilder(
		noopMetrics,
		bs.db,
		bs.state,
		bs.headerDB,
		bs.sealDB,
		bs.indexDB,
		bs.blockDB,
		bs.resultDB,
		bs.receiptsDB,
		bs.guarPool,
		bs.sealPool,
		bs.recPool,
		noopTracer,
	)
	require.NoError(bs.T(), err)

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

// TestPayloadSeals_AllValid checks that builder seals as many blocks as possible (happy path):
//    [first] <- [F0] <- [F1] <- [F2] <- [F3] <- [final] <- [A0] <- [A1] <- [A2] <- [A3] <- [parent]
// Where block
//   * [first] is sealed and finalized
//   * [F0] ... [F4] and [final] are finalized, unsealed blocks with candidate seals are included in mempool
//   * [A0] ... [A2] are non-finalized, unsealed blocks with candidate seals are included in mempool
//   * [A3] and [parent] are non-finalized, unsealed blocks _without_ candidate seals
// Expected behaviour:
//  * builder should include seals [F0], ..., [A4]
//  * note: Block [A3] will not have a seal in the happy path for the following reason:
//    In our example, the result for block A3 is incorporated in block A4. But, for the verifiers to start
//    their work, they need a child block of A4, because the child contains the source of randomness for
//    A4. But we are just constructing this child right now. Hence, the verifiers couldn't have checked
//    the result for A3.
func (bs *BuilderSuite) TestPayloadSeals_AllValid() {
	// populate seals mempool with valid chain of seals for blocks [F0], ..., [A2]
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain, bs.assembled.Seals, "should have included valid chain of seals")
}

// TestPayloadSeals_Limit verifies that builder does not exceed  maxSealLimit
func (bs *BuilderSuite) TestPayloadSeals_Limit() {
	// use valid chain of seals in mempool
	bs.pendingSeals = bs.irsMap

	// change maxSealCount to one less than the number of items in the mempool
	limit := uint(2)
	bs.build.cfg.maxSealCount = limit

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Equal(bs.chain[:limit], bs.assembled.Seals, "should have excluded seals above maxSealCount")
}

// TestPayloadSeals_OnlyFork checks that the builder only includes seals corresponding
// to blocks on the current fork (and _not_ seals for sealable blocks on other forks)
func (bs *BuilderSuite) TestPayloadSeals_OnlyFork() {
	// in the test setup, we already created a single fork
	//    [first] <- [F0] <- [F1] <- [F2] <- [F3] <- [final] <- [A0] <- [A1] <- [A2] ..
	// For this test, we add fork:                        ^
	//                                                    └--- [B0] <- [B1] <- ....<- [B6] <- [B7]
	// Where block
	//   * [first] is sealed and finalized
	//   * [F0] ... [F4] and [final] are finalized, unsealed blocks with candidate seals are included in mempool
	//   * [A0] ... [A2] are non-finalized, unsealed blocks with candidate seals are included in mempool
	forkHead := bs.blocks[bs.finalID]
	for i := 0; i < 8; i++ {
		// Usually, the blocks [B6] and [B7] will not have candidate seal for the following reason:
		// For the verifiers to start checking a result R, they need a source of randomness for the block _incorporating_
		// result R. The result for block [B6] is incorporated in [B7], which does _not_ have a child yet.
		forkHead = bs.createAndRecordBlock(forkHead, i < 6)
	}

	bs.pendingSeals = bs.irsMap
	_, err := bs.build.BuildOn(forkHead.ID(), bs.setter)
	bs.Require().NoError(err)

	// expected seals: [F0] <- ... <- [final] <- [B0] <- ... <- [B5]
	// Note: bs.chain contains seals for blocks [F0]...[A2] followed by seals for [final], [B0]...[B5]
	bs.Assert().Equal(10, len(bs.assembled.Seals), "unexpected number of seals")
	bs.Assert().ElementsMatch(bs.chain[:4], bs.assembled.Seals[:4], "should have included only valid chain of seals")
	bs.Assert().ElementsMatch(bs.chain[8:], bs.assembled.Seals[4:], "should have included only valid chain of seals")

	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
}

// TestPayloadSeals_EnforceGap checks that builder leaves a 1-block gap between block incorporating the result
// and the block sealing the result. Without this gap, some nodes might not be able to compute the Verifier
// assignment for the seal and therefore reject the block. This edge case only occurs in a very specific situation:
//
//                                                                        ┌---- [A5] (orphaned fork)
//                                                                        v
// ...<- [B0] <- [B1] <- [B2] <- [B3] <- [B4{incorporates result R for B1}] <- ░newBlock░
//
// SCENARIO:
// * block B0 is sealed
// Proposer for ░newBlock░:
//  * Knows block A5. Hence, it knows a QC for block B4, which contains the Source Of Randomness (SOR) for B4.
//    Therefore, the proposer can construct the verifier assignment for [B4{incorporates result R for B1}]
//  * Assume that verification was fast enough, so the proposer has sufficient approvals for result R.
//    Therefore, the proposer has a candidate seal, sealing result R for block B4, in its mempool.
// Replica trying to verify ░newBlock░:
//  * Assume that the replica does _not_ know A5. Therefore, it _cannot_ compute the verifier assignment for B4.
//
// Problem:  If the proposer included the seal for B1, the replica could not check it.
// Solution: There must be a gap between the block incorporating the result (here B4) and
//           the block sealing the result. A gap of one block is sufficient.
//                                                                        ┌---- [A5] (orphaned fork)
//                                                                        v
// ...<- [B0] <- [B1] <- [B2] <- [B3] <- [B4{incorporates result R for B1}] <- [B5] <- [B6{seals B1}]
//                                                                            ~~~~~~
//                                                                             gap
// We test the two distinct cases:
//   (i) Builder does _not_ include seal for B1 when constructing block B5
//  (ii) Builder _includes_ seal for B1 when constructing block B6
func (bs *BuilderSuite) TestPayloadSeals_EnforceGap() {
	// we use bs.parentID as block B0
	b0result := bs.resultForBlock[bs.parentID]
	b0seal := unittest.Seal.Fixture(unittest.Seal.WithResult(b0result))

	// create blocks B1 to B4:
	b1 := bs.createAndRecordBlock(bs.blocks[bs.parentID], true)
	bchain := unittest.ChainFixtureFrom(3, b1.Header) // creates blocks b2, b3, b4
	b4 := bchain[2]

	// Incorporate result for block B1 into payload of block B4
	resultB1 := bs.resultForBlock[b1.ID()]
	receiptB1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultB1))
	b4.SetPayload(
		flow.Payload{
			Results:  []*flow.ExecutionResult{&receiptB1.ExecutionResult},
			Receipts: []*flow.ExecutionReceiptMeta{receiptB1.Meta()},
		})

	// add blocks B2, B3, B4, A5 to the mocked storage layer (block b0 and b1 are already added):
	a5 := unittest.BlockWithParentFixture(b4.Header)
	for _, b := range append(bchain, &a5) {
		bs.storeBlock(b)
	}

	// mock for of candidate seal mempool:
	bs.pendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
	b1seal := storeSealForIncorporatedResult(resultB1, b4.ID(), bs.pendingSeals)

	// mock for seals storage layer:
	bs.sealDB = &storage.Seals{}
	bs.build.seals = bs.sealDB

	bs.T().Run("Build on top of B4 and check that no seals are included", func(t *testing.T) {
		bs.sealDB.On("ByBlockID", b4.ID()).Return(b0seal, nil)

		_, err := bs.build.BuildOn(b4.ID(), bs.setter)
		require.NoError(t, err)
		bs.recPool.AssertExpectations(t)
		require.Empty(t, bs.assembled.Seals, "should not include any seals")
	})

	bs.T().Run("Build on top of B5 and check that seals for B1 is included", func(t *testing.T) {
		b5 := unittest.BlockWithParentFixture(b4.Header) // creating block b5
		bs.storeBlock(&b5)
		bs.sealDB.On("ByBlockID", b5.ID()).Return(b0seal, nil)

		_, err := bs.build.BuildOn(b5.ID(), bs.setter)
		require.NoError(t, err)
		bs.recPool.AssertExpectations(t)
		require.Equal(t, 1, len(bs.assembled.Seals), "only seal for B1 expected")
		require.Equal(t, b1seal.Seal, bs.assembled.Seals[0])
	})
}

// TestPayloadSeals_Duplicates verifies that the builder does not duplicate seals for already sealed blocks:
//  ... <- [F0] <- [F1] <- [F2] <- [F3] <- [A0] <- [A1] <- [A2] <- [A3]
// Where block
//   * [F0] ... [F3] sealed blocks but their candidate seals are still included in mempool
//   * [A0] ... [A3] unsealed blocks with candidate seals are included in mempool
// Expected behaviour:
//  * builder should only include seals [A0], ..., [A3]
func (bs *BuilderSuite) TestPayloadSeals_Duplicate() {
	// pretend that the first n blocks are already sealed
	n := 4
	lastSeal := bs.chain[n-1]
	mockSealDB := &storage.Seals{}
	mockSealDB.On("ByBlockID", mock.Anything).Return(lastSeal, nil)
	bs.build.seals = mockSealDB

	// seals for all blocks [F0], ..., [A3] are still in the mempool:
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.chain[n:], bs.assembled.Seals, "should have rejected duplicate seals")
}

// TestPayloadSeals_MissingNextSeal checks how the builder handles the fork
//    [S] <- [F0] <- [F1] <- [F2] <- [F3] <- [A0] <- [A1] <- [A2] <- [A3]
// Where block
//   * [S] is sealed and finalized
//   * [F0] finalized, unsealed block but _without_ candidate seal in mempool
//   * [F1] ... [F3] are finalized, unsealed blocks with candidate seals are included in mempool
//   * [A0] ... [A3] non-finalized, unsealed blocks with candidate seals are included in mempool
// Expected behaviour:
//  * builder should not include any seals as the immediately next seal is not in mempool
func (bs *BuilderSuite) TestPayloadSeals_MissingNextSeal() {
	// remove the seal for block [F0]
	firstSeal := bs.irsList[0]
	delete(bs.irsMap, firstSeal.ID())
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should not have included any seals from cutoff chain")
}

// TestPayloadSeals_MissingInterimSeal checks how the builder handles the fork
//   [S] <- [F0] <- [F1] <- [F2] <- [F3] <- [A0] <- [A1] <- [A2] <- [A3]
// Where block
//   * [S] is sealed and finalized
//   * [F0] ... [F2] are finalized, unsealed blocks with candidate seals are included in mempool
//   * [F4] finalized, unsealed block but _without_ candidate seal in mempool
//   * [A0] ... [A3] non-finalized, unsealed blocks with candidate seals are included in mempool
// Expected behaviour:
//  * builder should only include candidate seals for [F0], [F1], [F2]
func (bs *BuilderSuite) TestPayloadSeals_MissingInterimSeal() {
	// remove a seal for block [F4]
	seal := bs.irsList[3]
	delete(bs.irsMap, seal.ID())
	bs.pendingSeals = bs.irsMap

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain[:3], bs.assembled.Seals, "should have included only beginning of broken chain")
}

// TestValidatePayloadSeals_ExecutionForks checks how the builder's seal-inclusion logic
// handles execution forks.
//  * we have the chain in storage:
//     F <- A{Result[F]_1, Result[F]_2, ReceiptMeta[F]_1, ReceiptMeta[F]_2}
//           <- B{Result[A]_1, Result[A]_2, ReceiptMeta[A]_1, ReceiptMeta[A]_2}
//             <- C{Result[B]_1, Result[B]_2, ReceiptMeta[B]_1, ReceiptMeta[B]_2}
//                 <- D{Seal for Result[F]_1}
//     here F is the latest finalized block (with ID bs.finalID)
//  * Note that we are explicitly testing the handling of an execution fork that
//    was incorporated _before_ the seal
//       Blocks:      F  <-----------   A    <-----------   B
//      Results:   Result[F]_1  <-  Result[A]_1  <-  Result[B]_1 :: the root of this execution tree is sealed
//                 Result[F]_2  <-  Result[A]_2  <-  Result[B]_2 :: the root of this execution tree conflicts with sealed result
// The builder is tasked with creating the payload for block X:
//     F <- A{..} <- B{..} <- C{..} <- D{..} <- X
// We test the two distinct cases:
//   (i) verify that execution fork conflicting with sealed result is not sealed
//  (ii) verify that multiple execution forks are properly handled
func (bs *BuilderSuite) TestValidatePayloadSeals_ExecutionForks() {
	bs.build.cfg.expiry = 4 // reduce expiry so collection dedup algorithm doesn't walk past  [lastSeal]

	blockF := bs.blocks[bs.finalID]
	blocks := []*flow.Block{blockF}
	blocks = append(blocks, unittest.ChainFixtureFrom(4, blockF.Header)...)              // elements  [F, A, B, C, D]
	receiptChain1 := unittest.ReceiptChainFor(blocks, unittest.ExecutionResultFixture()) // elements  [Result[F]_1, Result[A]_1, Result[B]_1, ...]
	receiptChain2 := unittest.ReceiptChainFor(blocks, unittest.ExecutionResultFixture()) // elements  [Result[F]_2, Result[A]_2, Result[B]_2, ...]

	for i := 1; i <= 3; i++ { // set payload for blocks A, B, C
		blocks[i].SetPayload(flow.Payload{
			Results:  []*flow.ExecutionResult{&receiptChain1[i-1].ExecutionResult, &receiptChain2[i-1].ExecutionResult},
			Receipts: []*flow.ExecutionReceiptMeta{receiptChain1[i-1].Meta(), receiptChain2[i-1].Meta()},
		})
	}
	sealedResult := receiptChain1[0].ExecutionResult
	sealF := unittest.Seal.Fixture(unittest.Seal.WithResult(&sealedResult))
	blocks[4].SetPayload(flow.Payload{ // set payload for block D
		Seals: []*flow.Seal{sealF},
	})
	for i := 0; i <= 4; i++ {
		// we need to run this several times, as in each iteration as we have _multiple_ execution chains.
		// In each iteration, we only mange to reconnect one additional height
		unittest.ReconnectBlocksAndReceipts(blocks, receiptChain1)
		unittest.ReconnectBlocksAndReceipts(blocks, receiptChain2)
	}

	for _, b := range blocks {
		bs.storeBlock(b)
	}
	bs.sealDB = &storage.Seals{}
	bs.build.seals = bs.sealDB
	bs.sealDB.On("ByBlockID", mock.Anything).Return(sealF, nil)
	bs.resultByID[sealedResult.ID()] = &sealedResult

	bs.T().Run("verify that execution fork conflicting with sealed result is not sealed", func(t *testing.T) {
		bs.pendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
		storeSealForIncorporatedResult(&receiptChain2[1].ExecutionResult, blocks[2].ID(), bs.pendingSeals)

		_, err := bs.build.BuildOn(blocks[4].ID(), bs.setter)
		require.NoError(t, err)
		require.Empty(t, bs.assembled.Seals, "should not have included seal for conflicting execution fork")
	})

	bs.T().Run("verify that multiple execution forks are properly handled", func(t *testing.T) {
		bs.pendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
		sealResultA_1 := storeSealForIncorporatedResult(&receiptChain1[1].ExecutionResult, blocks[2].ID(), bs.pendingSeals)
		sealResultB_1 := storeSealForIncorporatedResult(&receiptChain1[2].ExecutionResult, blocks[3].ID(), bs.pendingSeals)
		storeSealForIncorporatedResult(&receiptChain2[1].ExecutionResult, blocks[2].ID(), bs.pendingSeals)
		storeSealForIncorporatedResult(&receiptChain2[2].ExecutionResult, blocks[3].ID(), bs.pendingSeals)

		_, err := bs.build.BuildOn(blocks[4].ID(), bs.setter)
		require.NoError(t, err)
		require.ElementsMatch(t, []*flow.Seal{sealResultA_1.Seal, sealResultB_1.Seal}, bs.assembled.Seals, "valid fork should have been sealed")
	})
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
	x0 := bs.createAndRecordBlock(bs.blocks[bs.finalID], true)
	x1 := bs.createAndRecordBlock(x0, true)

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
	bs.recPool.On("ReachableReceipts", bs.lastSeal.ResultID, mock.Anything, mock.Anything).Return([]*flow.ExecutionReceipt{}, nil).Once()
	_, err := bs.build.BuildOn(x0.ID(), bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())

	// building on top of X1: latest finalized block in fork is [F4]; expect search to start with sealed result
	bs.sealDB.On("ByBlockID", x1.ID()).Return(f4Seal, nil)
	bs.recPool.On("ReachableReceipts", f4Seal.ResultID, mock.Anything, mock.Anything).Return([]*flow.ExecutionReceipt{}, nil).Once()
	_, err = bs.build.BuildOn(x1.ID(), bs.setter)
	bs.Require().NoError(err)
	bs.recPool.AssertExpectations(bs.T())

	// building on top of A3 (with ID bs.parentID): latest finalized block in fork is [F4]; expect search to start with sealed result
	bs.sealDB.On("ByBlockID", bs.parentID).Return(f2eal, nil)
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
	b1 := bs.createAndRecordBlock(bs.blocks[bs.finalID], true)
	b2 := bs.createAndRecordBlock(b1, true)
	b3 := bs.createAndRecordBlock(b2, true)
	b4 := bs.createAndRecordBlock(b3, true)
	b5 := bs.createAndRecordBlock(b4, true)

	x1 := bs.createAndRecordBlock(bs.blocks[bs.finalID], true)
	y2 := bs.createAndRecordBlock(b1, true)
	a6 := bs.createAndRecordBlock(b5, true)

	c3 := bs.createAndRecordBlock(b2, true)
	c4 := bs.createAndRecordBlock(c3, true)
	d4 := bs.createAndRecordBlock(c3, true)

	// set last sealed blocks:
	b1Seal := unittest.Seal.Fixture(unittest.Seal.WithResult(bs.resultForBlock[b1.ID()]))
	bs.sealDB = &storage.Seals{}
	bs.sealDB.On("ByBlockID", b5.ID()).Return(b1Seal, nil)
	bs.build.seals = bs.sealDB

	// setup mock to test the BlockFilter provided by Builder
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
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
	bs.recPool.On("ReachableReceipts", bs.lastSeal.ResultID, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			receiptFilter := args[2].(mempoolAPIs.ReceiptFilter)
			// verify that all receipts already included in blocks are filtered out:
			for _, block := range bs.blocks {
				resultByID := block.Payload.Results.Lookup()
				for _, meta := range block.Payload.Receipts {
					result := resultByID[meta.ResultID]
					rcpt := flow.ExecutionReceiptFromMeta(*meta, *result)
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
	metas := []*flow.ExecutionReceiptMeta{}
	expectedResults := []*flow.ExecutionResult{}
	var i uint64
	for i = 0; i < 5; i++ {
		blockOnFork := bs.blocks[bs.irsList[i].Seal.BlockID]
		pendingReceipt := unittest.ReceiptForBlockFixture(blockOnFork)
		receipts = append(receipts, pendingReceipt)
		metas = append(metas, pendingReceipt.Meta())
		expectedResults = append(expectedResults, &pendingReceipt.ExecutionResult)
	}
	bs.pendingReceipts = receipts

	// set maxReceiptCount to 3
	var limit uint = 3
	bs.build.cfg.maxReceiptCount = limit

	// ensure that only 3 of the 5 receipts were included
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(metas[:limit], bs.assembled.Receipts, "should have excluded receipts above maxReceiptCount")
	bs.Assert().ElementsMatch(expectedResults[:limit], bs.assembled.Results, "should have excluded results above maxReceiptCount")
}

// TestPayloadReceipts_AsProvidedByReceiptForest tests the receipt selection.
// Expectation: Builder should embed the Receipts as provided by the ExecutionTree
func (bs *BuilderSuite) TestPayloadReceipts_AsProvidedByReceiptForest() {
	var expectedReceipts []*flow.ExecutionReceipt
	var expectedMetas []*flow.ExecutionReceiptMeta
	var expectedResults []*flow.ExecutionResult
	for i := 0; i < 10; i++ {
		expectedReceipts = append(expectedReceipts, unittest.ExecutionReceiptFixture())
		expectedMetas = append(expectedMetas, expectedReceipts[i].Meta())
		expectedResults = append(expectedResults, &expectedReceipts[i].ExecutionResult)
	}
	bs.recPool = &mempool.ExecutionTree{}
	bs.recPool.On("Size").Return(uint(0)).Maybe()
	bs.recPool.On("AddResult", mock.Anything, mock.Anything).Return(nil).Maybe()
	bs.recPool.On("ReachableReceipts", mock.Anything, mock.Anything, mock.Anything).Return(expectedReceipts, nil).Once()
	bs.build.recPool = bs.recPool

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(expectedMetas, bs.assembled.Receipts, "should include receipts as returned by ExecutionTree")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "should include results as returned by ExecutionTree")
	bs.recPool.AssertExpectations(bs.T())
}

// TestIntegration_PayloadReceiptNoParentResult is a mini-integration test combining the
// Builder with a full ExecutionTree mempool. We check that the builder does not include
// receipts whose PreviousResult is not already incorporated in the chain.
//
// Here we create 4 consecutive blocks S, A, B, and C, where A contains a valid
// receipt for block S, but blocks B and C have empty payloads.
//
// We populate the mempool with valid receipts for blocks A, and C, but NOT for
// block B.
//
// The expected behaviour is that the builder should not include the receipt for
// block C, because the chain and the mempool do not contain a valid receipt for
// the parent result (block B's result).
//
// ... <- S[ER{parent}] <- A[ER{S}] <- B <- C <- X (candidate)
func (bs *BuilderSuite) TestIntegration_PayloadReceiptNoParentResult() {
	// make blocks S, A, B, C
	parentReceipt := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.parentID]))
	blockSABC := unittest.ChainFixtureFrom(4, bs.blocks[bs.parentID].Header)
	resultS := unittest.ExecutionResultFixture(unittest.WithBlock(blockSABC[0]), unittest.WithPreviousResult(*bs.resultForBlock[bs.parentID]))
	receiptSABC := unittest.ReceiptChainFor(blockSABC, resultS)
	blockSABC[0].Payload.Receipts = []*flow.ExecutionReceiptMeta{parentReceipt.Meta()}
	blockSABC[0].Payload.Results = []*flow.ExecutionResult{&parentReceipt.ExecutionResult}
	blockSABC[1].Payload.Receipts = []*flow.ExecutionReceiptMeta{receiptSABC[0].Meta()}
	blockSABC[1].Payload.Results = []*flow.ExecutionResult{&receiptSABC[0].ExecutionResult}
	blockSABC[2].Payload.Receipts = []*flow.ExecutionReceiptMeta{}
	blockSABC[3].Payload.Receipts = []*flow.ExecutionReceiptMeta{}
	unittest.ReconnectBlocksAndReceipts(blockSABC, receiptSABC) // update block header so that blocks are chained together

	bs.storeBlock(blockSABC[0])
	bs.storeBlock(blockSABC[1])
	bs.storeBlock(blockSABC[2])
	bs.storeBlock(blockSABC[3])

	// Instantiate real Execution Tree mempool;
	bs.build.recPool = mempoolImpl.NewExecutionTree()
	for _, block := range bs.blocks {
		resultByID := block.Payload.Results.Lookup()
		for _, meta := range block.Payload.Receipts {
			result := resultByID[meta.ResultID]
			rcpt := flow.ExecutionReceiptFromMeta(*meta, *result)
			_, err := bs.build.recPool.AddReceipt(rcpt, bs.blocks[rcpt.ExecutionResult.BlockID].Header)
			bs.NoError(err)
		}
	}
	// for receipts _not_ included in blocks, add only receipt for A and C but NOT B
	_, _ = bs.build.recPool.AddReceipt(receiptSABC[1], blockSABC[1].Header)
	_, _ = bs.build.recPool.AddReceipt(receiptSABC[3], blockSABC[3].Header)

	_, err := bs.build.BuildOn(blockSABC[3].ID(), bs.setter)
	bs.Require().NoError(err)
	expectedReceipts := flow.ExecutionReceiptMetaList{receiptSABC[1].Meta()}
	expectedResults := flow.ExecutionResultList{&receiptSABC[1].ExecutionResult}
	bs.Assert().Equal(expectedReceipts, bs.assembled.Receipts, "payload should contain only receipt for block a")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "payload should contain only result for block a")
}

// TestIntegration_ExtendDifferentExecutionPathsOnSameFork tests that the
// builder includes receipts that form different valid execution paths contained
// on the current fork.
//
//                                         candidate
// P <- A[ER{P}] <- B[ER{A}, ER{A}'] <- X[ER{B}, ER{B}']
func (bs *BuilderSuite) TestIntegration_ExtendDifferentExecutionPathsOnSameFork() {

	// A is a block containing a valid receipt for block P
	recP := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.parentID]))
	A := unittest.BlockWithParentFixture(bs.headers[bs.parentID])
	A.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recP.Meta()},
		Results:  []*flow.ExecutionResult{&recP.ExecutionResult},
	})

	// B is a block containing two valid receipts, with different results, for
	// block A
	resA1 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA1))
	resA2 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA2))
	B := unittest.BlockWithParentFixture(A.Header)
	B.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recA1.Meta(), recA2.Meta()},
		Results:  []*flow.ExecutionResult{&recA1.ExecutionResult, &recA2.ExecutionResult},
	})

	bs.storeBlock(&A)
	bs.storeBlock(&B)

	// Instantiate real Execution Tree mempool;
	bs.build.recPool = mempoolImpl.NewExecutionTree()
	for _, block := range bs.blocks {
		resultByID := block.Payload.Results.Lookup()
		for _, meta := range block.Payload.Receipts {
			result := resultByID[meta.ResultID]
			rcpt := flow.ExecutionReceiptFromMeta(*meta, *result)
			_, err := bs.build.recPool.AddReceipt(rcpt, bs.blocks[rcpt.ExecutionResult.BlockID].Header)
			bs.NoError(err)
		}
	}

	// Create two valid receipts for block B which build on different receipts
	// for the parent block (A); recB1 builds on top of RecA1, whilst recB2
	// builds on top of RecA2.
	resB1 := unittest.ExecutionResultFixture(unittest.WithBlock(&B), unittest.WithPreviousResult(recA1.ExecutionResult))
	recB1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resB1))
	resB2 := unittest.ExecutionResultFixture(unittest.WithBlock(&B), unittest.WithPreviousResult(recA2.ExecutionResult))
	recB2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resB2))

	// Add recB1 and recB2 to the mempool for inclusion in the next candidate
	_, _ = bs.build.recPool.AddReceipt(recB1, B.Header)
	_, _ = bs.build.recPool.AddReceipt(recB2, B.Header)

	_, err := bs.build.BuildOn(B.ID(), bs.setter)
	bs.Require().NoError(err)
	expectedReceipts := flow.ExecutionReceiptMetaList{recB1.Meta(), recB2.Meta()}
	expectedResults := flow.ExecutionResultList{&recB1.ExecutionResult, &recB2.ExecutionResult}
	bs.Assert().Equal(expectedReceipts, bs.assembled.Receipts, "payload should contain receipts from valid execution forks")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "payload should contain results from valid execution forks")
}

// TestIntegration_ExtendDifferentExecutionPathsOnDifferentForks tests that the
// builder picks up receipts that were already included in a different fork.
//
//                                   candidate
// P <- A[ER{P}] <- B[ER{A}] <- X[ER{A}',ER{B}, ER{B}']
//                |
//                < ------ C[ER{A}']
//
// Where:
// - ER{A} and ER{A}' are receipts for block A that don't have the same
//   result.
// - ER{B} is a receipt for B with parent result ER{A}
// - ER{B}' is a receipt for B with parent result ER{A}'
//
// ER{P} <- ER{A}  <- ER{B}
//        |
//        < ER{A}' <- ER{B}'
//
// When buiding on top of B, we expect the candidate payload to contain ER{A}',
// ER{B}, and ER{B}'
func (bs *BuilderSuite) TestIntegration_ExtendDifferentExecutionPathsOnDifferentForks() {
	// A is a block containing a valid receipt for block P
	recP := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.parentID]))
	A := unittest.BlockWithParentFixture(bs.headers[bs.parentID])
	A.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recP.Meta()},
		Results:  []*flow.ExecutionResult{&recP.ExecutionResult},
	})

	// B is a block that builds on A containing a valid receipt for A
	resA1 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA1))
	B := unittest.BlockWithParentFixture(A.Header)
	B.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recA1.Meta()},
		Results:  []*flow.ExecutionResult{&recA1.ExecutionResult},
	})

	// C is another block that builds on A containing a valid receipt for A but
	// different from the receipt contained in B
	resA2 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA2))
	C := unittest.BlockWithParentFixture(A.Header)
	C.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recA2.Meta()},
		Results:  []*flow.ExecutionResult{&recA2.ExecutionResult},
	})

	bs.storeBlock(&A)
	bs.storeBlock(&B)
	bs.storeBlock(&C)

	// Instantiate real Execution Tree mempool;
	bs.build.recPool = mempoolImpl.NewExecutionTree()
	for _, block := range bs.blocks {
		resultByID := block.Payload.Results.Lookup()
		for _, meta := range block.Payload.Receipts {
			result := resultByID[meta.ResultID]
			rcpt := flow.ExecutionReceiptFromMeta(*meta, *result)
			_, err := bs.build.recPool.AddReceipt(rcpt, bs.blocks[rcpt.ExecutionResult.BlockID].Header)
			bs.NoError(err)
		}
	}

	// create and add a receipt for block B which builds on top of recA2, which
	// is not on the same execution fork
	resB1 := unittest.ExecutionResultFixture(unittest.WithBlock(&B), unittest.WithPreviousResult(recA1.ExecutionResult))
	recB1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resB1))
	resB2 := unittest.ExecutionResultFixture(unittest.WithBlock(&B), unittest.WithPreviousResult(recA2.ExecutionResult))
	recB2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resB2))

	_, err := bs.build.recPool.AddReceipt(recB1, B.Header)
	bs.Require().NoError(err)
	_, err = bs.build.recPool.AddReceipt(recB2, B.Header)
	bs.Require().NoError(err)

	_, err = bs.build.BuildOn(B.ID(), bs.setter)
	bs.Require().NoError(err)
	expectedReceipts := []*flow.ExecutionReceiptMeta{recA2.Meta(), recB1.Meta(), recB2.Meta()}
	expectedResults := []*flow.ExecutionResult{&recA2.ExecutionResult, &recB1.ExecutionResult, &recB2.ExecutionResult}
	bs.Assert().ElementsMatch(expectedReceipts, bs.assembled.Receipts, "builder should extend different execution paths")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "builder should extend different execution paths")
}

// TestIntegration_DuplicateReceipts checks that the builder does not re-include
// receipts that are already incorporated in blocks on the fork.
//
//
// P <- A(r_P) <- B(r_A) <- X (candidate)
func (bs *BuilderSuite) TestIntegration_DuplicateReceipts() {
	// A is a block containing a valid receipt for block P
	recP := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.parentID]))
	A := unittest.BlockWithParentFixture(bs.headers[bs.parentID])
	A.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recP.Meta()},
		Results:  []*flow.ExecutionResult{&recP.ExecutionResult},
	})

	// B is a block that builds on A containing a valid receipt for A
	resA1 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA1))
	B := unittest.BlockWithParentFixture(A.Header)
	B.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recA1.Meta()},
		Results:  []*flow.ExecutionResult{&recA1.ExecutionResult},
	})

	bs.storeBlock(&A)
	bs.storeBlock(&B)

	// Instantiate real Execution Tree mempool;
	bs.build.recPool = mempoolImpl.NewExecutionTree()
	for _, block := range bs.blocks {
		resultByID := block.Payload.Results.Lookup()
		for _, meta := range block.Payload.Receipts {
			result := resultByID[meta.ResultID]
			rcpt := flow.ExecutionReceiptFromMeta(*meta, *result)
			_, err := bs.build.recPool.AddReceipt(rcpt, bs.blocks[rcpt.ExecutionResult.BlockID].Header)
			bs.NoError(err)
		}
	}

	_, err := bs.build.BuildOn(B.ID(), bs.setter)
	bs.Require().NoError(err)
	expectedReceipts := []*flow.ExecutionReceiptMeta{}
	expectedResults := []*flow.ExecutionResult{}
	bs.Assert().ElementsMatch(expectedReceipts, bs.assembled.Receipts, "builder should not include receipts that are already incorporated in the current fork")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "builder should not include results that were already incorporated")
}

// TestIntegration_ResultAlreadyIncorporated checks that the builder includes
// receipts for results that were already incorporated in blocks on the fork.
//
//
// P <- A(ER[P]) <- X (candidate)
func (bs *BuilderSuite) TestIntegration_ResultAlreadyIncorporated() {
	// A is a block containing a valid receipt for block P
	recP := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.parentID]))
	A := unittest.BlockWithParentFixture(bs.headers[bs.parentID])
	A.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recP.Meta()},
		Results:  []*flow.ExecutionResult{&recP.ExecutionResult},
	})

	recP_B := unittest.ExecutionReceiptFixture(unittest.WithResult(&recP.ExecutionResult))

	bs.storeBlock(&A)

	// Instantiate real Execution Tree mempool;
	bs.build.recPool = mempoolImpl.NewExecutionTree()
	for _, block := range bs.blocks {
		resultByID := block.Payload.Results.Lookup()
		for _, meta := range block.Payload.Receipts {
			result := resultByID[meta.ResultID]
			rcpt := flow.ExecutionReceiptFromMeta(*meta, *result)
			_, err := bs.build.recPool.AddReceipt(rcpt, bs.blocks[rcpt.ExecutionResult.BlockID].Header)
			bs.NoError(err)
		}
	}

	_, err := bs.build.recPool.AddReceipt(recP_B, bs.blocks[recP_B.ExecutionResult.BlockID].Header)
	bs.NoError(err)

	_, err = bs.build.BuildOn(A.ID(), bs.setter)
	bs.Require().NoError(err)
	expectedReceipts := []*flow.ExecutionReceiptMeta{recP_B.Meta()}
	expectedResults := []*flow.ExecutionResult{}
	bs.Assert().ElementsMatch(expectedReceipts, bs.assembled.Receipts, "builder should include receipt metas for results that were already incorporated")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "builder should not include results that were already incorporated")
}

func storeSealForIncorporatedResult(result *flow.ExecutionResult, incorporatingBlockID flow.Identifier, pendingSeals map[flow.Identifier]*flow.IncorporatedResultSeal) *flow.IncorporatedResultSeal {
	incorporatedResultSeal := unittest.IncorporatedResultSeal.Fixture(
		unittest.IncorporatedResultSeal.WithResult(result),
		unittest.IncorporatedResultSeal.WithIncorporatedBlockID(incorporatingBlockID),
	)
	pendingSeals[incorporatedResultSeal.ID()] = incorporatedResultSeal
	return incorporatedResultSeal
}

// TestIntegration_RepopulateExecutionTreeAtStartup tests that the
// builder includes receipts for candidate block after fresh start, meaning
// it will repopulate execution tree in constructor
// P <- A[ER{P}] <- B[ER{A}, ER{A}'] <- C <- X[ER{B}, ER{B}', ER{C} ]
//        |
//     finalized
func (bs *BuilderSuite) TestIntegration_RepopulateExecutionTreeAtStartup() {
	// setup initial state
	// A is a block containing a valid receipt for block P
	recP := unittest.ExecutionReceiptFixture(unittest.WithResult(bs.resultForBlock[bs.parentID]))
	A := unittest.BlockWithParentFixture(bs.headers[bs.parentID])
	A.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recP.Meta()},
		Results:  []*flow.ExecutionResult{&recP.ExecutionResult},
	})

	// B is a block containing two valid receipts, with different results, for
	// block A
	resA1 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA1))
	resA2 := unittest.ExecutionResultFixture(unittest.WithBlock(&A), unittest.WithPreviousResult(recP.ExecutionResult))
	recA2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resA2))
	B := unittest.BlockWithParentFixture(A.Header)
	B.SetPayload(flow.Payload{
		Receipts: []*flow.ExecutionReceiptMeta{recA1.Meta(), recA2.Meta()},
		Results:  []*flow.ExecutionResult{&recA1.ExecutionResult, &recA2.ExecutionResult},
	})

	C := unittest.BlockWithParentFixture(B.Header)

	bs.storeBlock(&A)
	bs.storeBlock(&B)
	bs.storeBlock(&C)

	// store execution results
	for _, block := range []*flow.Block{&A, &B, &C} {
		// for current block create empty receipts list
		bs.receiptsByBlockID[block.ID()] = flow.ExecutionReceiptList{}

		for _, result := range block.Payload.Results {
			bs.resultByID[result.ID()] = result
		}
		for _, meta := range block.Payload.Receipts {
			receipt := flow.ExecutionReceiptFromMeta(*meta, *bs.resultByID[meta.ResultID])
			bs.receiptsByID[meta.ID()] = receipt
			bs.receiptsByBlockID[receipt.ExecutionResult.BlockID] = append(bs.receiptsByBlockID[receipt.ExecutionResult.BlockID], receipt)
		}
	}

	// mark A as finalized
	bs.finalID = A.ID()

	// set up no-op dependencies
	noopMetrics := metrics.NewNoopCollector()
	noopTracer := trace.NewNoopTracer()

	// Instantiate real Execution Tree mempool;
	recPool := mempoolImpl.NewExecutionTree()

	// create builder which has to repopulate execution tree
	var err error
	bs.build, err = NewBuilder(
		noopMetrics,
		bs.db,
		bs.state,
		bs.headerDB,
		bs.sealDB,
		bs.indexDB,
		bs.blockDB,
		bs.resultDB,
		bs.receiptsDB,
		bs.guarPool,
		bs.sealPool,
		recPool,
		noopTracer,
	)
	require.NoError(bs.T(), err)
	bs.build.cfg.expiry = 11

	// Create two valid receipts for block B which build on different receipts
	// for the parent block (A); recB1 builds on top of RecA1, whilst recB2
	// builds on top of RecA2.
	resB1 := unittest.ExecutionResultFixture(unittest.WithBlock(&B), unittest.WithPreviousResult(recA1.ExecutionResult))
	recB1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resB1))
	resB2 := unittest.ExecutionResultFixture(unittest.WithBlock(&B), unittest.WithPreviousResult(recA2.ExecutionResult))
	recB2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resB2))
	resC := unittest.ExecutionResultFixture(unittest.WithBlock(&C), unittest.WithPreviousResult(recB1.ExecutionResult))
	recC := unittest.ExecutionReceiptFixture(unittest.WithResult(resC))

	// Add recB1 and recB2 to the mempool for inclusion in the next candidate
	_, _ = bs.build.recPool.AddReceipt(recB1, B.Header)
	_, _ = bs.build.recPool.AddReceipt(recB2, B.Header)
	_, _ = bs.build.recPool.AddReceipt(recC, C.Header)

	_, err = bs.build.BuildOn(C.ID(), bs.setter)
	bs.Require().NoError(err)
	expectedReceipts := flow.ExecutionReceiptMetaList{recB1.Meta(), recB2.Meta(), recC.Meta()}
	expectedResults := flow.ExecutionResultList{&recB1.ExecutionResult, &recB2.ExecutionResult, &recC.ExecutionResult}
	bs.Assert().ElementsMatch(expectedReceipts, bs.assembled.Receipts, "payload should contain receipts from valid execution forks")
	bs.Assert().ElementsMatch(expectedResults, bs.assembled.Results, "payload should contain results from valid execution forks")
}
