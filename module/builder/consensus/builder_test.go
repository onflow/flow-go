package consensus

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	storerr "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestConsensusBuilder(t *testing.T) {
	suite.Run(t, new(BuilderSuite))
}

type BuilderSuite struct {
	suite.Suite

	// test helpers
	firstID           flow.Identifier   // first block in the range we look at
	finalID           flow.Identifier   // last finalized block
	parentID          flow.Identifier   // parent block we build on
	finalizedBlockIDs []flow.Identifier // blocks between first and final
	pendingBlockIDs   []flow.Identifier // blocks between final and parent

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
	seals   map[flow.Identifier]*flow.Seal
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
func (bs *BuilderSuite) createAndRecordBlock(previous *flow.Block) *flow.Block {
	var header flow.Header
	if previous == nil {
		header = unittest.BlockHeaderFixture()
	} else {
		header = unittest.BlockHeaderWithParentFixture(previous.Header)
	}

	block := &flow.Block{
		Header:  &header,
		Payload: unittest.PayloadFixture(),
	}

	// if previous is not nil, create a receipt for a result of the previous
	// block, and add it to the payload. The corresponding IncorporatedResult
	// will be use to seal the previous block, and to create an
	// IncorporatedResultSeal for the seal mempool.
	var incorporatedResult *flow.IncorporatedResult

	if previous != nil {
		previousResult := unittest.ResultForBlockFixture(previous)
		receipt := unittest.ExecutionReceiptFixture()
		receipt.ExecutionResult = *previousResult
		block.Payload.Receipts = append(block.Payload.Receipts, receipt)

		// include a result of the previous block
		incorporatedResult = &flow.IncorporatedResult{
			IncorporatedBlockID: block.ID(),
			Result:              previousResult,
		}
	}

	// record block in dbs
	bs.headers[block.ID()] = block.Header
	bs.heights[block.Header.Height] = block.Header
	bs.blocks[block.ID()] = block
	bs.index[block.ID()] = block.Payload.Index()

	// seal the previous block with the result included in this block. Do not
	// seal the first block because it is assumed that it is already sealed.
	if previous != nil && previous.ID() != bs.firstID {
		bs.chainSeal(incorporatedResult)
	}

	return block
}

// Create a seal for the result's block, and link it to the previous seal's
// final state, thereby creating a valid chain. The corresponding
// IncorporatedResultSeal, which ties the seal to the incorporated result it
// seals, is also recorded for future access.
func (bs *BuilderSuite) chainSeal(incorporatedResult *flow.IncorporatedResult) {
	initial := bs.lastSeal.FinalState
	final := unittest.StateCommitmentFixture()
	if len(bs.chain) > 0 {
		initial = bs.chain[len(bs.chain)-1].FinalState
	}
	seal := &flow.Seal{
		BlockID:      incorporatedResult.Result.BlockID,
		ResultID:     incorporatedResult.Result.ID(),
		InitialState: initial,
		FinalState:   final,
	}
	bs.chain = append(bs.chain, seal)

	incorporatedResultSeal := &flow.IncorporatedResultSeal{
		IncorporatedResult: incorporatedResult,
		Seal:               seal,
	}
	bs.irsMap[incorporatedResultSeal.ID()] = incorporatedResultSeal
	bs.irsList = append(bs.irsList, incorporatedResultSeal)
}

/*

first                   final                    parent
 |                        |                        |
[ ]--[ ]--[ ]--[ ]--[ ]--[ ]--[ ]--[ ]--[ ]--[ ]--[ ]--{ }
 |
sealed

*/
func (bs *BuilderSuite) SetupTest() {

	// set up no-op dependencies
	noop := metrics.NewNoopCollector()

	// set up test parameters
	numFinalizedBlocks := 4
	numPendingBlocks := 4

	// reset test helpers
	bs.pendingBlockIDs = nil
	bs.finalizedBlockIDs = nil

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
	bs.seals = make(map[flow.Identifier]*flow.Seal)
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
	bs.lastSeal = &flow.Seal{
		BlockID:      first.ID(),
		ResultID:     flow.ZeroID,
		InitialState: unittest.StateCommitmentFixture(),
		FinalState:   unittest.StateCommitmentFixture(),
	}
	bs.seals[first.ID()] = bs.lastSeal

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

	// set up storage mocks for tests
	bs.sealDB = &storage.Seals{}
	bs.sealDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Seal {
			return bs.seals[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := bs.seals[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

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
		noop,
		bs.db,
		bs.headerDB,
		bs.sealDB,
		bs.indexDB,
		bs.blockDB,
		bs.guarPool,
		bs.sealPool,
		bs.recPool,
		bs.resPool,
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

func (bs *BuilderSuite) TestPayloadSealSomeValid() {

	bs.pendingSeals = bs.irsMap

	// add some invalid seals to the mempool
	for i := 0; i < 8; i++ {
		invalid := &flow.IncorporatedResultSeal{
			IncorporatedResult: unittest.IncorporatedResultFixture(),
			Seal:               unittest.BlockSealFixture(),
		}
		bs.pendingSeals[invalid.ID()] = invalid
	}

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain, bs.assembled.Seals, "should have included only valid chain of seals")
}

func (bs *BuilderSuite) TestPayloadSealCutoffChain() {

	// remove the seal at the start
	cutoff := bs.irsMap
	first := bs.irsList[0]
	delete(cutoff, first.ID())

	bs.pendingSeals = cutoff

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should have not included any seals from cutoff chain")
}

func (bs *BuilderSuite) TestPayloadSealBrokenChain() {

	// remove the 4th seal
	brokenChain := bs.irsMap
	fourth := bs.irsList[3]
	delete(brokenChain, fourth.ID())

	bs.pendingSeals = brokenChain

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain[:3], bs.assembled.Seals, "should have included only beginning of broken chain")
}

// Receipts for unknown blocks should not be inserted, and should be removed
// from the mempool.
func (bs *BuilderSuite) TestPayloadReceiptUnknownBlock() {

	bs.pendingReceipts = []*flow.ExecutionReceipt{}

	// create a valid receipt for an unknown block
	pendingReceiptUnknownBlock := unittest.ExecutionReceiptFixture()
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceiptUnknownBlock)

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when pending receipts are for unknown blocks")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts with unknown blocks")
}

// Receipts for sealed blocks should not be reinserted, and should be removed
// from the mempool.
func (bs *BuilderSuite) TestPayloadReceiptSealedBlock() {

	bs.pendingReceipts = []*flow.ExecutionReceipt{}

	// create a valid receipt for a known but sealed block
	pendingReceiptSealedBlock := unittest.ExecutionReceiptFixture()
	bs.headers[pendingReceiptSealedBlock.ExecutionResult.BlockID] = &flow.Header{}
	bs.seals[pendingReceiptSealedBlock.ExecutionResult.BlockID] = &flow.Seal{}
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceiptSealedBlock)

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when pending receipts are for sealed blocks")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts with sealed blocks")
}

// Receipts that are already included in finalized blocks should not be
// reinserted and should be removed from the mempool.
func (bs *BuilderSuite) TestPayloadReceiptInFinalizedBlock() {

	bs.pendingReceipts = []*flow.ExecutionReceipt{}

	// create a valid receipt for a known, unsealed block
	pendingReceipt := unittest.ExecutionReceiptFixture()
	bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{}
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceipt)

	// put the receipt in a finalized block (chosen at random)
	blockID := bs.finalizedBlockIDs[rand.Intn(len(bs.finalizedBlockIDs))]
	index := bs.index[blockID]
	index.ReceiptIDs = append(index.ReceiptIDs, pendingReceipt.ID())
	bs.index[blockID] = index

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when pending receipts are already included in finalized blocks")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts that are already in finalized blocks")
}

// Receipts that are already included in pending blocks should not be reinserted
// but should stay in the mempool.
func (bs *BuilderSuite) TestPayloadReceiptInPendingBlock() {
	bs.pendingReceipts = []*flow.ExecutionReceipt{}

	// create a valid receipt for a known, unsealed block
	pendingReceipt := unittest.ExecutionReceiptFixture()
	bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{}
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceipt)

	// put the receipt in a pending block block (chosen at random)
	blockID := bs.pendingBlockIDs[rand.Intn(len(bs.pendingBlockIDs))]
	index := bs.index[blockID]
	index.ReceiptIDs = append(index.ReceiptIDs, pendingReceipt.ID())
	bs.index[blockID] = index

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when pending receipts are already included in pending blocks")
	bs.Assert().Empty(bs.remRecIDs, "should not remove receipts that are already in pending blocks")
}

// Receipts for blocks that are not on the fork should be skipped but not
// removed from the mempool
func (bs *BuilderSuite) TestPayloadReceiptForBlockNotInFork() {
	bs.pendingReceipts = []*flow.ExecutionReceipt{}

	// create a valid receipt for a known, unsealed block, not on the fork
	pendingReceipt := unittest.ExecutionReceiptFixture()
	bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{}
	bs.pendingReceipts = append(bs.pendingReceipts, pendingReceipt)

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Receipts, "should have no receipts in payload when receipts correspond to blocks not on the fork")
	bs.Assert().Empty(bs.remRecIDs, "should not remove receipts that are for blocks not on the fork")
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

	header, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.assembled.Receipts, receipts, "payload should contain receipts ordered by block height")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts that have been inserted in payload")

	expectedIncorporatedResults := []*flow.IncorporatedResult{}
	for i := 0; i < 5; i++ {
		expectedIncorporatedResults = append(expectedIncorporatedResults,
			&flow.IncorporatedResult{
				IncorporatedBlockID: header.ID(),
				Result:              &receipts[i].ExecutionResult,
			})
	}

	bs.Assert().ElementsMatch(bs.incorporatedResults, expectedIncorporatedResults, "should insert incorporated results in mempool")
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

		bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{
			Height: i,
		}
	}

	bs.pendingReceipts = receipts

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(receipts, bs.assembled.Receipts, "payload should contain all receipts for a given block")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts that have been inserted in payload")
}

// IncorporatedBlockID should correspond to the first block on the fork that
// contained the result.
func (bs *BuilderSuite) TestPayloadResultFirstAppearance() {
	// Insert a receipt for the same result as a receipt that is already on the
	// fork. The generated IncorporatedResult should reference the ID of the
	// first block that contained the result.

	// We create receipts for the second block because the first block is
	// already sealed and the incoming receipt would be skipped
	secondBlock := bs.blocks[bs.irsList[1].Seal.BlockID]
	secondBlockResult := secondBlock.Payload.Receipts[0].ExecutionResult

	// create another receipt for the same result in the third block
	thirdBlock := bs.blocks[bs.irsList[2].Seal.BlockID]
	duplicateReceipt := unittest.ReceiptForBlockFixture(secondBlock)
	duplicateReceipt.ExecutionResult = secondBlockResult
	thirdBlock.Payload.Receipts = append(thirdBlock.Payload.Receipts, duplicateReceipt)

	// add another receipt for the same result to the mempool
	receiptForExistingResult := unittest.ReceiptForBlockFixture(secondBlock)
	receiptForExistingResult.ExecutionResult = secondBlockResult
	bs.pendingReceipts = []*flow.ExecutionReceipt{receiptForExistingResult}

	// Now check that only one IncorporatedResult was added to the mempoool, and
	// that its IncorporatedBlockID corresponds to the first block

	expectedIncorporatedResults := []*flow.IncorporatedResult{
		{
			IncorporatedBlockID: secondBlock.ID(),
			Result:              &secondBlockResult,
		},
	}

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.assembled.Receipts, bs.pendingReceipts, "payload should contain receipts even if they contain an existing result")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts that have been inserted in payload")
	bs.Assert().ElementsMatch(bs.incorporatedResults, expectedIncorporatedResults, "should insert incorporated results in mempool")
}
