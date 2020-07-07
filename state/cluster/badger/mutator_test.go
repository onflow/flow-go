package badger

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/bootstrap"
	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/state/cluster"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type MutatorSuite struct {
	suite.Suite
	db    *badger.DB
	dbdir string

	genesis *model.Block
	chainID flow.ChainID

	// protocol state for reference blocks for transactions
	protoState *protocol.State

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

	metrics := metrics.NewNoopCollector()
	headers, identities, _, seals, index, conPayloads, blocks := util.StorageLayer(suite.T(), suite.db)
	colPayloads := storage.NewClusterPayloads(metrics, suite.db)

	suite.state, err = NewState(suite.db, suite.chainID, headers, colPayloads)
	suite.Assert().Nil(err)
	suite.mutator = suite.state.Mutate()

	suite.protoState, err = protocol.NewState(metrics, suite.db, headers, identities, seals, index, conPayloads, blocks)
	require.NoError(suite.T(), err)
}

// runs after each test finishes
func (suite *MutatorSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().Nil(err)
	err = os.RemoveAll(suite.dbdir)
	suite.Assert().Nil(err)
}

// Bootstrap bootstraps the cluster chain. Useful for conciseness in test cases
// working an already bootstrapped state.
func (suite *MutatorSuite) Bootstrap() {

	// just bootstrap with a genesis block, we'll use this as reference
	genesis := unittest.GenesisFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))
	result := bootstrap.Result(genesis, unittest.GenesisStateCommitment)
	seal := bootstrap.Seal(result)
	err := suite.protoState.Mutate().Bootstrap(genesis, result, seal)
	suite.Require().Nil(err)

	// bootstrap cluster chain
	err = suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Nil(err)
}

// Payload returns a valid cluster block payload containing the given transactions.
func (suite *MutatorSuite) Payload(transactions ...*flow.TransactionBody) model.Payload {
	final, err := suite.protoState.Final().Head()
	suite.Require().Nil(err)

	// find the oldest reference block among the transactions
	minRefID := final.ID() // use final by default
	minRefHeight := uint64(math.MaxUint64)
	for _, tx := range transactions {
		refBlock, err := suite.protoState.AtBlockID(tx.ReferenceBlockID).Head()
		if err != nil {
			continue
		}
		if refBlock.Height < minRefHeight {
			minRefHeight = refBlock.Height
			minRefID = refBlock.ID()
		}
	}
	return model.PayloadFromTransactions(minRefID, transactions...)
}

// BlockWithParent returns a valid block with the given parent.
func (suite *MutatorSuite) BlockWithParent(parent *model.Block) model.Block {
	block := unittest.ClusterBlockWithParent(parent)
	payload := suite.Payload()
	block.SetPayload(payload)
	return block
}

// Block returns a valid cluster block with genesis as parent.
func (suite *MutatorSuite) Block() model.Block {
	return suite.BlockWithParent(suite.genesis)
}

func (suite *MutatorSuite) Tx(opts ...func(*flow.TransactionBody)) flow.TransactionBody {
	tx := unittest.TransactionBodyFixture(opts...)
	tx.ReferenceBlockID = suite.genesis.ID()
	return tx
}

func TestMutator(t *testing.T) {
	suite.Run(t, new(MutatorSuite))
}

func (suite *MutatorSuite) TestBootstrap_InvalidChainID() {
	suite.genesis.Header.ChainID = flow.ChainID(fmt.Sprintf("%s-invalid", suite.genesis.Header.ChainID))

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
		err = operation.LookupCollectionPayload(suite.genesis.ID(), &collection.Transactions)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Payload.Collection.Light(), collection)

		// should insert header
		var header flow.Header
		err = operation.RetrieveHeader(suite.genesis.ID(), &header)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Header.ID(), header.ID())

		// should insert block number -> ID lookup
		var blockID flow.Identifier
		err = operation.LookupClusterBlockHeight(suite.genesis.Header.ChainID, suite.genesis.Header.Height, &blockID)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.ID(), blockID)

		// should insert boundary
		var boundary uint64
		err = operation.RetrieveClusterFinalizedHeight(suite.genesis.Header.ChainID, &boundary)(tx)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Header.Height, boundary)

		return nil
	})
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_WithoutBootstrap() {
	block := unittest.ClusterBlockWithParent(suite.genesis)
	err := suite.mutator.Extend(&block)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_InvalidChainID() {
	suite.Bootstrap()

	block := suite.Block()
	// change the chain ID
	block.Header.ChainID = flow.ChainID(fmt.Sprintf("%s-invalid", block.Header.ChainID))

	err := suite.mutator.Extend(&block)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_InvalidBlockNumber() {
	suite.Bootstrap()

	block := suite.Block()
	// change the block number
	block.Header.Height = block.Header.Height - 1

	err := suite.mutator.Extend(&block)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_OnParentOfFinalized() {
	suite.Bootstrap()

	// build one block on top of genesis
	block1 := suite.Block()
	err := suite.mutator.Extend(&block1)
	suite.Assert().Nil(err)

	// finalize the block
	err = suite.db.Update(procedure.FinalizeClusterBlock(block1.ID()))
	suite.Assert().Nil(err)

	// insert another block on top of genesis
	// since we have already finalized block 1, this is invalid
	block2 := suite.Block()

	// try to extend with the invalid block
	err = suite.mutator.Extend(&block2)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_Success() {
	suite.Bootstrap()

	block := suite.Block()
	err := suite.mutator.Extend(&block)
	suite.Assert().Nil(err)

	// should be able to retrieve the block
	var extended model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(block.ID(), &extended))
	suite.Assert().Nil(err)
	suite.Assert().Equal(*block.Payload, *extended.Payload)

	// the block should be indexed by its parent
	var childIDs []flow.Identifier
	err = suite.db.View(procedure.LookupBlockChildren(suite.genesis.ID(), &childIDs))
	suite.Assert().Nil(err)
	suite.Require().Len(childIDs, 1)
	suite.Assert().Equal(block.ID(), childIDs[0])
}

func (suite *MutatorSuite) TestExtend_WithEmptyCollection() {
	suite.Bootstrap()

	block := suite.Block()
	// set an empty collection as the payload
	block.SetPayload(model.EmptyPayload(flow.ZeroID))
	err := suite.mutator.Extend(&block)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_UnfinalizedBlockWithDupeTx() {
	suite.Bootstrap()

	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	block1 := suite.Block()
	payload1 := suite.Payload(&tx1)
	block1.SetPayload(payload1)

	// should be able to extend block 1
	err := suite.mutator.Extend(&block1)
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := suite.BlockWithParent(&block1)
	payload2 := suite.Payload(&tx1)
	block2.SetPayload(payload2)

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.mutator.Extend(&block2)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_FinalizedBlockWithDupeTx() {
	suite.Bootstrap()

	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	block1 := suite.Block()
	payload1 := suite.Payload(&tx1)
	block1.SetPayload(payload1)

	// should be able to extend block 1
	err := suite.mutator.Extend(&block1)
	suite.Assert().Nil(err)

	// should be able to finalize block 1
	err = suite.db.Update(procedure.FinalizeClusterBlock(block1.ID()))
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := suite.BlockWithParent(&block1)
	payload2 := suite.Payload(&tx1)
	block2.SetPayload(payload2)

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.mutator.Extend(&block2)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_ConflictingForkWithDupeTx() {
	suite.Bootstrap()

	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	block1 := suite.Block()
	payload1 := suite.Payload(&tx1)
	block1.SetPayload(payload1)

	// should be able to extend block 1
	err := suite.mutator.Extend(&block1)
	suite.Assert().Nil(err)

	// create a block ALSO extending genesis ALSO containing tx1
	block2 := suite.Block()
	payload2 := suite.Payload(&tx1)
	block2.SetPayload(payload2)

	// should be able to extend block2
	// although it conflicts with block1, it is on a different fork
	err = suite.mutator.Extend(&block2)
	suite.Assert().Nil(err)
}
