package badger

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/suite"

	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type MutatorSuite struct {
	suite.Suite
	db    *badger.DB
	dbdir string

	genesis *model.Block
	chainID string

	state   cluster.State
	mutator cluster.Mutator
}

// runs before each test runs
func (suite *MutatorSuite) SetupTest() {
	var err error

	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	suite.genesis = model.Genesis()
	suite.chainID = suite.genesis.Header.ChainID

	suite.dbdir = unittest.TempDir(suite.T())
	suite.db = unittest.BadgerDB(suite.T(), suite.dbdir)

	suite.state, err = NewState(suite.db, suite.chainID)
	suite.Assert().Nil(err)
	suite.mutator = suite.state.Mutate()
}

// runs after each test finishes
func (suite *MutatorSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().Nil(err)
	err = os.RemoveAll(suite.dbdir)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) Bootstrap() {
	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) InsertBlock(block model.Block) {
	err := suite.db.Update(procedure.InsertClusterBlock(&block))
	suite.Assert().Nil(err)
}

func TestMutator(t *testing.T) {
	suite.Run(t, new(MutatorSuite))
}

func (suite *MutatorSuite) TestBootstrap_InvalidChainID() {
	suite.genesis.Header.ChainID = fmt.Sprintf("%s-invalid", suite.genesis.Header.ChainID)

	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidNumber() {
	suite.genesis.Header.Height = 1

	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidParentHash() {
	suite.genesis.Header.ParentID = unittest.IdentifierFixture()

	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidPayloadHash() {
	suite.genesis.Header.PayloadHash = unittest.IdentifierFixture()

	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidPayload() {
	// this is invalid because genesis collection should be empty
	suite.genesis.Payload = unittest.ClusterPayloadFixture(2)

	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_Successful() {

	// bootstrap
	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Nil(err)

	err = suite.db.View(func(tx *badger.Txn) error {

		// should insert collection
		var collection flow.LightCollection
		err = operation.RetrieveCollection(suite.genesis.Payload.Collection.ID(), &collection)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Payload.Collection.Light(), collection)

		// should index collection
		collection = flow.LightCollection{} // reset the collection
		err = operation.LookupCollectionPayload(suite.genesis.Header.Height, suite.genesis.ID(), suite.genesis.Header.ParentID, &collection.Transactions)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Payload.Collection.Light(), collection)

		// should insert header
		var header flow.Header
		err = operation.RetrieveHeader(suite.genesis.ID(), &header)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Header.ID(), header.ID())

		// should insert block number -> ID lookup
		var blockID flow.Identifier
		err = operation.RetrieveNumberForCluster(suite.genesis.Header.ChainID, suite.genesis.Header.Height, &blockID)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.ID(), blockID)

		// should insert boundary
		var boundary uint64
		err = operation.RetrieveBoundaryForCluster(suite.genesis.Header.ChainID, &boundary)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Header.Height, boundary)

		return nil
	})
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_WithoutBootstrap() {
	block := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(block)

	err := suite.mutator.Extend(block.ID())
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_NonexistentBlock() {
	suite.Bootstrap()

	// ID of a non-existent block
	blockID := unittest.IdentifierFixture()

	err := suite.mutator.Extend(blockID)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_InvalidChainID() {
	suite.Bootstrap()

	block := unittest.ClusterBlockWithParent(suite.genesis)
	// change the chain ID
	block.Header.ChainID = fmt.Sprintf("%s-invalid", block.Header.ChainID)
	suite.InsertBlock(block)

	err := suite.mutator.Extend(block.ID())
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_InvalidBlockNumber() {
	suite.Bootstrap()

	block := unittest.ClusterBlockWithParent(suite.genesis)
	// change the block number
	block.Header.Height = block.Header.Height - 1
	suite.InsertBlock(block)

	err := suite.mutator.Extend(block.ID())
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_OnParentOfFinalized() {
	suite.Bootstrap()

	// build one block on top of genesis
	block1 := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(block1)
	err := suite.mutator.Extend(block1.ID())
	suite.Assert().Nil(err)

	// finalize the block
	err = suite.db.Update(procedure.FinalizeClusterBlock(block1.ID()))
	suite.Assert().Nil(err)

	// insert another block on top of genesis
	// since we have already finalized block 1, this is invalid
	block2 := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(block2)

	// try to extend with the invalid block
	err = suite.mutator.Extend(block2.ID())
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_Success() {
	suite.Bootstrap()

	block := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(block)

	err := suite.mutator.Extend(block.ID())
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_WithEmptyCollection() {
	suite.Bootstrap()

	block := unittest.ClusterBlockWithParent(suite.genesis)
	// set an empty collection as the payload
	block.SetPayload(model.EmptyPayload())
	suite.InsertBlock(block)

	err := suite.mutator.Extend(block.ID())
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_UnfinalizedBlockWithDupeTx() {
	suite.Bootstrap()

	tx1 := unittest.TransactionBodyFixture()

	// create a block extending genesis containing tx1
	block1 := unittest.ClusterBlockWithParent(suite.genesis)
	payload1 := model.PayloadFromTransactions(&tx1)
	block1.SetPayload(payload1)
	suite.InsertBlock(block1)

	// should be able to extend block 1
	err := suite.mutator.Extend(block1.ID())
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := unittest.ClusterBlockWithParent(&block1)
	payload2 := model.PayloadFromTransactions(&tx1)
	block2.SetPayload(payload2)
	suite.InsertBlock(block2)

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.mutator.Extend(block2.ID())
	suite.T().Log(err)
	suite.Assert().True(errors.Is(err, storage.ErrAlreadyIndexed))
}

func (suite *MutatorSuite) TestExtend_FinalizedBlockWithDupeTx() {
	suite.Bootstrap()

	tx1 := unittest.TransactionBodyFixture()

	// create a block extending genesis containing tx1
	block1 := unittest.ClusterBlockWithParent(suite.genesis)
	payload1 := model.PayloadFromTransactions(&tx1)
	block1.SetPayload(payload1)
	suite.InsertBlock(block1)

	// should be able to extend block 1
	err := suite.mutator.Extend(block1.ID())
	suite.Assert().Nil(err)

	// should be able to finalize block 1
	err = suite.db.Update(procedure.FinalizeClusterBlock(block1.ID()))
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := unittest.ClusterBlockWithParent(&block1)
	payload2 := model.PayloadFromTransactions(&tx1)
	block2.SetPayload(payload2)
	suite.InsertBlock(block2)

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.mutator.Extend(block2.ID())
	suite.T().Log(err)
	suite.Assert().True(errors.Is(err, storage.ErrAlreadyIndexed))
}

func (suite *MutatorSuite) TestExtend_ConflictingForkWithDupeTx() {
	suite.Bootstrap()

	tx1 := unittest.TransactionBodyFixture()

	// create a block extending genesis containing tx1
	block1 := unittest.ClusterBlockWithParent(suite.genesis)
	payload1 := model.PayloadFromTransactions(&tx1)
	block1.SetPayload(payload1)
	suite.InsertBlock(block1)

	// should be able to extend block 1
	err := suite.mutator.Extend(block1.ID())
	suite.Assert().Nil(err)

	// create a block ALSO extending genesis ALSO containing tx1
	block2 := unittest.ClusterBlockWithParent(suite.genesis)
	payload2 := model.PayloadFromTransactions(&tx1)
	block2.SetPayload(payload2)
	suite.InsertBlock(block2)

	// should be able to extend block2, although it conflicts with block1,
	// it is on a different fork
	err = suite.mutator.Extend(block2.ID())
	suite.Assert().Nil(err)
}
