package badger

import (
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	model "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

type SnapshotSuite struct {
	suite.Suite
	db    *badger.DB
	dbdir string

	genesis *model.Block
	chainID flow.ChainID

	protoState protocol.State

	state cluster.MutableState
}

// runs before each test runs
func (suite *SnapshotSuite) SetupTest() {
	var err error

	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	suite.genesis = model.Genesis()
	suite.chainID = suite.genesis.Header.ChainID

	suite.dbdir = unittest.TempDir(suite.T())
	suite.db = unittest.BadgerDB(suite.T(), suite.dbdir)

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	headers, _, seals, _, _, blocks, setups, commits, statuses, results := util.StorageLayer(suite.T(), suite.db)
	colPayloads := storage.NewClusterPayloads(metrics, suite.db)

	clusterStateRoot, err := NewStateRoot(suite.genesis)
	suite.Assert().Nil(err)
	clusterState, err := Bootstrap(suite.db, clusterStateRoot)
	suite.Assert().Nil(err)
	suite.state, err = NewMutableState(clusterState, tracer, headers, colPayloads)
	suite.Assert().Nil(err)

	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	root := unittest.RootSnapshotFixture(participants)

	suite.protoState, err = pbadger.Bootstrap(metrics, suite.db, headers, seals, results, blocks, setups, commits, statuses, root)
	require.NoError(suite.T(), err)

	suite.Require().Nil(err)
}

// runs after each test finishes
func (suite *SnapshotSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().Nil(err)
	err = os.RemoveAll(suite.dbdir)
	suite.Assert().Nil(err)
}

// Payload returns a valid cluster block payload containing the given transactions.
func (suite *SnapshotSuite) Payload(transactions ...*flow.TransactionBody) model.Payload {
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
func (suite *SnapshotSuite) BlockWithParent(parent *model.Block) model.Block {
	block := unittest.ClusterBlockWithParent(parent)
	payload := suite.Payload()
	block.SetPayload(payload)
	return block
}

// Block returns a valid cluster block with genesis as parent.
func (suite *SnapshotSuite) Block() model.Block {
	return suite.BlockWithParent(suite.genesis)
}

func (suite *SnapshotSuite) InsertBlock(block model.Block) {
	err := suite.db.Update(procedure.InsertClusterBlock(&block))
	suite.Assert().Nil(err)
}

// InsertSubtree recursively inserts chain state as a subtree of the parent
// block. The subtree has the given depth and `fanout` children at each node.
// All child indices are updated.
func (suite *SnapshotSuite) InsertSubtree(parent model.Block, depth, fanout int) {
	if depth == 0 {
		return
	}

	for i := 0; i < fanout; i++ {
		block := suite.BlockWithParent(&parent)
		suite.InsertBlock(block)
		suite.InsertSubtree(block, depth-1, fanout)
	}
}

func TestSnapshot(t *testing.T) {
	suite.Run(t, new(SnapshotSuite))
}

func (suite *SnapshotSuite) TestNonexistentBlock() {
	t := suite.T()

	nonexistentBlockID := unittest.IdentifierFixture()
	snapshot := suite.state.AtBlockID(nonexistentBlockID)

	_, err := snapshot.Collection()
	assert.Error(t, err)

	_, err = snapshot.Head()
	assert.Error(t, err)
}

func (suite *SnapshotSuite) TestAtBlockID() {
	t := suite.T()

	snapshot := suite.state.AtBlockID(suite.genesis.ID())

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.Nil(t, err)
	assert.Equal(t, &suite.genesis.Payload.Collection, coll)

	// ensure head is correct
	head, err := snapshot.Head()
	assert.Nil(t, err)
	assert.Equal(t, suite.genesis.ID(), head.ID())
}

func (suite *SnapshotSuite) TestEmptyCollection() {
	t := suite.T()

	// create a block with an empty collection
	block := suite.BlockWithParent(suite.genesis)
	block.SetPayload(model.EmptyPayload(flow.ZeroID))
	suite.InsertBlock(block)

	snapshot := suite.state.AtBlockID(block.ID())

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.Nil(t, err)
	assert.Equal(t, &block.Payload.Collection, coll)
}

func (suite *SnapshotSuite) TestFinalizedBlock() {
	t := suite.T()

	// create a new finalized block on genesis (height=1)
	finalizedBlock1 := suite.Block()
	err := suite.state.Extend(&finalizedBlock1)
	assert.Nil(t, err)

	// create an un-finalized block on genesis (height=1)
	unFinalizedBlock1 := suite.Block()
	err = suite.state.Extend(&unFinalizedBlock1)
	assert.Nil(t, err)

	// create a second un-finalized on top of the finalized block (height=2)
	unFinalizedBlock2 := suite.BlockWithParent(&finalizedBlock1)
	err = suite.state.Extend(&unFinalizedBlock2)
	assert.Nil(t, err)

	// finalize the block
	err = suite.db.Update(procedure.FinalizeClusterBlock(finalizedBlock1.ID()))
	assert.Nil(t, err)

	// get the final snapshot, should map to finalizedBlock1
	snapshot := suite.state.Final()

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.Nil(t, err)
	assert.Equal(t, &finalizedBlock1.Payload.Collection, coll)

	// ensure head is correct
	head, err := snapshot.Head()
	assert.Nil(t, err)
	assert.Equal(t, finalizedBlock1.ID(), head.ID())
}

// test that no pending blocks are returned when there are none
func (suite *SnapshotSuite) TestPending_NoPendingBlocks() {

	// first, check that a freshly bootstrapped state has no pending blocks
	suite.Run("freshly bootstrapped state", func() {
		pending, err := suite.state.Final().Pending()
		suite.Require().Nil(err)
		suite.Assert().Len(pending, 0)
	})

}

// test that the appropriate pending blocks are included
func (suite *SnapshotSuite) TestPending_WithPendingBlocks() {

	// check with some finalized blocks
	parent := suite.genesis
	pendings := make([]flow.Identifier, 0, 10)
	for i := 0; i < 10; i++ {
		next := suite.BlockWithParent(parent)
		suite.InsertBlock(next)
		pendings = append(pendings, next.ID())
	}

	pending, err := suite.state.Final().Pending()
	suite.Require().Nil(err)
	suite.Require().Equal(pendings, pending)
}

// ensure that pending blocks are included, even they aren't direct children
// of the finalized head
func (suite *SnapshotSuite) TestPending_Grandchildren() {

	// create 3 levels of children
	suite.InsertSubtree(*suite.genesis, 3, 3)

	pending, err := suite.state.Final().Pending()
	suite.Require().Nil(err)

	// we should have 3 + 3^2 + 3^3 = 39 total children
	suite.Assert().Len(pending, 39)

	// the result must be ordered so that we see parents before their children
	parents := make(map[flow.Identifier]struct{})
	// initialize with the latest finalized block, which is the parent of the
	// first level of children
	parents[suite.genesis.ID()] = struct{}{}

	for _, blockID := range pending {
		var header flow.Header
		err := suite.db.View(operation.RetrieveHeader(blockID, &header))
		suite.Require().Nil(err)

		// we must have already seen the parent
		_, seen := parents[header.ParentID]
		suite.Assert().True(seen, "pending list contained child (%x) before parent (%x)", header.ID(), header.ParentID)

		// mark this block as seen
		parents[header.ID()] = struct{}{}
	}
}

func (suite *SnapshotSuite) TestParams_ChainID() {

	chainID, err := suite.state.Params().ChainID()
	suite.Require().Nil(err)
	suite.Assert().Equal(suite.genesis.Header.ChainID, chainID)
}
