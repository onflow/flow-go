package collection_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotstuffmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	model "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	builder "github.com/onflow/flow-go/module/builder/collection"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/state/cluster"
	clusterkv "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/procedure"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

var signer = func(*flow.Header) ([]byte, error) { return unittest.SignatureFixture(), nil }
var setter = func(h *flow.HeaderBodyBuilder) error {
	h.WithHeight(42).
		WithChainID(flow.Emulator).
		WithParentID(unittest.IdentifierFixture()).
		WithView(1337).
		WithParentView(1336).
		WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
		WithParentVoterSigData(unittest.QCSigDataFixture()).
		WithProposerID(unittest.IdentifierFixture())

	return nil
}

type BuilderSuite struct {
	suite.Suite
	db          storage.DB
	dbdir       string
	lockManager lockctx.Manager

	genesis      *model.Block
	chainID      flow.ChainID
	epochCounter uint64

	headers  storage.Headers
	payloads storage.ClusterPayloads
	blocks   storage.Blocks

	state cluster.MutableState

	// protocol state for reference blocks for transactions
	protoState protocol.FollowerState

	pool    mempool.Transactions
	builder *builder.Builder
}

// runs before each test runs
func (suite *BuilderSuite) SetupTest() {
	fmt.Println("SetupTest>>>>")
	suite.lockManager = storage.NewTestingLockManager()
	var err error

	suite.genesis, err = unittest.ClusterBlock.Genesis()
	require.NoError(suite.T(), err)
	suite.chainID = suite.genesis.ChainID

	suite.pool = herocache.NewTransactions(1000, unittest.Logger(), metrics.NewNoopCollector())

	suite.dbdir = unittest.TempDir(suite.T())
	pdb := unittest.PebbleDB(suite.T(), suite.dbdir)
	suite.db = pebbleimpl.ToDB(pdb)

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	log := zerolog.Nop()

	all := store.InitAll(metrics, suite.db)
	consumer := events.NewNoop()

	suite.headers = all.Headers
	suite.blocks = all.Blocks
	suite.payloads = store.NewClusterPayloads(metrics, suite.db)

	// just bootstrap with a genesis block, we'll use this as reference
	root, result, seal := unittest.BootstrapFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))
	// ensure we don't enter a new epoch for tests that build many blocks
	result.ServiceEvents[0].Event.(*flow.EpochSetup).FinalView = root.View + 100000
	seal.ResultID = result.ID()
	safetyParams, err := protocol.DefaultEpochSafetyParams(root.ChainID)
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
	root.Payload.ProtocolStateID = rootProtocolState.ID()
	rootSnapshot, err := unittest.SnapshotFromBootstrapState(root, result, seal, unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(root.ID())))
	require.NoError(suite.T(), err)
	suite.epochCounter = rootSnapshot.Encodable().SealingSegment.LatestProtocolStateEntry().EpochEntry.EpochCounter()

	require.NoError(suite.T(), err)
	clusterQC := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(suite.genesis.ID()))
	clusterStateRoot, err := clusterkv.NewStateRoot(suite.genesis, clusterQC, suite.epochCounter)
	suite.Require().NoError(err)
	clusterState, err := clusterkv.Bootstrap(suite.db, suite.lockManager, clusterStateRoot)
	suite.Require().NoError(err)

	suite.state, err = clusterkv.NewMutableState(clusterState, suite.lockManager, tracer, suite.headers, suite.payloads)
	suite.Require().NoError(err)

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
		log,
		tracer,
		consumer,
		state,
		all.Index,
		all.Payloads,
		util.MockBlockTimer(),
	)
	require.NoError(suite.T(), err)

	// add some transactions to transaction pool
	for i := 0; i < 3; i++ {
		transaction := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
			tx.ReferenceBlockID = root.ID()
			tx.ProposalKey.SequenceNumber = uint64(i)
			tx.GasLimit = uint64(9999)
		})
		added := suite.pool.Add(transaction.ID(), &transaction)
		suite.Assert().True(added)
	}

	suite.builder, _ = builder.NewBuilder(
		suite.db,
		tracer,
		suite.lockManager,
		metrics,
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
	)
}

// runs after each test finishes
func (suite *BuilderSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().NoError(err)
	err = os.RemoveAll(suite.dbdir)
	suite.Assert().NoError(err)
}

func (suite *BuilderSuite) InsertBlock(block *model.Block) {
	lctx := suite.lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock)
	suite.Assert().NoError(err)
	err = suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return procedure.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
	})
	suite.Assert().NoError(err)
}

func (suite *BuilderSuite) FinalizeBlock(block model.Block) {
	lctx := suite.lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock)
	suite.Assert().NoError(err)

	err = suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		var refBlock flow.Header
		err := operation.RetrieveHeader(rw.GlobalReader(), block.Payload.ReferenceBlockID, &refBlock)
		if err != nil {
			return err
		}
		err = procedure.FinalizeClusterBlock(lctx, rw, block.ID())
		if err != nil {
			return err
		}
		return operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), refBlock.Height, block.ID())
	})
	suite.Assert().NoError(err)
}

// Payload returns a payload containing the given transactions, with a valid
// reference block ID.
func (suite *BuilderSuite) Payload(transactions ...*flow.TransactionBody) model.Payload {
	final, err := suite.protoState.Final().Head()
	suite.Require().NoError(err)

	payload, err := model.NewPayload(
		model.UntrustedPayload{
			ReferenceBlockID: final.ID(),
			Collection:       flow.Collection{Transactions: transactions},
		},
	)
	suite.Require().NoError(err)

	return *payload
}

// ProtoStateRoot returns the root block of the protocol state.
func (suite *BuilderSuite) ProtoStateRoot() *flow.Header {
	return suite.protoState.Params().FinalizedRoot()
}

// ClearPool removes all items from the pool
func (suite *BuilderSuite) ClearPool() {
	suite.pool.Clear()
}

// FillPool adds n transactions to the pool, using the given generator function.
func (suite *BuilderSuite) FillPool(n int, create func() *flow.TransactionBody) {
	for i := 0; i < n; i++ {
		tx := create()
		suite.pool.Add(tx.ID(), tx)
	}
}

func TestBuilder(t *testing.T) {
	suite.Run(t, new(BuilderSuite))
}

func (suite *BuilderSuite) TestBuildOn_NonExistentParent() {
	// use a non-existent parent ID
	parentID := unittest.IdentifierFixture()

	_, err := suite.builder.BuildOn(parentID, setter, signer)
	suite.Assert().Error(err)
}

func (suite *BuilderSuite) TestBuildOn_Success() {
	var expectedHeight uint64 = 42

	proposal, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// setter should have been run
	suite.Assert().Equal(expectedHeight, proposal.Header.Height)

	// should be able to retrieve built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), proposal.Header.ID(), &built)
	suite.Assert().NoError(err)
	builtCollection := built.Payload.Collection

	// should reference a valid reference block
	// (since genesis is the only block, it's the only valid reference)
	mainGenesis, err := suite.protoState.AtHeight(0).Head()
	suite.Assert().NoError(err)
	suite.Assert().Equal(mainGenesis.ID(), built.Payload.ReferenceBlockID)

	// payload should include only items from mempool
	mempoolTransactions := suite.pool.Values()
	suite.Assert().Len(builtCollection.Transactions, 3)
	suite.Assert().True(collectionContains(builtCollection, flow.GetIDs(mempoolTransactions)...))
}

// TestBuildOn_SetterErrorPassthrough validates that errors from the setter function are passed through to the caller.
func (suite *BuilderSuite) TestBuildOn_SetterErrorPassthrough() {
	sentinel := errors.New("sentinel")
	setter := func(h *flow.HeaderBodyBuilder) error {
		return sentinel
	}
	_, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Assert().ErrorIs(err, sentinel)
}

// TestBuildOn_SignerErrorPassthrough validates that errors from the sign function are passed through to the caller.
func (suite *BuilderSuite) TestBuildOn_SignerErrorPassthrough() {
	suite.T().Run("unexpected Exception", func(t *testing.T) {
		exception := errors.New("exception")
		sign := func(h *flow.Header) ([]byte, error) {
			return nil, exception
		}
		_, err := suite.builder.BuildOn(suite.genesis.ID(), setter, sign)
		suite.Assert().ErrorIs(err, exception)
	})
	suite.T().Run("NoVoteError", func(t *testing.T) {
		// the EventHandler relies on this sentinel in particular to be passed through
		sentinel := hotstuffmodel.NewNoVoteErrorf("not voting")
		sign := func(h *flow.Header) ([]byte, error) {
			return nil, sentinel
		}
		_, err := suite.builder.BuildOn(suite.genesis.ID(), setter, sign)
		suite.Assert().ErrorIs(err, sentinel)
	})
}

// when there are transactions with an unknown reference block in the pool, we should not include them in collections
func (suite *BuilderSuite) TestBuildOn_WithUnknownReferenceBlock() {

	// before modifying the mempool, note the valid transactions already in the pool
	validMempoolTransactions := suite.pool.Values()

	// add a transaction unknown reference block to the pool
	unknownReferenceTx := unittest.TransactionBodyFixture()
	unknownReferenceTx.ReferenceBlockID = unittest.IdentifierFixture()
	suite.pool.Add(unknownReferenceTx.ID(), &unknownReferenceTx)

	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// should be able to retrieve built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Assert().NoError(err)
	builtCollection := built.Payload.Collection

	suite.Assert().Len(builtCollection.Transactions, 3)
	// payload should include only the transactions with a valid reference block
	suite.Assert().True(collectionContains(builtCollection, flow.GetIDs(validMempoolTransactions)...))
	// should not contain the unknown-reference transaction
	suite.Assert().False(collectionContains(builtCollection, unknownReferenceTx.ID()))
}

// when there are transactions with a known but unfinalized reference block in the pool, we should not include them in collections
func (suite *BuilderSuite) TestBuildOn_WithUnfinalizedReferenceBlock() {

	// before modifying the mempool, note the valid transactions already in the pool
	validMempoolTransactions := suite.pool.Values()

	// add an unfinalized block to the protocol state
	genesis, err := suite.protoState.Final().Head()
	suite.Require().NoError(err)
	protocolState, err := suite.protoState.Final().ProtocolState()
	suite.Require().NoError(err)
	protocolStateID := protocolState.ID()

	unfinalizedReferenceBlock := unittest.BlockWithParentAndPayload(
		genesis,
		unittest.PayloadFixture(unittest.WithProtocolStateID(protocolStateID)),
	)
	err = suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(unfinalizedReferenceBlock))
	suite.Require().NoError(err)

	// add a transaction with unfinalized reference block to the pool
	unfinalizedReferenceTx := unittest.TransactionBodyFixture()
	unfinalizedReferenceTx.ReferenceBlockID = unfinalizedReferenceBlock.ID()
	suite.pool.Add(unfinalizedReferenceTx.ID(), &unfinalizedReferenceTx)

	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// should be able to retrieve built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Assert().NoError(err)
	builtCollection := built.Payload.Collection

	suite.Assert().Len(builtCollection.Transactions, 3)
	// payload should include only the transactions with a valid reference block
	suite.Assert().True(collectionContains(builtCollection, flow.GetIDs(validMempoolTransactions)...))
	// should not contain the unfinalized-reference transaction
	suite.Assert().False(collectionContains(builtCollection, unfinalizedReferenceTx.ID()))
}

// when there are transactions with an orphaned reference block in the pool, we should not include them in collections
func (suite *BuilderSuite) TestBuildOn_WithOrphanedReferenceBlock() {

	// before modifying the mempool, note the valid transactions already in the pool
	validMempoolTransactions := suite.pool.Values()

	// add an orphaned block to the protocol state
	genesis, err := suite.protoState.Final().Head()
	suite.Require().NoError(err)
	protocolState, err := suite.protoState.Final().ProtocolState()
	suite.Require().NoError(err)
	protocolStateID := protocolState.ID()

	// create a block extending genesis which will be orphaned
	orphan := unittest.BlockWithParentAndPayload(
		genesis,
		unittest.PayloadFixture(unittest.WithProtocolStateID(protocolStateID)),
	)
	err = suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(orphan))
	suite.Require().NoError(err)
	// create and finalize a block on top of genesis, orphaning `orphan`
	block1 := unittest.BlockWithParentAndPayload(
		genesis,
		unittest.PayloadFixture(unittest.WithProtocolStateID(protocolStateID)),
	)
	err = suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block1))
	suite.Require().NoError(err)
	err = suite.protoState.Finalize(context.Background(), block1.ID())
	suite.Require().NoError(err)

	// add a transaction with orphaned reference block to the pool
	orphanedReferenceTx := unittest.TransactionBodyFixture()
	orphanedReferenceTx.ReferenceBlockID = orphan.ID()
	suite.pool.Add(orphanedReferenceTx.ID(), &orphanedReferenceTx)

	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// should be able to retrieve built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Assert().NoError(err)
	builtCollection := built.Payload.Collection

	suite.Assert().Len(builtCollection.Transactions, 3)
	// payload should include only the transactions with a valid reference block
	suite.Assert().True(collectionContains(builtCollection, flow.GetIDs(validMempoolTransactions)...))
	// should not contain the unknown-reference transaction
	suite.Assert().False(collectionContains(builtCollection, orphanedReferenceTx.ID()))
	// the transaction with orphaned reference should be removed from the mempool
	suite.Assert().False(suite.pool.Has(orphanedReferenceTx.ID()))
}

func (suite *BuilderSuite) TestBuildOn_WithForks() {
	t := suite.T()

	mempoolTransactions := suite.pool.Values()
	tx1 := mempoolTransactions[0] // in fork 1
	tx2 := mempoolTransactions[1] // in fork 2
	tx3 := mempoolTransactions[2] // in no block

	// build first fork on top of genesis
	block1 := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(suite.genesis),
		unittest.ClusterBlock.WithPayload(suite.Payload(tx1)),
	)
	// insert block on fork 1
	suite.InsertBlock(block1)

	// build second fork on top of genesis
	block2 := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(suite.genesis),
		unittest.ClusterBlock.WithPayload(suite.Payload(tx2)),
	)
	// insert block on fork 2
	suite.InsertBlock(block2)

	// build on top of fork 1
	header, err := suite.builder.BuildOn(block1.ID(), setter, signer)
	require.NoError(t, err)

	// should be able to retrieve built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	assert.NoError(t, err)
	builtCollection := built.Payload.Collection

	// payload should include ONLY tx2 and tx3
	assert.Len(t, builtCollection.Transactions, 2)
	assert.True(t, collectionContains(builtCollection, tx2.ID(), tx3.ID()))
	assert.False(t, collectionContains(builtCollection, tx1.ID()))
}

func (suite *BuilderSuite) TestBuildOn_ConflictingFinalizedBlock() {
	t := suite.T()

	mempoolTransactions := suite.pool.Values()
	tx1 := mempoolTransactions[0] // in a finalized block
	tx2 := mempoolTransactions[1] // in an un-finalized block
	tx3 := mempoolTransactions[2] // in no blocks

	t.Logf("tx1: %s\ntx2: %s\ntx3: %s", tx1.ID(), tx2.ID(), tx3.ID())

	// build a block containing tx1 on genesis
	finalizedPayload := suite.Payload(tx1)
	finalizedBlock := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(suite.genesis),
		unittest.ClusterBlock.WithPayload(finalizedPayload),
	)
	suite.InsertBlock(finalizedBlock)
	t.Logf("finalized: height=%d id=%s txs=%s parent_id=%s\t\n", finalizedBlock.Height, finalizedBlock.ID(), finalizedPayload.Collection.Light(), finalizedBlock.ParentID)

	// build a block containing tx2 on the first block
	unFinalizedPayload := suite.Payload(tx2)
	unFinalizedBlock := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(finalizedBlock),
		unittest.ClusterBlock.WithPayload(unFinalizedPayload),
	)
	suite.InsertBlock(unFinalizedBlock)
	t.Logf("finalized: height=%d id=%s txs=%s parent_id=%s\t\n", unFinalizedBlock.Height, unFinalizedBlock.ID(), unFinalizedPayload.Collection.Light(), unFinalizedBlock.ParentID)

	// finalize first block
	suite.FinalizeBlock(*finalizedBlock)

	// build on the un-finalized block
	header, err := suite.builder.BuildOn(unFinalizedBlock.ID(), setter, signer)
	require.NoError(t, err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	assert.NoError(t, err)
	builtCollection := built.Payload.Collection

	// payload should only contain tx3
	assert.Len(t, builtCollection.Light().Transactions, 1)
	assert.True(t, collectionContains(builtCollection, tx3.ID()))
	assert.False(t, collectionContains(builtCollection, tx1.ID(), tx2.ID()))

	// tx1 should be removed from mempool, as it is in a finalized block
	assert.False(t, suite.pool.Has(tx1.ID()))
	// tx2 should NOT be removed from mempool, as it is in an un-finalized block
	assert.True(t, suite.pool.Has(tx2.ID()))
}

func (suite *BuilderSuite) TestBuildOn_ConflictingInvalidatedForks() {
	t := suite.T()

	mempoolTransactions := suite.pool.Values()
	tx1 := mempoolTransactions[0] // in a finalized block
	tx2 := mempoolTransactions[1] // in an invalidated block
	tx3 := mempoolTransactions[2] // in no blocks

	t.Logf("tx1: %s\ntx2: %s\ntx3: %s", tx1.ID(), tx2.ID(), tx3.ID())

	// build a block containing tx1 on genesis - will be finalized
	finalizedBlock := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(suite.genesis),
		unittest.ClusterBlock.WithPayload(suite.Payload(tx1)),
	)
	suite.InsertBlock(finalizedBlock)
	t.Logf("finalized: id=%s\tparent_id=%s\theight=%d\n", finalizedBlock.ID(), finalizedBlock.ParentID, finalizedBlock.Height)

	// build a block containing tx2 ALSO on genesis - will be invalidated
	invalidatedBlock := unittest.ClusterBlockFixture(
		unittest.ClusterBlock.WithParent(suite.genesis),
		unittest.ClusterBlock.WithPayload(suite.Payload(tx2)),
	)
	suite.InsertBlock(invalidatedBlock)
	t.Logf("invalidated: id=%s\tparent_id=%s\theight=%d\n", invalidatedBlock.ID(), invalidatedBlock.ParentID, invalidatedBlock.Height)

	// finalize first block - this indirectly invalidates the second block
	suite.FinalizeBlock(*finalizedBlock)

	// build on the finalized block
	header, err := suite.builder.BuildOn(finalizedBlock.ID(), setter, signer)
	require.NoError(t, err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	assert.NoError(t, err)
	builtCollection := built.Payload.Collection

	// tx2 and tx3 should be in the built collection
	assert.Len(t, builtCollection.Light().Transactions, 2)
	assert.True(t, collectionContains(builtCollection, tx2.ID(), tx3.ID()))
	assert.False(t, collectionContains(builtCollection, tx1.ID()))
}

func (suite *BuilderSuite) TestBuildOn_LargeHistory() {
	t := suite.T()

	// use a mempool with 2000 transactions, one per block
	suite.pool = herocache.NewTransactions(2000, unittest.Logger(), metrics.NewNoopCollector())
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10000),
	)

	// get a valid reference block ID
	final, err := suite.protoState.Final().Head()
	require.NoError(t, err)
	refID := final.ID()

	// keep track of the head of the chain
	head := suite.genesis

	// keep track of invalidated transaction IDs
	var invalidatedTxIds []flow.Identifier

	// create a large history of blocks with invalidated forks every 3 blocks on
	// average - build until the height exceeds transaction expiry
	for i := 0; ; i++ {

		// create a transaction
		tx := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
			tx.ReferenceBlockID = refID
			tx.ProposalKey.SequenceNumber = uint64(i)
		})
		added := suite.pool.Add(tx.ID(), &tx)
		assert.True(t, added)

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
			invalidatedTxIds = append(invalidatedTxIds, tx.ID())
		}

		// create a block containing the transaction
		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(head),
			unittest.ClusterBlock.WithPayload(suite.Payload(&tx)),
		)
		suite.InsertBlock(block)

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

	t.Log("conflicting: ", len(invalidatedTxIds))

	// build on the head block
	header, err := suite.builder.BuildOn(head.ID(), setter, signer)
	require.NoError(t, err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	require.NoError(t, err)
	builtCollection := built.Payload.Collection

	// payload should only contain transactions from invalidated blocks
	assert.Len(t, builtCollection.Transactions, len(invalidatedTxIds), "expected len=%d, got len=%d", len(invalidatedTxIds), len(builtCollection.Transactions))
	assert.True(t, collectionContains(builtCollection, invalidatedTxIds...))
}

func (suite *BuilderSuite) TestBuildOn_MaxCollectionSize() {
	// set the max collection size to 1
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(1),
	)

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Require().NoError(err)
	builtCollection := built.Payload.Collection

	// should be only 1 transaction in the collection
	suite.Assert().Equal(builtCollection.Len(), 1)
}

func (suite *BuilderSuite) TestBuildOn_MaxCollectionByteSize() {
	// set the max collection byte size to 400 (each tx is about 150 bytes)
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionByteSize(400),
	)

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Require().NoError(err)
	builtCollection := built.Payload.Collection

	// should be only 2 transactions in the collection, since each tx is ~273 bytes and the limit is 600 bytes
	suite.Assert().Equal(builtCollection.Len(), 2)
}

func (suite *BuilderSuite) TestBuildOn_MaxCollectionTotalGas() {
	// set the max gas to 20,000
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionTotalGas(20000),
	)

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Require().NoError(err)
	builtCollection := built.Payload.Collection

	// should be only 2 transactions in collection, since each transaction has gas limit of 9,999 and collection limit is set to 20,000
	suite.Assert().Equal(builtCollection.Len(), 2)
}

func (suite *BuilderSuite) TestBuildOn_ExpiredTransaction() {

	// create enough main-chain blocks that an expired transaction is possible
	genesis, err := suite.protoState.Final().Head()
	suite.Require().NoError(err)
	protocolState, err := suite.protoState.Final().ProtocolState()
	suite.Require().NoError(err)
	protocolStateID := protocolState.ID()

	head := genesis
	for i := 0; i < flow.DefaultTransactionExpiry+1; i++ {
		block := unittest.BlockWithParentAndPayload(
			head,
			unittest.PayloadFixture(unittest.WithProtocolStateID(protocolStateID)),
		)
		err = suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
		suite.Require().NoError(err)
		err = suite.protoState.Finalize(context.Background(), block.ID())
		suite.Require().NoError(err)
		head = block.ToHeader()
	}

	config := updatable_configs.DefaultBySealingLagRateLimiterConfigs()
	require.NoError(suite.T(), config.SetMaxSealingLag(flow.DefaultTransactionExpiry*2))
	require.NoError(suite.T(), config.SetHalvingInterval(flow.DefaultTransactionExpiry*2))

	// reset the pool and builder
	suite.pool = herocache.NewTransactions(10, unittest.Logger(), metrics.NewNoopCollector())
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		config,
	)

	// insert a transaction referring genesis (now expired)
	tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
		tx.ReferenceBlockID = genesis.ID()
		tx.ProposalKey.SequenceNumber = 0
	})
	added := suite.pool.Add(tx1.ID(), &tx1)
	suite.Assert().True(added)

	// insert a transaction referencing the head (valid)
	tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
		tx.ReferenceBlockID = head.ID()
		tx.ProposalKey.SequenceNumber = 1
	})
	added = suite.pool.Add(tx2.ID(), &tx2)
	suite.Assert().True(added)

	suite.T().Log("tx1: ", tx1.ID())
	suite.T().Log("tx2: ", tx2.ID())

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	// retrieve the built block from storage
	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Require().NoError(err)
	builtCollection := built.Payload.Collection

	// the block should only contain the un-expired transaction
	suite.Assert().False(collectionContains(builtCollection, tx1.ID()))
	suite.Assert().True(collectionContains(builtCollection, tx2.ID()))
	// the expired transaction should have been removed from the mempool
	suite.Assert().False(suite.pool.Has(tx1.ID()))
}

func (suite *BuilderSuite) TestBuildOn_EmptyMempool() {

	// start with an empty mempool
	suite.pool = herocache.NewTransactions(1000, unittest.Logger(), metrics.NewNoopCollector())
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
	)

	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter, signer)
	suite.Require().NoError(err)

	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Require().NoError(err)

	// should reference a valid reference block
	// (since genesis is the only block, it's the only valid reference)
	mainGenesis, err := suite.protoState.AtHeight(0).Head()
	suite.Assert().NoError(err)
	suite.Assert().Equal(mainGenesis.ID(), built.Payload.ReferenceBlockID)

	// the payload should be empty
	suite.Assert().Equal(0, built.Payload.Collection.Len())
}

// With rate limiting turned off, we should fill collections as fast as we can
// regardless of how many transactions with the same payer we include.
func (suite *BuilderSuite) TestBuildOn_NoRateLimiting() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with no rate limit and max 10 tx/collection
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10),
		builder.WithMaxPayerTransactionRate(0),
	)

	// fill the pool with 100 transactions from the same payer
	payer := unittest.RandomAddressFixture()
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = payer
		return &tx
	}
	suite.FillPool(100, create)

	// since we have no rate limiting we should fill all collections and in 10 blocks
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithHeight(1).
			WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())

		return nil
	}

	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 10)
	}
}

// With rate limiting turned on, we should be able to fill transactions as fast
// as possible so long as per-payer limits are not reached. This test generates
// transactions such that the number of transactions with a given proposer exceeds
// the rate limit -- since it's the proposer not the payer, it shouldn't limit
// our collections.
func (suite *BuilderSuite) TestBuildOn_RateLimitNonPayer() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with 5 tx/payer and max 10 tx/collection
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10),
		builder.WithMaxPayerTransactionRate(5),
	)

	// fill the pool with 100 transactions with the same proposer
	// since it's not the same payer, rate limit does not apply
	proposer := unittest.RandomAddressFixture()
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = unittest.RandomAddressFixture()
		tx.ProposalKey = flow.ProposalKey{
			Address:        proposer,
			KeyIndex:       rand.Uint32(),
			SequenceNumber: rand.Uint64(),
		}
		return &tx
	}
	suite.FillPool(100, create)

	// since rate limiting does not apply to non-payer keys, we should fill all collections in 10 blocks
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())

		return nil
	}
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 10)
	}
}

// When configured with a rate limit of k>1, we should be able to include up to
// k transactions with a given payer per collection
func (suite *BuilderSuite) TestBuildOn_HighRateLimit() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with 5 tx/payer and max 10 tx/collection
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10),
		builder.WithMaxPayerTransactionRate(5),
	)

	// fill the pool with 50 transactions from the same payer
	payer := unittest.RandomAddressFixture()
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = payer
		return &tx
	}
	suite.FillPool(50, create)

	// rate-limiting should be applied, resulting in half-full collections (5/10)
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())

		return nil
	}
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// each collection should be half-full with 5 transactions
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 5)
	}
}

// TestBuildOn_MaxCollectionSizeRateLimiting tests that when sealing lag is larger than maximum allowed value,
// then the builder will apply rate-limiting to the collection size, resulting in minimal collection size.
func (suite *BuilderSuite) TestBuildOn_MaxCollectionSizeRateLimiting() {

	// start with an empty mempool
	suite.ClearPool()

	cfg := updatable_configs.DefaultBySealingLagRateLimiterConfigs()
	suite.Require().NoError(cfg.SetMinSealingLag(50)) // set min sealing lag to 50 blocks so we can hit rate limiting
	suite.Require().NoError(cfg.SetMaxSealingLag(50)) // set max sealing lag to 50 blocks so we can hit rate limiting
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		cfg,
		builder.WithMaxCollectionSize(100),
	)

	// fill the pool with 50 transactions from the same payer
	payer := unittest.RandomAddressFixture()
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = payer
		return &tx
	}
	suite.FillPool(50, create)

	// add an unfinalized block to the protocol state
	genesis, err := suite.protoState.Final().Head()
	suite.Require().NoError(err)
	protocolState, err := suite.protoState.Final().ProtocolState()
	suite.Require().NoError(err)
	protocolStateID := protocolState.ID()

	head := genesis
	// build a long chain of blocks that were finalized but not sealed
	// this will lead to a big sealing lag.
	for i := 0; i < 100; i++ {
		block := unittest.BlockWithParentAndPayload(head, unittest.PayloadFixture(unittest.WithProtocolStateID(protocolStateID)))
		err = suite.protoState.ExtendCertified(context.Background(), unittest.NewCertifiedBlock(block))
		suite.Require().NoError(err)
		err = suite.protoState.Finalize(context.Background(), block.ID())
		suite.Require().NoError(err)
		head = block.ToHeader()
	}

	rateLimiterCfg := updatable_configs.DefaultBySealingLagRateLimiterConfigs()

	// rate-limiting should be applied, resulting in minimum collection size.
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())
		return nil
	}
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// each collection should be equal to the minimum collection size
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, int(rateLimiterCfg.MinCollectionSize()))
	}
}

// When configured with a rate limit of k<1, we should be able to include 1
// transactions with a given payer every ceil(1/k) collections
func (suite *BuilderSuite) TestBuildOn_LowRateLimit() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with .5 tx/payer and max 10 tx/collection
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10),
		builder.WithMaxPayerTransactionRate(.5),
	)

	// fill the pool with 5 transactions from the same payer
	payer := unittest.RandomAddressFixture()
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = payer
		return &tx
	}
	suite.FillPool(5, create)

	// rate-limiting should be applied, resulting in every ceil(1/k) collections
	// having one transaction and empty collections otherwise
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())

		return nil
	}
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// collections should either be empty or have 1 transaction
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		if i%2 == 0 {
			suite.Assert().Len(built.Payload.Collection.Transactions, 1)
		} else {
			suite.Assert().Len(built.Payload.Collection.Transactions, 0)
		}
	}
}
func (suite *BuilderSuite) TestBuildOn_UnlimitedPayer() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with 5 tx/payer and max 10 tx/collection
	// configure an unlimited payer
	payer := unittest.RandomAddressFixture()
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10),
		builder.WithMaxPayerTransactionRate(5),
		builder.WithUnlimitedPayers(payer),
	)

	// fill the pool with 100 transactions from the same payer
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = payer
		return &tx
	}
	suite.FillPool(100, create)

	// rate-limiting should not be applied, since the payer is marked as unlimited
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())

		return nil
	}
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 10)

	}
}

// TestBuildOn_RateLimitDryRun tests that rate limiting rules aren't enforced
// if dry-run is enabled.
func (suite *BuilderSuite) TestBuildOn_RateLimitDryRun() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with 5 tx/payer and max 10 tx/collection
	// configure an unlimited payer
	payer := unittest.RandomAddressFixture()
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(10),
		builder.WithMaxPayerTransactionRate(5),
		builder.WithRateLimitDryRun(true),
	)

	// fill the pool with 100 transactions from the same payer
	create := func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = payer
		return &tx
	}
	suite.FillPool(100, create)

	// rate-limiting should not be applied, since dry-run setting is enabled
	parentID := suite.genesis.ID()
	setter := func(h *flow.HeaderBodyBuilder) error {
		h.WithChainID(flow.Emulator).
			WithParentID(parentID).
			WithView(1337).
			WithParentView(1336).
			WithParentVoterIndices(unittest.SignerIndicesFixture(4)).
			WithParentVoterSigData(unittest.QCSigDataFixture()).
			WithProposerID(unittest.IdentifierFixture())

		return nil
	}
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, setter, signer)
		suite.Require().NoError(err)
		parentID = header.Header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
		suite.Assert().NoError(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 10)
	}
}

// TestBuildOn_SystemTxAlwaysIncluded tests that transaction made by a priority payer will always be included
// in the collection, even if mempool has more transactions than the maximum collection size.
func (suite *BuilderSuite) TestBuildOn_SystemTxAlwaysIncluded() {
	// start with an empty mempool
	suite.ClearPool()

	// create builder with 5 tx/payer and max 10 tx/collection
	// configure an unlimited payer
	serviceAccountAddress := unittest.AddressFixture() // priority address
	suite.builder, _ = builder.NewBuilder(
		suite.db,
		trace.NewNoopTracer(),
		suite.lockManager,
		metrics.NewNoopCollector(),
		suite.protoState,
		suite.state,
		suite.headers,
		suite.headers,
		suite.payloads,
		suite.pool,
		unittest.Logger(),
		suite.epochCounter,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		builder.WithMaxCollectionSize(2),
		builder.WithPriorityPayers(serviceAccountAddress),
	)

	// fill the pool with 100 transactions
	suite.FillPool(100, func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		return &tx
	})
	suite.FillPool(2, func() *flow.TransactionBody {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.ProtoStateRoot().ID()
		tx.Payer = serviceAccountAddress
		return &tx
	})

	// rate-limiting should not be applied, since the payer is marked as unlimited
	parentID := suite.genesis.ID()
	header, err := suite.builder.BuildOn(parentID, setter, signer)
	suite.Require().NoError(err)

	var built model.Block
	err = procedure.RetrieveClusterBlock(suite.db.Reader(), header.Header.ID(), &built)
	suite.Assert().NoError(err)
	suite.Assert().Len(built.Payload.Collection.Transactions, 2)
	for _, tx := range built.Payload.Collection.Transactions {
		suite.Assert().Equal(serviceAccountAddress, tx.Payer)
	}
}

// helper to check whether a collection contains each of the given transactions.
func collectionContains(collection flow.Collection, txIDs ...flow.Identifier) bool {

	lookup := make(map[flow.Identifier]struct{}, len(txIDs))
	for _, tx := range collection.Transactions {
		lookup[tx.ID()] = struct{}{}
	}

	for _, txID := range txIDs {
		_, exists := lookup[txID]
		if !exists {
			return false
		}
	}

	return true
}

func BenchmarkBuildOn10(b *testing.B)     { benchmarkBuildOn(b, 10) }
func BenchmarkBuildOn100(b *testing.B)    { benchmarkBuildOn(b, 100) }
func BenchmarkBuildOn1000(b *testing.B)   { benchmarkBuildOn(b, 1000) }
func BenchmarkBuildOn10000(b *testing.B)  { benchmarkBuildOn(b, 10000) }
func BenchmarkBuildOn100000(b *testing.B) { benchmarkBuildOn(b, 100000) }

func benchmarkBuildOn(b *testing.B, size int) {
	b.StopTimer()
	b.ResetTimer()

	// re-use the builder suite
	suite := new(BuilderSuite)

	// Copied from SetupTest. We can't use that function because suite.Assert
	// is incompatible with benchmarks.
	// ref: https://github.com/stretchr/testify/issues/811
	{
		var err error

		suite.genesis, err = unittest.ClusterBlock.Genesis()
		require.NoError(suite.T(), err)
		suite.chainID = suite.genesis.ChainID
		suite.lockManager = storage.NewTestingLockManager()

		suite.pool = herocache.NewTransactions(1000, unittest.Logger(), metrics.NewNoopCollector())

		suite.dbdir = unittest.TempDir(b)
		pdb := unittest.PebbleDB(suite.T(), suite.dbdir)
		suite.db = pebbleimpl.ToDB(pdb)
		defer func() {
			err = suite.db.Close()
			assert.NoError(b, err)
			err = os.RemoveAll(suite.dbdir)
			assert.NoError(b, err)
		}()

		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		all := store.InitAll(metrics, suite.db)
		suite.headers = all.Headers
		suite.blocks = all.Blocks
		suite.payloads = store.NewClusterPayloads(metrics, suite.db)

		qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(suite.genesis.ID()))
		stateRoot, err := clusterkv.NewStateRoot(suite.genesis, qc, suite.epochCounter)

		state, err := clusterkv.Bootstrap(suite.db, suite.lockManager, stateRoot)
		assert.NoError(b, err)

		suite.state, err = clusterkv.NewMutableState(state, suite.lockManager, tracer, suite.headers, suite.payloads)
		assert.NoError(b, err)

		// add some transactions to transaction pool
		for i := 0; i < 3; i++ {
			tx := unittest.TransactionBodyFixture()
			added := suite.pool.Add(tx.ID(), &tx)
			assert.True(b, added)
		}

		// create the builder
		suite.builder, _ = builder.NewBuilder(
			suite.db,
			tracer,
			suite.lockManager,
			metrics,
			suite.protoState,
			suite.state,
			suite.headers,
			suite.headers,
			suite.payloads,
			suite.pool,
			unittest.Logger(),
			suite.epochCounter,
			updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
		)
	}

	// create a block history to test performance against
	final := suite.genesis
	for i := 0; i < size; i++ {
		block := unittest.ClusterBlockFixture(
			unittest.ClusterBlock.WithParent(final),
		)
		lctx := suite.lockManager.NewContext()
		require.NoError(b, lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
		err := suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return procedure.InsertClusterBlock(lctx, rw, unittest.ClusterProposalFromBlock(block))
		})
		require.NoError(b, err)
		lctx.Release()

		// finalize the block 80% of the time, resulting in a fork-rate of 20%
		if rand.Intn(100) < 80 {
			lctx := suite.lockManager.NewContext()
			defer lctx.Release()
			require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock))
			err = suite.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return procedure.FinalizeClusterBlock(lctx, rw, block.ID())
			})
			require.NoError(b, err)
			final = block
		}
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err := suite.builder.BuildOn(final.ID(), setter, signer)
		assert.NoError(b, err)
	}
}
