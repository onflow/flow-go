package badger

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	model "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	protocolutil "github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

type MutatorSuite struct {
	suite.Suite
	db          storage.DB
	dbdir       string
	lockManager lockctx.Manager

	genesis      *model.Block
	chainID      flow.ChainID
	epochCounter uint64

	// protocol state for reference blocks for transactions
	protoState           protocol.FollowerState
	mutableProtocolState protocol.MutableProtocolState
	protoGenesis         *flow.Block

	state cluster.MutableState
}

// runs before each test runs
func (suite *MutatorSuite) SetupTest() {
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
	log := zerolog.Nop()
	all := store.InitAll(metrics, suite.db)
	colPayloads := store.NewClusterPayloads(metrics, suite.db)

	// just bootstrap with a genesis block, we'll use this as reference
	genesis, result, seal := unittest.BootstrapFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))

	// ensure we don't enter a new epoch for tests that build many blocks
	result.ServiceEvents[0].Event.(*flow.EpochSetup).FinalView = genesis.View + 100_000

	seal.ResultID = result.ID()
	qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(genesis.ID()))
	safetyParams, err := protocol.DefaultEpochSafetyParams(genesis.ChainID)
	require.NoError(suite.T(), err)
	minEpochStateEntry, err := inmem.EpochProtocolStateFromServiceEvents(
		result.ServiceEvents[0].Event.(*flow.EpochSetup),
		result.ServiceEvents[1].Event.(*flow.EpochCommit),
	)
	require.NoError(suite.T(), err)
	rootProtocolState, err := kvstore.NewDefaultKVStore(
		safetyParams.FinalizationSafetyThreshold,
		safetyParams.EpochExtensionViewCount,
		minEpochStateEntry.ID(),
	)
	require.NoError(suite.T(), err)
	genesis.Payload.ProtocolStateID = rootProtocolState.ID()
	rootSnapshot, err := unittest.SnapshotFromBootstrapState(genesis, result, seal, qc)
	require.NoError(suite.T(), err)
	suite.epochCounter = rootSnapshot.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.EpochCounter()

	suite.protoGenesis = genesis
	state, err := pbadger.Bootstrap(
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
		rootSnapshot,
	)
	require.NoError(suite.T(), err)
	suite.protoState, err = pbadger.NewFollowerState(
		log, tracer, events.NewNoop(), state, all.Index, all.Payloads, protocolutil.MockBlockTimer(),
	)
	require.NoError(suite.T(), err)

	suite.mutableProtocolState = protocol_state.NewMutableProtocolState(
		log,
		all.EpochProtocolStateEntries,
		all.ProtocolKVStore,
		state.Params(),
		all.Headers,
		all.Results,
		all.EpochSetups,
		all.EpochCommits,
	)

	clusterStateRoot, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.NoError(err)
	clusterState, err := Bootstrap(suite.db, suite.lockManager, clusterStateRoot)
	suite.Assert().Nil(err)
	suite.state, err = NewMutableState(clusterState, suite.lockManager, tracer, all.Headers, colPayloads)
	suite.Assert().Nil(err)
}

// runs after each test finishes
func (suite *MutatorSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().Nil(err)
	err = os.RemoveAll(suite.dbdir)
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

	// avoid a nil transaction list
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

// ProposalWithParent returns a valid block proposal with the given parent and the given payload.
func (suite *MutatorSuite) ProposalWithParentAndPayload(parent *model.Block, payload model.Payload) model.Proposal {
	block := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(parent),
		unittest.ClusterBlock.WithPayload(payload),
	)
	return *unittest.ClusterProposalFromBlock(block)
}

// Proposal returns a valid cluster block proposal with genesis as parent.
func (suite *MutatorSuite) Proposal() model.Proposal {
	return suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload())
}

func (suite *MutatorSuite) FinalizeBlock(block model.Block) {
	var refBlock flow.Header
	err := operation.RetrieveHeader(suite.db.Reader(), block.Payload.ReferenceBlockID, &refBlock)
	suite.Require().Nil(err)

	lctx := suite.lockManager.NewContext()
	defer lctx.Release()
	require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
	err = suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err = procedure.FinalizeClusterBlock(lctx, rw, block.ID())
		if err != nil {
			return err
		}
		return operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), refBlock.Height, block.ID())
	})
	suite.Assert().NoError(err)
}

func (suite *MutatorSuite) Tx(opts ...func(*flow.TransactionBody)) flow.TransactionBody {
	final, err := suite.protoState.Final().Head()
	suite.Require().Nil(err)

	tx := unittest.TransactionBodyFixture(opts...)
	tx.ReferenceBlockID = final.ID()
	return tx
}

func TestMutator(t *testing.T) {
	suite.Run(t, new(MutatorSuite))
}

func (suite *MutatorSuite) TestBootstrap_InvalidHeight() {
	suite.genesis.Height = 1

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidParentHash() {
	suite.genesis.ParentID = unittest.IdentifierFixture()

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidPayload() {
	// this is invalid because genesis collection should be empty
	suite.genesis.Payload = *unittest.ClusterPayloadFixture(2)

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

// TestBootstrap_Successful verifies that basic information is successfully persisted during bootstrapping.
//  1. The collector's root block was inserted and indexed. Specifically:
//     - The collection contained in the root block can be retrieved by its ID.
//     - The transactions contained in the root block's collection can be looked up by the root block's ID.
//     - The root block's header can be retrieved by its ID.
//     (The payload, i.e. the collection, we already retrieved above.)
//     - The root block can be looked up by its height.
//     - The latest finalized cluster block height should be the height of the root block.
func (suite *MutatorSuite) TestBootstrap_Successful() {
	err := (func(r storage.Reader) error {
		// Bootstrapping should have inserted the collection contained in the root block
		collection := new(flow.LightCollection)
		err := operation.RetrieveCollection(r, suite.genesis.Payload.Collection.ID(), collection)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Payload.Collection.Light(), collection)

		// Bootstrapping should have indexed the transactions contained in the collector's root block.
		var txIDs []flow.Identifier // reset the collection
		err = operation.LookupCollectionPayload(r, suite.genesis.ID(), &txIDs)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Payload.Collection.Light(), collection)

		// Bootstrapping should have inserted the collector's root block header
		var header flow.Header
		err = operation.RetrieveHeader(r, suite.genesis.ID(), &header)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.ToHeader().ID(), header.ID())

		// Bootstrapping should have indexed the root block's by the root block's height.
		var blockID flow.Identifier
		err = operation.LookupClusterBlockHeight(r, suite.genesis.ChainID, suite.genesis.Height, &blockID)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.ID(), blockID)

		// As the latest finalized cluster block height, bootstrapping should have indexed the root block.
		var latestFinalizedClusterBlockHeight uint64
		err = operation.RetrieveClusterFinalizedHeight(r, suite.genesis.ChainID, &latestFinalizedClusterBlockHeight)
		suite.Assert().Nil(err)
		suite.Assert().Equal(suite.genesis.Height, latestFinalizedClusterBlockHeight)

		return nil
	})(suite.db.Reader())
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_WithoutBootstrap() {
	block := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(suite.genesis),
	)
	err := suite.state.Extend(unittest.ClusterProposalFromBlock(block))
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_InvalidChainID() {
	proposal := suite.Proposal()
	// change the chain ID
	proposal.Block.ChainID = flow.ChainID(fmt.Sprintf("%s-invalid", proposal.Block.ChainID))

	err := suite.state.Extend(&proposal)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_InvalidBlockHeight() {
	proposal := suite.Proposal()
	// change the block height
	proposal.Block.Height = proposal.Block.Height + 1

	err := suite.state.Extend(&proposal)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

// TestExtend_InvalidParentView tests if mutator rejects block with invalid ParentView. ParentView must be consistent
// with view of block referred by ParentID.
func (suite *MutatorSuite) TestExtend_InvalidParentView() {
	tx1 := suite.Tx()
	tx2 := suite.Tx()

	proposal1 := suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload(&tx1))

	err := suite.state.Extend(&proposal1)
	suite.Assert().Nil(err)

	suite.FinalizeBlock(proposal1.Block)
	suite.Assert().Nil(err)

	proposal2 := suite.ProposalWithParentAndPayload(&proposal1.Block, suite.Payload(&tx2))
	// change the block ParentView
	proposal2.Block.ParentView--

	err = suite.state.Extend(&proposal2)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_DuplicateTxInPayload() {
	// add the same transaction to a payload twice
	tx := suite.Tx()
	proposal := suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload(&tx, &tx))

	// should fail to extend block with invalid payload
	err := suite.state.Extend(&proposal)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_OnParentOfFinalized() {
	// build one block on top of genesis
	proposal1 := suite.Proposal()
	err := suite.state.Extend(&proposal1)
	suite.Assert().Nil(err)

	// finalize the block
	suite.FinalizeBlock(proposal1.Block)

	// insert another block on top of genesis
	// since we have already finalized block 1, this is invalid
	proposal2 := suite.Proposal()

	// try to extend with the invalid block
	err = suite.state.Extend(&proposal2)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsOutdatedExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_Success() {
	proposal := suite.Proposal()
	err := suite.state.Extend(&proposal)
	suite.Assert().Nil(err)

	// should be able to retrieve the block
	r := suite.db.Reader()
	var extended model.Block
	err = procedure.RetrieveClusterBlock(r, proposal.Block.ID(), &extended)
	suite.Assert().Nil(err)
	suite.Assert().Equal(proposal.Block.Payload, extended.Payload)

	// the block should be indexed by its parent
	var childIDs flow.IdentifierList
	err = procedure.LookupBlockChildren(r, suite.genesis.ID(), &childIDs)
	suite.Assert().Nil(err)
	suite.Require().Len(childIDs, 1)
	suite.Assert().Equal(proposal.Block.ID(), childIDs[0])
}

func (suite *MutatorSuite) TestExtend_WithEmptyCollection() {
	// set an empty collection as the payload
	proposal := suite.Proposal()
	err := suite.state.Extend(&proposal)
	suite.Assert().Nil(err)
}

// an unknown reference block is unverifiable
func (suite *MutatorSuite) TestExtend_WithNonExistentReferenceBlock() {
	suite.Run("empty collection", func() {
		payload := suite.Payload()
		payload.ReferenceBlockID = unittest.IdentifierFixture()
		proposal := suite.ProposalWithParentAndPayload(suite.genesis, payload)
		err := suite.state.Extend(&proposal)
		suite.Assert().Error(err)
		suite.Assert().True(state.IsUnverifiableExtensionError(err))
	})
	suite.Run("non-empty collection", func() {
		tx := suite.Tx()
		payload := suite.Payload(&tx)
		// set a random reference block ID
		payload.ReferenceBlockID = unittest.IdentifierFixture()
		proposal := suite.ProposalWithParentAndPayload(suite.genesis, payload)
		err := suite.state.Extend(&proposal)
		suite.Assert().Error(err)
		suite.Assert().True(state.IsUnverifiableExtensionError(err))
	})
}

// a collection with an expired reference block is a VALID extension of chain state
func (suite *MutatorSuite) TestExtend_WithExpiredReferenceBlock() {
	// build enough blocks so that using genesis as a reference block causes
	// the collection to be expired
	parent := suite.protoGenesis
	for i := 0; i < flow.DefaultTransactionExpiry+1; i++ {
		next := unittest.BlockWithParentProtocolState(parent)
		err := suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(next))
		suite.Require().Nil(err)
		err = suite.protoState.Finalize(context.Background(), next.ID())
		suite.Require().Nil(err)
		parent = next
	}

	// set genesis as reference block
	proposal := suite.ProposalWithParentAndPayload(suite.genesis, *model.NewEmptyPayload(suite.protoGenesis.ID()))
	err := suite.state.Extend(&proposal)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_WithReferenceBlockFromClusterChain() {
	// TODO skipping as this isn't implemented yet
	unittest.SkipUnless(suite.T(), unittest.TEST_TODO, "skipping as this isn't implemented yet")
	// set genesis from cluster chain as reference block
	proposal := suite.ProposalWithParentAndPayload(suite.genesis, *model.NewEmptyPayload(suite.genesis.ID()))
	err := suite.state.Extend(&proposal)
	suite.Assert().Error(err)
}

// TestExtend_WithReferenceBlockFromDifferentEpoch tests extending the cluster state
// using a reference block in a different epoch than the cluster's epoch.
func (suite *MutatorSuite) TestExtend_WithReferenceBlockFromDifferentEpoch() {
	// build and complete the current epoch, then use a reference block from next epoch
	eb := unittest.NewEpochBuilder(suite.T(), suite.mutableProtocolState, suite.protoState)
	eb.BuildEpoch().CompleteEpoch()
	heights, ok := eb.EpochHeights(1)
	require.True(suite.T(), ok)
	nextEpochHeader, err := suite.protoState.AtHeight(heights.FinalHeight() + 1).Head()
	require.NoError(suite.T(), err)

	proposal := suite.ProposalWithParentAndPayload(suite.genesis, *model.NewEmptyPayload(nextEpochHeader.ID()))
	err = suite.state.Extend(&proposal)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

// TestExtend_WithUnfinalizedReferenceBlock tests that extending the cluster state
// with a reference block which is un-finalized and above the finalized boundary
// should be considered an unverifiable extension. It's possible that this reference
// block has been finalized, we just haven't processed it yet.
func (suite *MutatorSuite) TestExtend_WithUnfinalizedReferenceBlock() {
	unfinalized := unittest.BlockWithParentProtocolState(suite.protoGenesis)
	err := suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(unfinalized))
	suite.Require().NoError(err)

	proposal := suite.ProposalWithParentAndPayload(suite.genesis, *model.NewEmptyPayload(unfinalized.ID()))
	err = suite.state.Extend(&proposal)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsUnverifiableExtensionError(err))
}

// TestExtend_WithOrphanedReferenceBlock tests that extending the cluster state
// with a un-finalized reference block below the finalized boundary
// (i.e. orphaned) should be considered an invalid extension. As the proposer is supposed
// to only use finalized blocks as reference, the proposer knowingly generated an invalid
func (suite *MutatorSuite) TestExtend_WithOrphanedReferenceBlock() {
	// create a block extending genesis which is not finalized
	orphaned := unittest.BlockWithParentProtocolState(suite.protoGenesis)
	err := suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(orphaned))
	suite.Require().NoError(err)

	// create a block extending genesis (conflicting with previous) which is finalized
	finalized := unittest.BlockWithParentProtocolState(suite.protoGenesis)
	finalized.Payload.Guarantees = nil
	err = suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(finalized))
	suite.Require().NoError(err)
	err = suite.protoState.Finalize(context.Background(), finalized.ID())
	suite.Require().NoError(err)

	// test referencing the orphaned block
	proposal := suite.ProposalWithParentAndPayload(suite.genesis, *model.NewEmptyPayload(orphaned.ID()))
	err = suite.state.Extend(&proposal)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_UnfinalizedBlockWithDupeTx() {
	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	proposal1 := suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload(&tx1))

	// should be able to extend block 1
	err := suite.state.Extend(&proposal1)
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	proposal2 := suite.ProposalWithParentAndPayload(&proposal1.Block, suite.Payload(&tx1))

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.state.Extend(&proposal2)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_FinalizedBlockWithDupeTx() {
	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	proposal1 := suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload(&tx1))

	// should be able to extend block 1
	err := suite.state.Extend(&proposal1)
	suite.Assert().Nil(err)

	// should be able to finalize block 1
	suite.FinalizeBlock(proposal1.Block)
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(&proposal1.Block),
		unittest.ClusterBlock.WithPayload(suite.Payload(&tx1)),
	)
	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.state.Extend(unittest.ClusterProposalFromBlock(block2))
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_ConflictingForkWithDupeTx() {
	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	proposal1 := suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload(&tx1))

	// should be able to extend block 1
	err := suite.state.Extend(&proposal1)
	suite.Assert().Nil(err)

	// create a block ALSO extending genesis ALSO containing tx1
	proposal2 := suite.ProposalWithParentAndPayload(suite.genesis, suite.Payload(&tx1))

	// should be able to extend block2
	// although it conflicts with block1, it is on a different fork
	err = suite.state.Extend(&proposal2)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_LargeHistory() {
	t := suite.T()

	// get a valid reference block ID
	final, err := suite.protoState.Final().Head()
	require.NoError(t, err)
	refID := final.ID()

	// keep track of the head of the chain
	head := suite.genesis

	// keep track of transactions in orphaned forks (eligible for inclusion in future block)
	var invalidatedTransactions []*flow.TransactionBody
	// keep track of the oldest transactions (further back in ancestry than the expiry window)
	var oldTransactions []*flow.TransactionBody

	// create a large history of blocks with invalidated forks every 3 blocks on
	// average - build until the height exceeds transaction expiry
	for i := 0; ; i++ {

		// create a transaction
		tx := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
			tx.ReferenceBlockID = refID
			tx.ProposalKey.SequenceNumber = uint64(i)
		})

		// 1/3 of the time create a conflicting fork that will be invalidated
		// don't do this the first and last few times to ensure we don't
		// try to fork genesis and the last block is the valid fork.
		conflicting := rand.Intn(3) == 0 && i > 5 && i < 995

		// by default, build on the head - if we are building a
		// conflicting fork, build on the parent of the head
		parent := *head
		if conflicting {
			err = procedure.RetrieveClusterBlock(suite.db.Reader(), parent.ParentID, &parent)
			assert.NoError(t, err)
			// add the transaction to the invalidated list
			invalidatedTransactions = append(invalidatedTransactions, &tx)
		} else if head.Height < 50 {
			oldTransactions = append(oldTransactions, &tx)
		}

		// create a block containing the transaction
		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(head),
			unittest.ClusterBlock.WithPayload(suite.Payload(&tx)),
		)
		err = suite.state.Extend(unittest.ClusterProposalFromBlock(block))
		assert.NoError(t, err)

		// reset the valid head if we aren't building a conflicting fork
		if !conflicting {
			head = block
			suite.FinalizeBlock(*block)
			assert.NoError(t, err)
		}

		// stop building blocks once we've built a history which exceeds the transaction
		// expiry length - this tests that deduplication works properly against old blocks
		// which nevertheless have a potentially conflicting reference block
		if head.Height > flow.DefaultTransactionExpiry+100 {
			break
		}
	}

	t.Log("conflicting: ", len(invalidatedTransactions))

	t.Run("should be able to extend with transactions in orphaned forks", func(t *testing.T) {
		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(head),
			unittest.ClusterBlock.WithPayload(suite.Payload(invalidatedTransactions...)),
		)
		err = suite.state.Extend(unittest.ClusterProposalFromBlock(block))
		assert.NoError(t, err)
	})

	t.Run("should be unable to extend with conflicting transactions within reference height range of extending block", func(t *testing.T) {
		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(head),
			unittest.ClusterBlock.WithPayload(suite.Payload(oldTransactions...)),
		)
		err = suite.state.Extend(unittest.ClusterProposalFromBlock(block))
		assert.Error(t, err)
		suite.Assert().True(state.IsInvalidExtensionError(err))
	})
}
