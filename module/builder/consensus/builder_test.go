package consensus

import (
	"fmt"
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

	// test parameters
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
	payloads   map[flow.Identifier]*flow.Payload

	// real dependencies
	dir      string
	db       *badger.DB
	sentinel uint64
	setter   func(*flow.Header) error

	// mocked dependencies
	headerDB  *storage.Headers
	sealDB    *storage.Seals
	payloadDB *storage.Payloads
	blockDB   *storage.Blocks
	guarPool  *mempool.Guarantees
	sealPool  *mempool.Seals

	// tracking behaviour
	assembled  *flow.Payload     // built payload
	remCollIDs []flow.Identifier // guarantees removed from mempool
	remSealIDs []flow.Identifier // seals removed from mempool

	// component under test
	build *Builder
}

func (bs *BuilderSuite) SetupTest() {

	// set up no-op dependencies
	noop := metrics.NewNoopCollector()

	// set up test parameters
	numFinalized := 4
	numPending := 4

	// initialize the storage
	bs.pendingIDs = nil
	bs.finalizedIDs = nil
	bs.chain = nil
	bs.guarantees = nil
	bs.seals = nil
	bs.headers = make(map[flow.Identifier]*flow.Header)
	bs.payloads = make(map[flow.Identifier]*flow.Payload)

	// initialize behaviour tracking
	bs.assembled = nil
	bs.remCollIDs = nil
	bs.remSealIDs = nil

	// insert the first block in our range
	first := unittest.BlockHeaderFixture()
	bs.firstID = first.ID()
	bs.headers[first.ID()] = &first
	bs.payloads[first.ID()] = &flow.Payload{}
	bs.last = &flow.Seal{
		BlockID:      first.ID(),
		ResultID:     flow.ZeroID, // we don't care
		InitialState: unittest.StateCommitmentFixture(),
		FinalState:   unittest.StateCommitmentFixture(),
	}

	// insert the finalized blocks between first and final
	previous := &first
	last := bs.last
	for n := 0; n < numFinalized; n++ {
		finalized := unittest.BlockHeaderWithParentFixture(previous)
		bs.finalizedIDs = append(bs.finalizedIDs, finalized.ID())
		bs.headers[finalized.ID()] = &finalized
		bs.payloads[finalized.ID()] = &flow.Payload{}
		previous = &finalized
		seal := &flow.Seal{
			BlockID:      finalized.ID(),
			ResultID:     flow.ZeroID, // we don't care
			InitialState: last.FinalState,
			FinalState:   unittest.StateCommitmentFixture(),
		}
		fmt.Printf("%x -> %x\n", seal.InitialState, seal.FinalState)
		bs.chain = append(bs.chain, seal)
		last = seal
	}

	// insert the finalized block with an empty payload
	final := unittest.BlockHeaderWithParentFixture(previous)
	bs.finalID = final.ID()
	bs.headers[final.ID()] = &final
	bs.payloads[final.ID()] = &flow.Payload{}
	seal := &flow.Seal{
		BlockID:      final.ID(),
		ResultID:     flow.ZeroID, // we don't care
		InitialState: last.FinalState,
		FinalState:   unittest.StateCommitmentFixture(),
	}
	fmt.Printf("%x -> %x\n", seal.InitialState, seal.FinalState)
	bs.chain = append(bs.chain, seal)
	last = seal

	// insert the finalized ancestors with empty payload
	previous = &final
	for n := 0; n < numPending; n++ {
		pending := unittest.BlockHeaderWithParentFixture(previous)
		bs.pendingIDs = append(bs.pendingIDs, pending.ID())
		bs.headers[pending.ID()] = &pending
		bs.payloads[pending.ID()] = &flow.Payload{}
		previous = &pending
		seal := &flow.Seal{
			BlockID:      final.ID(),
			ResultID:     flow.ZeroID, // we don't care
			InitialState: last.FinalState,
			FinalState:   unittest.StateCommitmentFixture(),
		}
		fmt.Printf("%x -> %x\n", seal.InitialState, seal.FinalState)
		bs.chain = append(bs.chain, seal)
		last = seal
	}

	// insert the parent block with an empty payload
	parent := unittest.BlockHeaderWithParentFixture(previous)
	bs.parentID = parent.ID()
	bs.headers[parent.ID()] = &parent
	bs.payloads[parent.ID()] = &flow.Payload{}
	seal = &flow.Seal{
		BlockID:      final.ID(),
		ResultID:     flow.ZeroID, // we don't care
		InitialState: last.FinalState,
		FinalState:   unittest.StateCommitmentFixture(),
	}
	fmt.Printf("%x -> %x\n", seal.InitialState, seal.FinalState)
	bs.chain = append(bs.chain, seal)

	// set up temporary database for tests
	bs.db, bs.dir = unittest.TempBadgerDB(bs.T())
	err := bs.db.Update(operation.InsertFinalizedHeight(final.Height))
	bs.Require().NoError(err)
	err = bs.db.Update(operation.IndexBlockHeight(final.Height, bs.finalID))
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
	bs.payloadDB = &storage.Payloads{}
	bs.payloadDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Payload {
			return bs.payloads[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := bs.headers[blockID]
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
		bs.payloadDB,
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
		bs.payloads[finalizedID].Guarantees = append(bs.payloads[finalizedID].Guarantees, guarantee)
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
		bs.payloads[pendingID].Guarantees = append(bs.payloads[pendingID].Guarantees, guarantee)
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

func (bs *BuilderSuite) TestPayloadSealValid() {

	// TODO: enable with new sealing logic
	bs.T().Skip()

	// use valid chain of seals in mempool
	bs.seals = bs.chain
	_, err := bs.build.BuildOn(bs.parentID, bs.setter)
	bs.Require().NoError(err)
	bs.Assert().Empty(bs.assembled.Guarantees, "should have no guarantees in payload with empty mempool")
	bs.Assert().ElementsMatch(bs.seals, bs.assembled.Seals, "should have included valid chain of seals")
	bs.Assert().Empty(bs.remSealIDs, "should not have removed empty seals")
}
