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
	firstID           flow.Identifier   // first block in the range we look at
	finalID           flow.Identifier   // last finalized block
	parentID          flow.Identifier   // parent block we build on
	finalizedBlockIDs []flow.Identifier // blocks between first and final
	pendingBlockIDs   []flow.Identifier // blocks between final and parent
	chain             []*flow.Seal      // chain of seals starting first

	// stored data
	pendingGuarantees []*flow.CollectionGuarantee
	pendingSeals      []*flow.Seal
	pendingReceipts   []*flow.ExecutionReceipt

	seals   map[flow.Identifier]*flow.Seal
	headers map[flow.Identifier]*flow.Header
	heights map[uint64]*flow.Header
	index   map[flow.Identifier]*flow.Index

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
	guarPool *mempool.Guarantees
	sealPool *mempool.Seals
	recPool  *mempool.Receipts

	// tracking behaviour
	assembled  *flow.Payload     // built payload
	remCollIDs []flow.Identifier // guarantees removed from mempool
	remSealIDs []flow.Identifier // seals removed from mempool
	remRecIDs  []flow.Identifier // receipts removed from mempool

	// component under test
	build *Builder
}

func (bs *BuilderSuite) chainSeal(blockID flow.Identifier) {
	final := unittest.StateCommitmentFixture()

	seal := &flow.Seal{
		BlockID:    blockID,
		ResultID:   flow.ZeroID, // we don't care
		FinalState: final,
	}
	bs.chain = append(bs.chain, seal)
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

	// initialize the pools
	bs.pendingGuarantees = nil
	bs.pendingSeals = nil
	bs.pendingReceipts = nil

	// initialise the dbs
	bs.lastSeal = nil
	bs.seals = make(map[flow.Identifier]*flow.Seal)
	bs.headers = make(map[flow.Identifier]*flow.Header)
	bs.heights = make(map[uint64]*flow.Header)
	bs.index = make(map[flow.Identifier]*flow.Index)

	// initialize behaviour tracking
	bs.assembled = nil
	bs.remCollIDs = nil
	bs.remSealIDs = nil
	bs.remRecIDs = nil

	// insert the first block in our range
	first := unittest.BlockHeaderFixture()
	bs.firstID = first.ID()
	bs.headers[first.ID()] = &first
	bs.heights[first.Height] = &first
	bs.index[first.ID()] = &flow.Index{}
	bs.lastSeal = &flow.Seal{
		BlockID:    first.ID(),
		ResultID:   flow.ZeroID,
		FinalState: unittest.StateCommitmentFixture(),
	}
	bs.seals[first.ID()] = bs.lastSeal

	// insert the finalized blocks between first and final
	previous := &first
	for n := 0; n < numFinalizedBlocks; n++ {
		finalized := unittest.BlockHeaderWithParentFixture(previous)
		bs.finalizedBlockIDs = append(bs.finalizedBlockIDs, finalized.ID())
		bs.headers[finalized.ID()] = &finalized
		bs.heights[finalized.Height] = &finalized
		bs.index[finalized.ID()] = &flow.Index{}
		bs.chainSeal(finalized.ID())
		previous = &finalized
	}

	// insert the finalized block with an empty payload
	final := unittest.BlockHeaderWithParentFixture(previous)
	bs.finalID = final.ID()
	bs.headers[final.ID()] = &final
	bs.heights[final.Height] = &final
	bs.index[final.ID()] = &flow.Index{}
	bs.chainSeal(final.ID())

	// insert the pending ancestors with empty payload
	previous = &final
	for n := 0; n < numPendingBlocks; n++ {
		pending := unittest.BlockHeaderWithParentFixture(previous)
		bs.pendingBlockIDs = append(bs.pendingBlockIDs, pending.ID())
		bs.headers[pending.ID()] = &pending
		bs.index[pending.ID()] = &flow.Index{}
		bs.chainSeal(pending.ID())
		previous = &pending
	}

	// insert the parent block with an empty payload
	parent := unittest.BlockHeaderWithParentFixture(previous)
	bs.parentID = parent.ID()
	bs.headers[parent.ID()] = &parent
	bs.index[parent.ID()] = &flow.Index{}
	bs.chainSeal(parent.ID())

	// set up temporary database for tests
	bs.db, bs.dir = unittest.TempBadgerDB(bs.T())

	err := bs.db.Update(operation.InsertFinalizedHeight(final.Height))
	bs.Require().NoError(err)
	err = bs.db.Update(operation.IndexBlockHeight(final.Height, bs.finalID))
	bs.Require().NoError(err)

	err = bs.db.Update(operation.InsertRootHeight(13))
	bs.Require().NoError(err)

	err = bs.db.Update(operation.InsertSealedHeight(first.Height))
	bs.Require().NoError(err)
	err = bs.db.Update(operation.IndexBlockHeight(first.Height, first.ID()))
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

	bs.sealPool = &mempool.Seals{}
	bs.sealPool.On("Size").Return(uint(0)) // only used by metrics
	bs.sealPool.On("All").Return(
		func() []*flow.Seal {
			return bs.pendingSeals
		},
	)
	bs.sealPool.On("Rem", mock.Anything).Run(func(args mock.Arguments) {
		sealID := args.Get(0).(flow.Identifier)
		bs.remSealIDs = append(bs.remSealIDs, sealID)
	}).Return(true)

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

	// initialize the builder
	bs.build = NewBuilder(
		noop,
		bs.db,
		bs.state,
		bs.headerDB,
		bs.sealDB,
		bs.indexDB,
		bs.guarPool,
		bs.sealPool,
		bs.recPool,
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
	bs.pendingSeals = bs.chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.pendingSeals, bs.assembled.Seals, "should have included valid chain of seals")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}

func (bs *BuilderSuite) TestPayloadSealSomeValid() {

	// generate invalid seals
	invalid := unittest.BlockSealsFixture(8)

	// use both valid and non-valid seals for chain
	bs.pendingSeals = append(bs.chain, invalid...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain, bs.assembled.Seals, "should have included only valid chain of seals")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}

func (bs *BuilderSuite) TestPayloadSealCutoffChain() {

	// remove the seal at the start
	bs.pendingSeals = bs.chain[1:]

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should have not included any chains from cutoff chain")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}

func (bs *BuilderSuite) TestPayloadSealBrokenChain() {

	// remove the seal at the start
	bs.pendingSeals = bs.chain[:3]
	bs.pendingSeals = append(bs.pendingSeals, bs.chain[4:]...)

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain[:3], bs.assembled.Seals, "should have included only beginning of broken chain")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
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

// Valid receipts should be inserted in the payload, sorted by block height.
func (bs *BuilderSuite) TestPayloadReceiptSorted() {

	// create valid receipts for known, unsealed blocks
	receipts := []*flow.ExecutionReceipt{}
	var i uint64
	for i = 0; i < 5; i++ {
		pendingReceipt := unittest.ExecutionReceiptFixture()
		bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{
			Height: i,
		}
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
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts that have been inserted in payload")
}

// Payloads can contain multiple receipts for a given block.
func (bs *BuilderSuite) TestPayloadReceiptMultipleReceiptsWithDifferentResults() {

	// create MULTIPLE valid receipts for known, unsealed blocks
	receipts := []*flow.ExecutionReceipt{}

	var i uint64
	for i = 0; i < 5; i++ {
		// receipt template
		pendingReceipt := unittest.ExecutionReceiptFixture()
		receipts = append(receipts, pendingReceipt)

		// insert 3 receipts for the same block but different results
		for j := 0; j < 3; j++ {
			dupReceipt := unittest.ExecutionReceiptFixture()
			dupReceipt.ExecutionResult.BlockID = pendingReceipt.ExecutionResult.BlockID
			receipts = append(receipts, dupReceipt)
		}

		bs.headers[pendingReceipt.ExecutionResult.BlockID] = &flow.Header{
			Height: i,
		}
	}

	bs.pendingReceipts = receipts

	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Equal(bs.assembled.Receipts, receipts, "payload should contain all receipts for a given block")
	bs.Assert().ElementsMatch(flow.GetIDs(bs.pendingReceipts), bs.remRecIDs, "should remove receipts that have been inserted in payload")
}
