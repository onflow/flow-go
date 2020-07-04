package consensus

import (
	"math/rand"
	"os"
	"testing"

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
	firstID      flow.Identifier   // first block in the range we look at
	finalID      flow.Identifier   // last finalized block
	parentID     flow.Identifier   // parent block we build on
	finalizedIDs []flow.Identifier // blocks between first and final
	pendingIDs   []flow.Identifier // blocks between final and parent
	chain        []*flow.Seal      // chain of seals starting first

	// stored data
	last       *flow.Seal
	guarantees []*flow.CollectionGuarantee
	seals      []*flow.Seal
	headers    map[flow.Identifier]*flow.Header
	heights    map[uint64]*flow.Header
	index      map[flow.Identifier]*flow.Index

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
	sealPool *mempool.Seals

	// tracking behaviour
	assembled  *flow.Payload     // built payload
	remCollIDs []flow.Identifier // guarantees removed from mempool
	remSealIDs []flow.Identifier // seals removed from mempool

	// component under test
	build *Builder
}

func (bs *BuilderSuite) chainSeal(blockID flow.Identifier) {
	initial := bs.last.FinalState
	final := unittest.StateCommitmentFixture()
	if len(bs.chain) > 0 {
		initial = bs.chain[len(bs.chain)-1].FinalState
	}
	seal := &flow.Seal{
		BlockID:      blockID,
		ResultID:     flow.ZeroID, // we don't care
		InitialState: initial,
		FinalState:   final,
	}
	bs.chain = append(bs.chain, seal)
}

func (bs *BuilderSuite) SetupTest() {

	// set up no-op dependencies
	noop := metrics.NewNoopCollector()

	// set up test parameters
	numFinalized := 4
	numPending := 4

	// reset test helpers
	bs.pendingIDs = nil
	bs.finalizedIDs = nil
	bs.chain = nil

	// initialize the storage
	bs.last = nil
	bs.guarantees = nil
	bs.seals = nil
	bs.headers = make(map[flow.Identifier]*flow.Header)
	bs.heights = make(map[uint64]*flow.Header)
	bs.index = make(map[flow.Identifier]*flow.Index)

	// initialize behaviour tracking
	bs.assembled = nil
	bs.remCollIDs = nil
	bs.remSealIDs = nil

	// insert the first block in our range
	first := unittest.BlockHeaderFixture()
	bs.firstID = first.ID()
	bs.headers[first.ID()] = &first
	bs.heights[first.Height] = &first
	bs.index[first.ID()] = &flow.Index{}
	bs.last = &flow.Seal{
		BlockID:      first.ID(),
		ResultID:     flow.ZeroID,
		InitialState: unittest.StateCommitmentFixture(),
		FinalState:   unittest.StateCommitmentFixture(),
	}

	// insert the finalized blocks between first and final
	previous := &first
	for n := 0; n < numFinalized; n++ {
		finalized := unittest.BlockHeaderWithParentFixture(previous)
		bs.finalizedIDs = append(bs.finalizedIDs, finalized.ID())
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

	// insert the finalized ancestors with empty payload
	previous = &final
	for n := 0; n < numPending; n++ {
		pending := unittest.BlockHeaderWithParentFixture(previous)
		bs.pendingIDs = append(bs.pendingIDs, pending.ID())
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
	bs.sentinel = 1337
	bs.setter = func(header *flow.Header) error {
		header.View = 1337
		return nil
	}

	// set up storage mocks for tests
	bs.sealDB = &storage.Seals{}
	bs.sealDB.On("ByBlockID", bs.parentID).Return(bs.last, nil)
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
			return bs.guarantees
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
			return bs.seals
		},
	)
	bs.sealPool.On("Rem", mock.Anything).Run(func(args mock.Arguments) {
		sealID := args.Get(0).(flow.Identifier)
		bs.remSealIDs = append(bs.remSealIDs, sealID)
	}).Return(true)

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
	bs.guarantees = unittest.CollectionGuaranteesFixture(16, unittest.WithCollRef(bs.finalID))
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(bs.guarantees, bs.assembled.Guarantees, "should have guarantees from mempool in payload")
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().Empty(bs.remCollIDs, "should not remove any valid guarantees")
}

func (bs *BuilderSuite) TestPayloadGuaranteeDuplicateFinalized() {

	// create some valid guarantees
	valid := unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(bs.finalID))

	// create some duplicate guarantees and add to random finalized block
	duplicated := unittest.CollectionGuaranteesFixture(12, unittest.WithCollRef(bs.finalID))
	for _, guarantee := range duplicated {
		finalizedID := bs.finalizedIDs[rand.Intn(len(bs.finalizedIDs))]
		index := bs.index[finalizedID]
		index.CollectionIDs = append(index.CollectionIDs, guarantee.ID())
		bs.index[finalizedID] = index
	}

	// add sixteen guarantees to the pool
	bs.guarantees = append(valid, duplicated...)
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
		pendingID := bs.pendingIDs[rand.Intn(len(bs.pendingIDs))]
		index := bs.index[pendingID]
		index.CollectionIDs = append(index.CollectionIDs, guarantee.ID())
		bs.index[pendingID] = index
	}

	// add sixteen guarantees to the pool
	bs.guarantees = append(valid, duplicated...)
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
	bs.guarantees = append(valid, unknown...)
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
	bs.guarantees = append(valid, expired...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().ElementsMatch(valid, bs.assembled.Guarantees, "should have valid from mempool in payload")
	bs.Assert().Empty(bs.assembled.Seals, "should have no seals in payload with empty mempool")
	bs.Assert().ElementsMatch(flow.GetIDs(expired), bs.remCollIDs, "should remove guarantees with expired reference from mempool")
}

func (bs *BuilderSuite) TestPayloadSealAllValid() {

	// use valid chain of seals in mempool
	bs.seals = bs.chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.seals, bs.assembled.Seals, "should have included valid chain of seals")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}

func (bs *BuilderSuite) TestPayloadSealSomeValid() {

	// generate invalid seals
	invalid := unittest.BlockSealsFixture(8)

	// use both valid and non-valid seals for chain
	bs.seals = append(bs.chain, invalid...)
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain, bs.assembled.Seals, "should have included only valid chain of seals")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}

func (bs *BuilderSuite) TestPayloadSealCutoffChain() {

	// remove the seal at the start
	bs.seals = bs.chain[1:]

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().Empty(bs.assembled.Seals, "should have not included any chains from cutoff chain")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}

func (bs *BuilderSuite) TestPayloadSealBrokenChain() {

	// remove the seal at the start
	bs.seals = bs.chain[:3]
	bs.seals = append(bs.seals, bs.chain[4:]...)

	// use both valid and non-valid seals for chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.chain[:3], bs.assembled.Seals, "should have included only beginning of broken chain")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}
