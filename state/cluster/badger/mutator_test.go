package badger

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
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
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	protocolutil "github.com/onflow/flow-go/state/protocol/util"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

type MutatorSuite struct {
	suite.Suite
	db    *badger.DB
	dbdir string

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

	suite.genesis = model.Genesis()
	suite.chainID = suite.genesis.Header.ChainID

	suite.dbdir = unittest.TempDir(suite.T())
	suite.db = unittest.BadgerDB(suite.T(), suite.dbdir)

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	log := zerolog.Nop()
	all := util.StorageLayer(suite.T(), suite.db)
	colPayloads := storage.NewClusterPayloads(metrics, suite.db)

	// just bootstrap with a genesis block, we'll use this as reference
	genesis, result, seal := unittest.BootstrapFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))

	// ensure we don't enter a new epoch for tests that build many blocks
	result.ServiceEvents[0].Event.(*flow.EpochSetup).FinalView = genesis.Header.View + 100_000

	seal.ResultID = result.ID()
	qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(genesis.ID()))
	genesis.Payload.ProtocolStateID = inmem.ProtocolStateFromEpochServiceEvents(
		result.ServiceEvents[0].Event.(*flow.EpochSetup),
		result.ServiceEvents[1].Event.(*flow.EpochCommit),
	).ID()
	rootSnapshot, err := inmem.SnapshotFromBootstrapState(genesis, result, seal, qc)
	require.NoError(suite.T(), err)
	suite.epochCounter = rootSnapshot.Encodable().Epochs.Current.Counter

	suite.protoGenesis = genesis
	state, err := pbadger.Bootstrap(
		metrics,
		suite.db,
		all.Headers,
		all.Seals,
		all.Results,
		all.Blocks,
		all.QuorumCertificates,
		all.Setups,
		all.EpochCommits,
		all.ProtocolState,
		all.VersionBeacons,
		rootSnapshot,
	)
	require.NoError(suite.T(), err)
	suite.protoState, err = pbadger.NewFollowerState(log, tracer, events.NewNoop(), state, all.Index, all.Payloads, protocolutil.MockBlockTimer())
	require.NoError(suite.T(), err)

	suite.mutableProtocolState = protocol_state.NewMutableProtocolStateFactory(
		all.ProtocolState,
		state.Params(),
		all.Headers,
		all.Results,
		all.Setups,
		all.EpochCommits,
	)

	clusterStateRoot, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.NoError(err)
	clusterState, err := Bootstrap(suite.db, clusterStateRoot)
	suite.Assert().Nil(err)
	suite.state, err = NewMutableState(clusterState, tracer, all.Headers, colPayloads)
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

func (suite *MutatorSuite) FinalizeBlock(block model.Block) {
	err := suite.db.Update(func(tx *badger.Txn) error {
		var refBlock flow.Header
		err := operation.RetrieveHeader(block.Payload.ReferenceBlockID, &refBlock)(tx)
		if err != nil {
			return err
		}
		err = procedure.FinalizeClusterBlock(block.ID())(tx)
		if err != nil {
			return err
		}
		err = operation.IndexClusterBlockByReferenceHeight(refBlock.Height, block.ID())(tx)
		return err
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
	suite.genesis.Header.Height = 1

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidParentHash() {
	suite.genesis.Header.ParentID = unittest.IdentifierFixture()

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidPayloadHash() {
	suite.genesis.Header.PayloadHash = unittest.IdentifierFixture()

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_InvalidPayload() {
	// this is invalid because genesis collection should be empty
	suite.genesis.Payload = unittest.ClusterPayloadFixture(2)

	_, err := NewStateRoot(suite.genesis, unittest.QuorumCertificateFixture(), suite.epochCounter)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestBootstrap_Successful() {
	err := suite.db.View(func(tx *badger.Txn) error {

		// should insert collection
		var collection flow.LightCollection
		err := operation.RetrieveCollection(suite.genesis.Payload.Collection.ID(), &collection)(tx)
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

		// should insert block height -> ID lookup
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
	err := suite.state.Extend(&block)
	suite.Assert().Error(err)
}

func (suite *MutatorSuite) TestExtend_InvalidChainID() {
	block := suite.Block()
	// change the chain ID
	block.Header.ChainID = flow.ChainID(fmt.Sprintf("%s-invalid", block.Header.ChainID))

	err := suite.state.Extend(&block)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_InvalidBlockHeight() {
	block := suite.Block()
	// change the block height
	block.Header.Height = block.Header.Height - 1

	err := suite.state.Extend(&block)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

// TestExtend_InvalidParentView tests if mutator rejects block with invalid ParentView. ParentView must be consistent
// with view of block referred by ParentID.
func (suite *MutatorSuite) TestExtend_InvalidParentView() {
	block := suite.Block()
	// change the block parent view
	block.Header.ParentView--

	err := suite.state.Extend(&block)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_DuplicateTxInPayload() {
	block := suite.Block()
	// add the same transaction to a payload twice
	tx := suite.Tx()
	payload := suite.Payload(&tx, &tx)
	block.SetPayload(payload)

	// should fail to extend block with invalid payload
	err := suite.state.Extend(&block)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_OnParentOfFinalized() {
	// build one block on top of genesis
	block1 := suite.Block()
	err := suite.state.Extend(&block1)
	suite.Assert().Nil(err)

	// finalize the block
	suite.FinalizeBlock(block1)

	// insert another block on top of genesis
	// since we have already finalized block 1, this is invalid
	block2 := suite.Block()

	// try to extend with the invalid block
	err = suite.state.Extend(&block2)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsOutdatedExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_Success() {
	block := suite.Block()
	err := suite.state.Extend(&block)
	suite.Assert().Nil(err)

	// should be able to retrieve the block
	var extended model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(block.ID(), &extended))
	suite.Assert().Nil(err)
	suite.Assert().Equal(*block.Payload, *extended.Payload)

	// the block should be indexed by its parent
	var childIDs flow.IdentifierList
	err = suite.db.View(procedure.LookupBlockChildren(suite.genesis.ID(), &childIDs))
	suite.Assert().Nil(err)
	suite.Require().Len(childIDs, 1)
	suite.Assert().Equal(block.ID(), childIDs[0])
}

func (suite *MutatorSuite) TestExtend_WithEmptyCollection() {
	block := suite.Block()
	// set an empty collection as the payload
	block.SetPayload(suite.Payload())
	err := suite.state.Extend(&block)
	suite.Assert().Nil(err)
}

// an unknown reference block is unverifiable
func (suite *MutatorSuite) TestExtend_WithNonExistentReferenceBlock() {
	suite.Run("empty collection", func() {
		block := suite.Block()
		block.Payload.ReferenceBlockID = unittest.IdentifierFixture()
		block.SetPayload(*block.Payload)
		err := suite.state.Extend(&block)
		suite.Assert().Error(err)
		suite.Assert().True(state.IsUnverifiableExtensionError(err))
	})
	suite.Run("non-empty collection", func() {
		block := suite.Block()
		tx := suite.Tx()
		payload := suite.Payload(&tx)
		// set a random reference block ID
		payload.ReferenceBlockID = unittest.IdentifierFixture()
		block.SetPayload(payload)
		err := suite.state.Extend(&block)
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
		err := suite.protoState.ExtendCertified(context.Background(), next, unittest.CertifyBlock(next.Header))
		suite.Require().Nil(err)
		err = suite.protoState.Finalize(context.Background(), next.ID())
		suite.Require().Nil(err)
		parent = next
	}

	block := suite.Block()
	// set genesis as reference block
	block.SetPayload(model.EmptyPayload(suite.protoGenesis.ID()))
	err := suite.state.Extend(&block)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_WithReferenceBlockFromClusterChain() {
	// TODO skipping as this isn't implemented yet
	unittest.SkipUnless(suite.T(), unittest.TEST_TODO, "skipping as this isn't implemented yet")

	block := suite.Block()
	// set genesis from cluster chain as reference block
	block.SetPayload(model.EmptyPayload(suite.genesis.ID()))
	err := suite.state.Extend(&block)
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

	block := suite.Block()
	block.SetPayload(model.EmptyPayload(nextEpochHeader.ID()))
	err = suite.state.Extend(&block)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

// TestExtend_WithUnfinalizedReferenceBlock tests that extending the cluster state
// with a reference block which is un-finalized and above the finalized boundary
// should be considered an unverifiable extension. It's possible that this reference
// block has been finalized, we just haven't processed it yet.
func (suite *MutatorSuite) TestExtend_WithUnfinalizedReferenceBlock() {
	unfinalized := unittest.BlockWithParentProtocolState(suite.protoGenesis)
	err := suite.protoState.ExtendCertified(context.Background(), unfinalized, unittest.CertifyBlock(unfinalized.Header))
	suite.Require().NoError(err)

	block := suite.Block()
	block.SetPayload(model.EmptyPayload(unfinalized.ID()))
	err = suite.state.Extend(&block)
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
	err := suite.protoState.ExtendCertified(context.Background(), orphaned, unittest.CertifyBlock(orphaned.Header))
	suite.Require().NoError(err)

	// create a block extending genesis (conflicting with previous) which is finalized
	finalized := unittest.BlockWithParentProtocolState(suite.protoGenesis)
	finalized.Payload.Guarantees = nil
	finalized.SetPayload(*finalized.Payload)
	err = suite.protoState.ExtendCertified(context.Background(), finalized, unittest.CertifyBlock(finalized.Header))
	suite.Require().NoError(err)
	err = suite.protoState.Finalize(context.Background(), finalized.ID())
	suite.Require().NoError(err)

	// test referencing the orphaned block
	block := suite.Block()
	block.SetPayload(model.EmptyPayload(orphaned.ID()))
	err = suite.state.Extend(&block)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_UnfinalizedBlockWithDupeTx() {
	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	block1 := suite.Block()
	payload1 := suite.Payload(&tx1)
	block1.SetPayload(payload1)

	// should be able to extend block 1
	err := suite.state.Extend(&block1)
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := suite.BlockWithParent(&block1)
	payload2 := suite.Payload(&tx1)
	block2.SetPayload(payload2)

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.state.Extend(&block2)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_FinalizedBlockWithDupeTx() {
	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	block1 := suite.Block()
	payload1 := suite.Payload(&tx1)
	block1.SetPayload(payload1)

	// should be able to extend block 1
	err := suite.state.Extend(&block1)
	suite.Assert().Nil(err)

	// should be able to finalize block 1
	suite.FinalizeBlock(block1)
	suite.Assert().Nil(err)

	// create a block building on block1 ALSO containing tx1
	block2 := suite.BlockWithParent(&block1)
	payload2 := suite.Payload(&tx1)
	block2.SetPayload(payload2)

	// should be unable to extend block 2, as it contains a dupe transaction
	err = suite.state.Extend(&block2)
	suite.Assert().Error(err)
	suite.Assert().True(state.IsInvalidExtensionError(err))
}

func (suite *MutatorSuite) TestExtend_ConflictingForkWithDupeTx() {
	tx1 := suite.Tx()

	// create a block extending genesis containing tx1
	block1 := suite.Block()
	payload1 := suite.Payload(&tx1)
	block1.SetPayload(payload1)

	// should be able to extend block 1
	err := suite.state.Extend(&block1)
	suite.Assert().Nil(err)

	// create a block ALSO extending genesis ALSO containing tx1
	block2 := suite.Block()
	payload2 := suite.Payload(&tx1)
	block2.SetPayload(payload2)

	// should be able to extend block2
	// although it conflicts with block1, it is on a different fork
	err = suite.state.Extend(&block2)
	suite.Assert().Nil(err)
}

func (suite *MutatorSuite) TestExtend_LargeHistory() {
	t := suite.T()

	// get a valid reference block ID
	final, err := suite.protoState.Final().Head()
	require.NoError(t, err)
	refID := final.ID()

	// keep track of the head of the chain
	head := *suite.genesis

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
		parent := head
		if conflicting {
			err = suite.db.View(procedure.RetrieveClusterBlock(parent.Header.ParentID, &parent))
			assert.NoError(t, err)
			// add the transaction to the invalidated list
			invalidatedTransactions = append(invalidatedTransactions, &tx)
		} else if head.Header.Height < 50 {
			oldTransactions = append(oldTransactions, &tx)
		}

		// create a block containing the transaction
		block := unittest.ClusterBlockWithParent(&head)
		payload := suite.Payload(&tx)
		block.SetPayload(payload)
		err = suite.state.Extend(&block)
		assert.NoError(t, err)

		// reset the valid head if we aren't building a conflicting fork
		if !conflicting {
			head = block
			suite.FinalizeBlock(block)
			assert.NoError(t, err)
		}

		// stop building blocks once we've built a history which exceeds the transaction
		// expiry length - this tests that deduplication works properly against old blocks
		// which nevertheless have a potentially conflicting reference block
		if head.Header.Height > flow.DefaultTransactionExpiry+100 {
			break
		}
	}

	t.Log("conflicting: ", len(invalidatedTransactions))

	t.Run("should be able to extend with transactions in orphaned forks", func(t *testing.T) {
		block := unittest.ClusterBlockWithParent(&head)
		payload := suite.Payload(invalidatedTransactions...)
		block.SetPayload(payload)
		err = suite.state.Extend(&block)
		assert.NoError(t, err)
	})

	t.Run("should be unable to extend with conflicting transactions within reference height range of extending block", func(t *testing.T) {
		block := unittest.ClusterBlockWithParent(&head)
		payload := suite.Payload(oldTransactions...)
		block.SetPayload(payload)
		err = suite.state.Extend(&block)
		assert.Error(t, err)
		suite.Assert().True(state.IsInvalidExtensionError(err))
	})
}
