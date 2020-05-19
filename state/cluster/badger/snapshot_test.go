package badger

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/state/cluster"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type SnapshotSuite struct {
	suite.Suite
	db    *badger.DB
	dbdir string

	genesis *model.Block
	chainID string

	state   cluster.State
	mutator cluster.Mutator
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

	headers := storage.NewHeaders(metrics, suite.db)
	payloads := storage.NewClusterPayloads(metrics, suite.db)

	suite.state, err = NewState(suite.db, suite.chainID, headers, payloads)
	suite.Assert().Nil(err)
	suite.mutator = suite.state.Mutate()

	suite.Bootstrap()
}

// runs after each test finishes
func (suite *SnapshotSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().Nil(err)
	err = os.RemoveAll(suite.dbdir)
	suite.Assert().Nil(err)
}

func (suite *SnapshotSuite) Bootstrap() {
	err := suite.mutator.Bootstrap(suite.genesis)
	suite.Assert().Nil(err)
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
		block := unittest.ClusterBlockWithParent(&parent)
		suite.InsertBlock(block)
		err := suite.db.Update(procedure.IndexBlockChild(parent.ID(), block.ID()))
		suite.Require().Nil(err)
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
	block := unittest.ClusterBlockWithParent(suite.genesis)
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
	finalizedBlock1 := unittest.ClusterBlockWithParent(suite.genesis)
	err := suite.mutator.Extend(&finalizedBlock1)
	assert.Nil(t, err)

	// create an un-finalized block on genesis (height=1)
	unFinalizedBlock1 := unittest.ClusterBlockWithParent(suite.genesis)
	err = suite.mutator.Extend(&unFinalizedBlock1)
	assert.Nil(t, err)

	// create a second un-finalized on top of the finalized block (height=2)
	unFinalizedBlock2 := unittest.ClusterBlockWithParent(&finalizedBlock1)
	t.Logf("header: %x", unFinalizedBlock2.Header.PayloadHash)
	t.Logf("payload: %x", unFinalizedBlock2.Payload.Hash())
	err = suite.mutator.Extend(&unFinalizedBlock2)
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

	// check with some finalized blocks
	suite.Run("with some chain history", func() {
		parent := suite.genesis
		for i := 0; i < 10; i++ {
			next := unittest.ClusterBlockWithParent(parent)
			suite.InsertBlock(next)
			parent = &next
		}

		pending, err := suite.state.Final().Pending()
		suite.Require().Nil(err)
		suite.Assert().Len(pending, 0)
	})
}

// test that the appropriate pending blocks are included
func (suite *SnapshotSuite) TestPending_WithPendingBlocks() {

	// build a block that wasn't validated by hotstuff, thus wasn't indexed as a child
	unvalidated := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(unvalidated)

	// build a block that was validated by hotstuff, is indexed as a child
	validated := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(validated)
	err := suite.db.Update(procedure.IndexBlockChild(suite.genesis.ID(), validated.ID()))
	suite.Assert().Nil(err)

	// only the validated block should be included
	pending, err := suite.state.Final().Pending()
	suite.Require().Nil(err)
	suite.Require().Len(pending, 1)
	suite.Assert().Equal(validated.ID(), pending[0])
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
