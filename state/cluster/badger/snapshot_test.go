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
	"github.com/dapperlabs/flow-go/state/cluster"
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
	suite.chainID = suite.genesis.ChainID

	suite.db, suite.dbdir = unittest.TempBadgerDB(suite.T())

	suite.state, err = NewState(suite.db, suite.chainID)
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
	assert.Equal(t, &suite.genesis.Collection, coll)

	// ensure head is correct
	head, err := snapshot.Head()
	assert.Nil(t, err)
	assert.Equal(t, suite.genesis.ID(), head.ID())
}

func (suite *SnapshotSuite) TestEmptyCollection() {
	t := suite.T()

	// create a block with an empty collection
	block := unittest.ClusterBlockWithParent(suite.genesis)
	block.SetPayload(model.EmptyPayload())
	suite.InsertBlock(block)

	snapshot := suite.state.AtBlockID(block.ID())

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.Nil(t, err)
	assert.Equal(t, &block.Collection, coll)
}

func (suite *SnapshotSuite) TestFinalizedBlock() {
	t := suite.T()

	// create a new finalized block on genesis (height=1)
	finalizedBlock1 := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(finalizedBlock1)
	err := suite.mutator.Extend(finalizedBlock1.ID())
	assert.Nil(t, err)

	// create an un-finalized block on genesis (height=1)
	unFinalizedBlock1 := unittest.ClusterBlockWithParent(suite.genesis)
	suite.InsertBlock(unFinalizedBlock1)
	err = suite.mutator.Extend(unFinalizedBlock1.ID())
	assert.Nil(t, err)

	// create a second un-finalized on top of the finalized block (height=2)
	unFinalizedBlock2 := unittest.ClusterBlockWithParent(&finalizedBlock1)
	suite.InsertBlock(unFinalizedBlock2)
	err = suite.mutator.Extend(unFinalizedBlock2.ID())
	assert.Nil(t, err)

	// finalize the block
	err = suite.db.Update(procedure.FinalizeClusterBlock(finalizedBlock1.ID()))
	assert.Nil(t, err)

	// get the final snapshot, should map to finalizedBlock1
	snapshot := suite.state.Final()

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.Nil(t, err)
	assert.Equal(t, &finalizedBlock1.Collection, coll)

	// ensure head is correct
	head, err := snapshot.Head()
	assert.Nil(t, err)
	assert.Equal(t, finalizedBlock1.ID(), head.ID())
}
