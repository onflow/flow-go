package collection_test

import (
	"context"
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
	builder "github.com/onflow/flow-go/module/builder/collection"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/cluster"
	clusterkv "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/util"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/procedure"
	sutil "github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

var noopSetter = func(*flow.Header) error { return nil }

type BuilderSuite struct {
	suite.Suite
	db    *badger.DB
	dbdir string

	genesis *model.Block
	chainID flow.ChainID

	headers  *storage.Headers
	payloads *storage.ClusterPayloads
	blocks   *storage.Blocks

	state cluster.MutableState

	// protocol state for reference blocks for transactions
	protoState protocol.MutableState

	pool    *stdmap.Transactions
	builder *builder.Builder
}

// runs before each test runs
func (suite *BuilderSuite) SetupTest() {
	var err error

	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	suite.genesis = model.Genesis()
	suite.chainID = suite.genesis.Header.ChainID

	suite.pool = stdmap.NewTransactions(1000)

	suite.dbdir = unittest.TempDir(suite.T())
	suite.db = unittest.BadgerDB(suite.T(), suite.dbdir)

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	headers, _, seals, index, conPayloads, blocks, setups, commits, statuses, results := sutil.StorageLayer(suite.T(), suite.db)
	consumer := events.NewNoop()
	suite.headers = headers
	suite.blocks = blocks
	suite.payloads = storage.NewClusterPayloads(metrics, suite.db)

	clusterStateRoot, err := clusterkv.NewStateRoot(suite.genesis)
	suite.Require().Nil(err)
	clusterState, err := clusterkv.Bootstrap(suite.db, clusterStateRoot)
	suite.Require().Nil(err)

	suite.state, err = clusterkv.NewMutableState(clusterState, tracer, suite.headers, suite.payloads)
	suite.Require().Nil(err)

	// just bootstrap with a genesis block, we'll use this as reference
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	root, result, seal := unittest.BootstrapFixture(participants)
	qc := unittest.QuorumCertificateFixture(unittest.QCWithBlockID(root.ID()))
	// ensure we don't enter a new epoch for tests that build many blocks
	result.ServiceEvents[0].Event.(*flow.EpochSetup).FinalView = root.Header.View + 100000
	seal.ResultID = result.ID()

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	require.NoError(suite.T(), err)

	state, err := pbadger.Bootstrap(metrics, suite.db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
	require.NoError(suite.T(), err)

	suite.protoState, err = pbadger.NewFollowerState(state, index, conPayloads, tracer, consumer, util.MockBlockTimer())
	require.NoError(suite.T(), err)

	// add some transactions to transaction pool
	for i := 0; i < 3; i++ {
		transaction := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
			tx.ReferenceBlockID = root.ID()
			tx.ProposalKey.SequenceNumber = uint64(i)
			tx.GasLimit = uint64(9999)
		})
		added := suite.pool.Add(&transaction)
		suite.Assert().True(added)
	}

	suite.builder = builder.NewBuilder(suite.db, tracer, suite.headers, suite.headers, suite.payloads, suite.pool)
}

// runs after each test finishes
func (suite *BuilderSuite) TearDownTest() {
	err := suite.db.Close()
	suite.Assert().Nil(err)
	err = os.RemoveAll(suite.dbdir)
	suite.Assert().Nil(err)
}

func (suite *BuilderSuite) InsertBlock(block model.Block) {
	err := suite.db.Update(procedure.InsertClusterBlock(&block))
	suite.Assert().Nil(err)
}

func (suite *BuilderSuite) FinalizeBlock(blockID flow.Identifier) {
	err := suite.db.Update(procedure.FinalizeClusterBlock(blockID))
	suite.Assert().Nil(err)
}

// Payload returns a payload containing the given transactions, with a valid
// reference block ID.
func (suite *BuilderSuite) Payload(transactions ...*flow.TransactionBody) model.Payload {
	final, err := suite.protoState.Final().Head()
	suite.Require().Nil(err)
	return model.PayloadFromTransactions(final.ID(), transactions...)
}

// ProtoStateRoot returns the root block of the protocol state.
func (suite *BuilderSuite) ProtoStateRoot() *flow.Header {
	root, err := suite.protoState.Params().Root()
	suite.Require().Nil(err)
	return root
}

// ClearPool removes all items from the pool
func (suite *BuilderSuite) ClearPool() {
	// TODO use Clear()
	for _, tx := range suite.pool.All() {
		suite.pool.Rem(tx.ID())
	}
}

// FillPool adds n transactions to the pool, using the given generator function.
func (suite *BuilderSuite) FillPool(n int, create func() *flow.TransactionBody) {
	for i := 0; i < n; i++ {
		tx := create()
		suite.pool.Add(tx)
	}
}

func TestBuilder(t *testing.T) {
	suite.Run(t, new(BuilderSuite))
}

func (suite *BuilderSuite) TestBuildOn_NonExistentParent() {
	// use a non-existent parent ID
	parentID := unittest.IdentifierFixture()

	_, err := suite.builder.BuildOn(parentID, noopSetter)
	suite.Assert().Error(err)
}

func (suite *BuilderSuite) TestBuildOn_Success() {

	var expectedHeight uint64 = 42
	setter := func(h *flow.Header) error {
		h.Height = expectedHeight
		return nil
	}

	header, err := suite.builder.BuildOn(suite.genesis.ID(), setter)
	suite.Require().Nil(err)

	// setter should have been run
	suite.Assert().Equal(expectedHeight, header.Height)

	// should be able to retrieve built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Assert().Nil(err)
	builtCollection := built.Payload.Collection

	// should reference a valid reference block
	// (since genesis is the only block, it's the only valid reference)
	mainGenesis, err := suite.protoState.AtHeight(0).Head()
	suite.Assert().Nil(err)
	suite.Assert().Equal(mainGenesis.ID(), built.Payload.ReferenceBlockID)

	// payload should include only items from mempool
	mempoolTransactions := suite.pool.All()
	suite.Assert().Len(builtCollection.Transactions, 3)
	suite.Assert().True(collectionContains(builtCollection, flow.GetIDs(mempoolTransactions)...))
}

// when there are transactions with an unknown reference block in the pool,
// we should not include them in collections
func (suite *BuilderSuite) TestBuildOn_WithUnknownReferenceBlock() {

	// before modifying the mempool, note the valid transactions already in the pool
	validMempoolTransactions := suite.pool.All()

	// add a transaction unknown reference block to the pool
	unknownReferenceTx := unittest.TransactionBodyFixture()
	unknownReferenceTx.ReferenceBlockID = unittest.IdentifierFixture()
	suite.pool.Add(&unknownReferenceTx)

	header, err := suite.builder.BuildOn(suite.genesis.ID(), noopSetter)
	suite.Require().Nil(err)

	// should be able to retrieve built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Assert().Nil(err)
	builtCollection := built.Payload.Collection

	suite.Assert().Len(builtCollection.Transactions, 3)
	// payload should include only the transactions with a valid reference block
	suite.Assert().True(collectionContains(builtCollection, flow.GetIDs(validMempoolTransactions)...))
	// should not contain the the unknown-reference transaction
	suite.Assert().False(collectionContains(builtCollection, unknownReferenceTx.ID()))
}

func (suite *BuilderSuite) TestBuildOn_WithForks() {
	t := suite.T()

	mempoolTransactions := suite.pool.All()
	tx1 := mempoolTransactions[0] // in fork 1
	tx2 := mempoolTransactions[1] // in fork 2
	tx3 := mempoolTransactions[2] // in no block

	// build first fork on top of genesis
	payload1 := suite.Payload(tx1)
	block1 := unittest.ClusterBlockWithParent(suite.genesis)
	block1.SetPayload(payload1)

	// insert block on fork 1
	suite.InsertBlock(block1)

	// build second fork on top of genesis
	payload2 := suite.Payload(tx2)
	block2 := unittest.ClusterBlockWithParent(suite.genesis)
	block2.SetPayload(payload2)

	// insert block on fork 2
	suite.InsertBlock(block2)

	// build on top of fork 1
	header, err := suite.builder.BuildOn(block1.ID(), noopSetter)
	require.Nil(t, err)

	// should be able to retrieve built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	assert.Nil(t, err)
	builtCollection := built.Payload.Collection

	// payload should include ONLY tx2 and tx3
	assert.Len(t, builtCollection.Transactions, 2)
	assert.True(t, collectionContains(builtCollection, tx2.ID(), tx3.ID()))
	assert.False(t, collectionContains(builtCollection, tx1.ID()))
}

func (suite *BuilderSuite) TestBuildOn_ConflictingFinalizedBlock() {
	t := suite.T()

	mempoolTransactions := suite.pool.All()
	tx1 := mempoolTransactions[0] // in a finalized block
	tx2 := mempoolTransactions[1] // in an un-finalized block
	tx3 := mempoolTransactions[2] // in no blocks

	t.Logf("tx1: %s\ntx2: %s\ntx3: %s", tx1.ID(), tx2.ID(), tx3.ID())

	// build a block containing tx1 on genesis
	finalizedPayload := suite.Payload(tx1)
	finalizedBlock := unittest.ClusterBlockWithParent(suite.genesis)
	finalizedBlock.SetPayload(finalizedPayload)
	suite.InsertBlock(finalizedBlock)
	t.Logf("finalized: id=%s\tparent_id=%s\theight=%d\n", finalizedBlock.ID(), finalizedBlock.Header.ParentID, finalizedBlock.Header.Height)

	// build a block containing tx2 on the first block
	unFinalizedPayload := suite.Payload(tx2)
	unFinalizedBlock := unittest.ClusterBlockWithParent(&finalizedBlock)
	unFinalizedBlock.SetPayload(unFinalizedPayload)
	suite.InsertBlock(unFinalizedBlock)
	t.Logf("unfinalized: id=%s\tparent_id=%s\theight=%d\n", unFinalizedBlock.ID(), unFinalizedBlock.Header.ParentID, unFinalizedBlock.Header.Height)

	// finalize first block
	err := suite.db.Update(procedure.FinalizeClusterBlock(finalizedBlock.ID()))
	assert.Nil(t, err)

	// build on the un-finalized block
	header, err := suite.builder.BuildOn(unFinalizedBlock.ID(), noopSetter)
	require.Nil(t, err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	assert.Nil(t, err)
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

	mempoolTransactions := suite.pool.All()
	tx1 := mempoolTransactions[0] // in a finalized block
	tx2 := mempoolTransactions[1] // in an invalidated block
	tx3 := mempoolTransactions[2] // in no blocks

	t.Logf("tx1: %s\ntx2: %s\ntx3: %s", tx1.ID(), tx2.ID(), tx3.ID())

	// build a block containing tx1 on genesis - will be finalized
	finalizedPayload := suite.Payload(tx1)
	finalizedBlock := unittest.ClusterBlockWithParent(suite.genesis)
	finalizedBlock.SetPayload(finalizedPayload)

	suite.InsertBlock(finalizedBlock)
	t.Logf("finalized: id=%s\tparent_id=%s\theight=%d\n", finalizedBlock.ID(), finalizedBlock.Header.ParentID, finalizedBlock.Header.Height)

	// build a block containing tx2 ALSO on genesis - will be invalidated
	invalidatedPayload := suite.Payload(tx2)
	invalidatedBlock := unittest.ClusterBlockWithParent(suite.genesis)
	invalidatedBlock.SetPayload(invalidatedPayload)
	suite.InsertBlock(invalidatedBlock)
	t.Logf("invalidated: id=%s\tparent_id=%s\theight=%d\n", invalidatedBlock.ID(), invalidatedBlock.Header.ParentID, invalidatedBlock.Header.Height)

	// finalize first block - this indirectly invalidates the second block
	err := suite.db.Update(procedure.FinalizeClusterBlock(finalizedBlock.ID()))
	assert.Nil(t, err)

	// build on the finalized block
	header, err := suite.builder.BuildOn(finalizedBlock.ID(), noopSetter)
	require.Nil(t, err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	assert.Nil(t, err)
	builtCollection := built.Payload.Collection

	// tx2 and tx3 should be in the built collection
	assert.Len(t, builtCollection.Light().Transactions, 2)
	assert.True(t, collectionContains(builtCollection, tx2.ID(), tx3.ID()))
	assert.False(t, collectionContains(builtCollection, tx1.ID()))
}

func (suite *BuilderSuite) TestBuildOn_LargeHistory() {
	t := suite.T()

	// use a mempool with 2000 transactions, one per block
	suite.pool = stdmap.NewTransactions(2000)
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool, builder.WithMaxCollectionSize(10000))

	// get a valid reference block ID
	final, err := suite.protoState.Final().Head()
	require.Nil(t, err)
	refID := final.ID()

	// keep track of the head of the chain
	head := *suite.genesis

	// keep track of invalidated transaction IDs
	var invalidatedTxIds []flow.Identifier

	// create 1000 blocks containing 1000 transactions
	//TODO for now limit this test to no more blocks than we look back by
	// when de-duplicating transactions.
	for i := 0; i < flow.DefaultTransactionExpiry-1; i++ {

		// create a transaction
		tx := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
			tx.ReferenceBlockID = refID
			tx.ProposalKey.SequenceNumber = uint64(i)
		})
		added := suite.pool.Add(&tx)
		assert.True(t, added)

		// 1/3 of the time create a conflicting fork that will be invalidated
		// don't do this the first and last few times to ensure we don't
		// try to fork genesis and the the last block is the valid fork.
		conflicting := rand.Intn(3) == 0 && i > 5 && i < 995

		// by default, build on the head - if we are building a
		// conflicting fork, build on the parent of the head
		parent := head
		if conflicting {
			err = suite.db.View(procedure.RetrieveClusterBlock(parent.Header.ParentID, &parent))
			assert.Nil(t, err)
			// add the transaction to the invalidated list
			invalidatedTxIds = append(invalidatedTxIds, tx.ID())
		}

		// create a block containing the transaction
		block := unittest.ClusterBlockWithParent(&head)
		payload := suite.Payload(&tx)
		block.SetPayload(payload)
		suite.InsertBlock(block)

		// reset the valid head if we aren't building a conflicting fork
		if !conflicting {
			head = block
			err = suite.db.Update(procedure.FinalizeClusterBlock(block.ID()))
			assert.Nil(t, err)
		}
	}

	t.Log("conflicting: ", len(invalidatedTxIds))

	// build on the the head block
	header, err := suite.builder.BuildOn(head.ID(), noopSetter)
	require.Nil(t, err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	require.Nil(t, err)
	builtCollection := built.Payload.Collection

	// payload should only contain transactions from invalidated blocks
	assert.Len(t, builtCollection.Transactions, len(invalidatedTxIds), "expected len=%d, got len=%d", len(invalidatedTxIds), len(builtCollection.Transactions))
	assert.True(t, collectionContains(builtCollection, invalidatedTxIds...))
}

func (suite *BuilderSuite) TestBuildOn_MaxCollectionSize() {
	// set the max collection size to 1
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool, builder.WithMaxCollectionSize(1))

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), noopSetter)
	suite.Require().Nil(err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Require().Nil(err)
	builtCollection := built.Payload.Collection

	// should be only 1 transaction in the collection
	suite.Assert().Equal(builtCollection.Len(), 1)
}

func (suite *BuilderSuite) TestBuildOn_MaxCollectionByteSize() {
	// set the max collection byte size to 400 (each tx is about 150 bytes)
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool, builder.WithMaxCollectionByteSize(400))

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), noopSetter)
	suite.Require().Nil(err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Require().Nil(err)
	builtCollection := built.Payload.Collection

	// should be only 2 transactions in the collection, since each tx is ~273 bytes and the limit is 600 bytes
	suite.Assert().Equal(builtCollection.Len(), 2)
}

func (suite *BuilderSuite) TestBuildOn_MaxCollectionTotalGas() {
	// set the max gas to 20,000
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool, builder.WithMaxCollectionTotalGas(20000))

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), noopSetter)
	suite.Require().Nil(err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Require().Nil(err)
	builtCollection := built.Payload.Collection

	// should be only 2 transactions in collection, since each transaction has gas limit of 9,999 and collection limit is set to 20,000
	suite.Assert().Equal(builtCollection.Len(), 2)
}

func (suite *BuilderSuite) TestBuildOn_ExpiredTransaction() {

	// create enough main-chain blocks that an expired transaction is possible
	genesis, err := suite.protoState.Final().Head()
	suite.Require().Nil(err)

	head := genesis
	for i := 0; i < flow.DefaultTransactionExpiry+1; i++ {
		block := unittest.BlockWithParentFixture(head)
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.PayloadHash = block.Payload.Hash()
		err = suite.protoState.Extend(context.Background(), block)
		suite.Require().Nil(err)
		err = suite.protoState.Finalize(context.Background(), block.ID())
		suite.Require().Nil(err)
		head = block.Header
	}

	// reset the pool and builder
	suite.pool = stdmap.NewTransactions(10)
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool)

	// insert a transaction referring genesis (now expired)
	tx1 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
		tx.ReferenceBlockID = genesis.ID()
		tx.ProposalKey.SequenceNumber = 0
	})
	added := suite.pool.Add(&tx1)
	suite.Assert().True(added)

	// insert a transaction referencing the head (valid)
	tx2 := unittest.TransactionBodyFixture(func(tx *flow.TransactionBody) {
		tx.ReferenceBlockID = head.ID()
		tx.ProposalKey.SequenceNumber = 1
	})
	added = suite.pool.Add(&tx2)
	suite.Assert().True(added)

	suite.T().Log("tx1: ", tx1.ID())
	suite.T().Log("tx2: ", tx2.ID())

	// build a block
	header, err := suite.builder.BuildOn(suite.genesis.ID(), noopSetter)
	suite.Require().Nil(err)

	// retrieve the built block from storage
	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Require().Nil(err)
	builtCollection := built.Payload.Collection

	// the block should only contain the un-expired transaction
	suite.Assert().False(collectionContains(builtCollection, tx1.ID()))
	suite.Assert().True(collectionContains(builtCollection, tx2.ID()))
	// the expired transaction should have been removed from the mempool
	suite.Assert().False(suite.pool.Has(tx1.ID()))
}

func (suite *BuilderSuite) TestBuildOn_EmptyMempool() {

	// start with an empty mempool
	suite.pool = stdmap.NewTransactions(1000)
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool)

	header, err := suite.builder.BuildOn(suite.genesis.ID(), noopSetter)
	suite.Require().Nil(err)

	var built model.Block
	err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
	suite.Require().Nil(err)

	// should reference a valid reference block
	// (since genesis is the only block, it's the only valid reference)
	mainGenesis, err := suite.protoState.AtHeight(0).Head()
	suite.Assert().Nil(err)
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
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool,
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
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, noopSetter)
		suite.Require().Nil(err)
		parentID = header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
		suite.Assert().Nil(err)
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
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool,
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
			KeyIndex:       rand.Uint64(),
			SequenceNumber: rand.Uint64(),
		}
		return &tx
	}
	suite.FillPool(100, create)

	// since rate limiting does not apply to non-payer keys, we should fill all collections in 10 blocks
	parentID := suite.genesis.ID()
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, noopSetter)
		suite.Require().Nil(err)
		parentID = header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
		suite.Assert().Nil(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 10)
	}
}

// When configured with a rate limit of k>1, we should be able to include up to
// k transactions with a given payer per collection
func (suite *BuilderSuite) TestBuildOn_HighRateLimit() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with 5 tx/payer and max 10 tx/collection
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool,
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
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, noopSetter)
		suite.Require().Nil(err)
		parentID = header.ID()

		// each collection should be half-full with 5 transactions
		var built model.Block
		err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
		suite.Assert().Nil(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 5)
	}
}

// When configured with a rate limit of k<1, we should be able to include 1
// transactions with a given payer every ceil(1/k) collections
func (suite *BuilderSuite) TestBuildOn_LowRateLimit() {

	// start with an empty mempool
	suite.ClearPool()

	// create builder with .5 tx/payer and max 10 tx/collection
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool,
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
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, noopSetter)
		suite.Require().Nil(err)
		parentID = header.ID()

		// collections should either be empty or have 1 transaction
		var built model.Block
		err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
		suite.Assert().Nil(err)
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
	suite.builder = builder.NewBuilder(suite.db, trace.NewNoopTracer(), suite.headers, suite.headers, suite.payloads, suite.pool,
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
	for i := 0; i < 10; i++ {
		header, err := suite.builder.BuildOn(parentID, noopSetter)
		suite.Require().Nil(err)
		parentID = header.ID()

		// each collection should be full with 10 transactions
		var built model.Block
		err = suite.db.View(procedure.RetrieveClusterBlock(header.ID(), &built))
		suite.Assert().Nil(err)
		suite.Assert().Len(built.Payload.Collection.Transactions, 10)

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

		suite.genesis = model.Genesis()
		suite.chainID = suite.genesis.Header.ChainID

		suite.pool = stdmap.NewTransactions(1000)

		suite.dbdir = unittest.TempDir(b)
		suite.db = unittest.BadgerDB(b, suite.dbdir)
		defer func() {
			err = suite.db.Close()
			assert.Nil(b, err)
			err = os.RemoveAll(suite.dbdir)
			assert.Nil(b, err)
		}()

		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		headers, _, _, _, _, blocks, _, _, _, _ := sutil.StorageLayer(suite.T(), suite.db)
		suite.headers = headers
		suite.blocks = blocks
		suite.payloads = storage.NewClusterPayloads(metrics, suite.db)

		stateRoot, err := clusterkv.NewStateRoot(suite.genesis)

		state, err := clusterkv.Bootstrap(suite.db, stateRoot)
		assert.Nil(b, err)

		suite.state, err = clusterkv.NewMutableState(state, tracer, suite.headers, suite.payloads)
		assert.Nil(b, err)

		// add some transactions to transaction pool
		for i := 0; i < 3; i++ {
			tx := unittest.TransactionBodyFixture()
			added := suite.pool.Add(&tx)
			assert.True(b, added)
		}

		// create the builder
		suite.builder = builder.NewBuilder(suite.db, tracer, suite.headers, suite.headers, suite.payloads, suite.pool)
	}

	// create a block history to test performance against
	final := suite.genesis
	for i := 0; i < size; i++ {
		block := unittest.ClusterBlockWithParent(final)
		err := suite.db.Update(procedure.InsertClusterBlock(&block))
		require.Nil(b, err)

		// finalize the block 80% of the time, resulting in a fork-rate of 20%
		if rand.Intn(100) < 80 {
			err = suite.db.Update(procedure.FinalizeClusterBlock(block.ID()))
			require.Nil(b, err)
			final = &block
		}
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err := suite.builder.BuildOn(final.ID(), noopSetter)
		assert.Nil(b, err)
	}
}
