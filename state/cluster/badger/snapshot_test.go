package badger

import (
	"math"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
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
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

type SnapshotSuite struct {
	suite.Suite

	db          storage.DB
	dbdir       string
	lockManager lockctx.Manager

	genesis      *model.Block
	chainID      flow.ChainID
	epochCounter uint64

	protoState protocol.State

	state cluster.MutableState
}

// runs before each test runs
func (suite *SnapshotSuite) SetupTest() {
	var err error

	suite.genesis, err = unittest.ClusterBlock.Genesis()
	require.NoError(suite.T(), err)
	suite.chainID = suite.genesis.ChainID

	suite.dbdir = unittest.TempDir(suite.T())
	pdb := unittest.PebbleDB(suite.T(), suite.dbdir)
	suite.db = pebbleimpl.ToDB(pdb)
	suite.lockManager = storage.NewTestingLockManager()

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	all := store.InitAll(metrics, suite.db)
	colPayloads := store.NewClusterPayloads(metrics, suite.db)

	root := unittest.RootSnapshotFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))
	suite.epochCounter = root.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.EpochCounter()

	suite.protoState, err = pbadger.Bootstrap(
		metrics,
		suite.db,
		suite.lockManager,
		all.Headers,
		all.Seals,
		all.Results,
		all.Blocks,
		all.QuorumCertificates,
		all.EpochSetups,
		all.EpochCommits,
		all.EpochProtocolStateEntries,
		all.ProtocolKVStore,
		all.VersionBeacons,
		root,
	)
	suite.Require().NoError(err)

	clusterStateRoot, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Require().NoError(err)
	clusterState, err := Bootstrap(suite.db, suite.lockManager, clusterStateRoot)
	suite.Require().NoError(err)
	suite.state, err = NewMutableState(clusterState, suite.lockManager, tracer, all.Headers, colPayloads)
	suite.Require().NoError(err)
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

	// avoid a nil transaction list to match empty (but non-nil) list returned by snapshot query
	if len(transactions) == 0 {
		transactions = []*flow.TransactionBody{}
	}

	payload, err := model.NewPayload(
		model.UntrustedPayload{
			ReferenceBlockID: minRefID,
			Collection:       flow.Collection{Transactions: transactions},
		},
	)
	suite.Assert().NoError(err)

	return *payload
}

// ProposalWithParentAndPayload returns a valid block proposal with the given parent and payload.
func (suite *SnapshotSuite) ProposalWithParentAndPayload(parent *model.Block, payload model.Payload) model.Proposal {
	block := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(parent),
		unittest.ClusterBlock.WithPayload(payload),
	)
	return *unittest.ClusterProposalFromBlock(block)
}

// Proposal returns a valid cluster block proposal with genesis as parent.
func (suite *SnapshotSuite) Proposal() model.Proposal {
	return suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload())
}

func (suite *SnapshotSuite) InsertBlock(proposal model.Proposal) {
	lctx := suite.lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock)
	suite.Assert().Nil(err)
	err = suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return procedure.InsertClusterBlock(lctx, rw, &proposal)
	})
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
		proposal := suite.ProposalWithParentAndPayload(&parent, suite.Payload())
		suite.InsertBlock(proposal)
		suite.InsertSubtree(proposal.Block, depth-1, fanout)
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
	assert.NoError(t, err)
	assert.Equal(t, &suite.genesis.Payload.Collection, coll)

	// ensure head is correct
	head, err := snapshot.Head()
	assert.NoError(t, err)
	assert.Equal(t, suite.genesis.ToHeader().ID(), head.ID())
}

func (suite *SnapshotSuite) TestEmptyCollection() {
	t := suite.T()

	// create a block with an empty collection
	proposal := suite.ProposalWithParentAndPayload(suite.genesis, *model.NewEmptyPayload(flow.ZeroID))
	suite.InsertBlock(proposal)

	snapshot := suite.state.AtBlockID(proposal.Block.ID())

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.NoError(t, err)
	assert.Equal(t, &proposal.Block.Payload.Collection, coll)
}

func (suite *SnapshotSuite) TestFinalizedBlock() {
	t := suite.T()

	// create a new finalized block on genesis (height=1)
	finalizedProposal1 := suite.Proposal()
	err := suite.state.Extend(&finalizedProposal1)
	assert.NoError(t, err)

	// create an un-finalized block on genesis (height=1)
	unFinalizedProposal1 := suite.Proposal()
	err = suite.state.Extend(&unFinalizedProposal1)
	assert.NoError(t, err)

	// create a second un-finalized on top of the finalized block (height=2)
	unFinalizedProposal2 := suite.ProposalWithParentAndPayload(&finalizedProposal1.Block, suite.Payload())
	err = suite.state.Extend(&unFinalizedProposal2)
	assert.NoError(t, err)

	// finalize the block
	lctx := suite.lockManager.NewContext()
	defer lctx.Release()
	require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
	err = suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return procedure.FinalizeClusterBlock(lctx, rw, finalizedProposal1.Block.ID())
	})
	assert.NoError(t, err)

	// get the final snapshot, should map to finalizedProposal1
	snapshot := suite.state.Final()

	// ensure collection is correct
	coll, err := snapshot.Collection()
	assert.NoError(t, err)
	assert.Equal(t, &finalizedProposal1.Block.Payload.Collection, coll)

	// ensure head is correct
	head, err := snapshot.Head()
	assert.NoError(t, err)
	assert.Equal(t, finalizedProposal1.Block.ToHeader().ID(), head.ID())
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
		next := suite.ProposalWithParentAndPayload(parent, suite.Payload())
		suite.InsertBlock(next)
		pendings = append(pendings, next.Block.ID())
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
		err := operation.RetrieveHeader(suite.db.Reader(), blockID, &header)
		suite.Require().Nil(err)

		// we must have already seen the parent
		_, seen := parents[header.ParentID]
		suite.Assert().True(seen, "pending list contained child (%x) before parent (%x)", blockID, header.ParentID)

		// mark this block as seen
		parents[blockID] = struct{}{}
	}
}

func (suite *SnapshotSuite) TestParams_ChainID() {
	chainID := suite.state.Params().ChainID()
	suite.Assert().Equal(suite.genesis.ChainID, chainID)
}
