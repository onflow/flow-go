package ingestion

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/mocknetwork"

	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	// protocol state
	proto struct {
		state    *protocol.MutableState
		snapshot *protocol.Snapshot
		params   *protocol.Params
	}

	me           *module.Local
	request      *module.Requester
	provider     *mocknetwork.Engine
	blocks       *storage.Blocks
	headers      *storage.Headers
	collections  *storage.Collections
	transactions *storage.Transactions
	receipts     *storage.ExecutionReceipts
	results      *storage.ExecutionResults

	eng *Engine
}

func TestIngestEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	log := zerolog.New(os.Stderr)

	obsIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))

	// mock out protocol state
	suite.proto.state = new(protocol.MutableState)
	suite.proto.snapshot = new(protocol.Snapshot)
	suite.proto.params = new(protocol.Params)
	suite.proto.state.On("Identity").Return(obsIdentity, nil)
	suite.proto.state.On("Final").Return(suite.proto.snapshot, nil)
	suite.proto.state.On("Params").Return(suite.proto.params)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(obsIdentity.NodeID)

	net := new(mocknetwork.Network)
	conduit := new(mocknetwork.Conduit)
	net.On("Register", engine.ReceiveReceipts, mock.Anything).
		Return(conduit, nil).
		Once()
	suite.request = new(module.Requester)

	suite.provider = new(mocknetwork.Engine)
	suite.blocks = new(storage.Blocks)
	suite.headers = new(storage.Headers)
	suite.collections = new(storage.Collections)
	suite.transactions = new(storage.Transactions)
	suite.receipts = new(storage.ExecutionReceipts)
	suite.results = new(storage.ExecutionResults)
	collectionsToMarkFinalized, err := stdmap.NewTimes(100)
	require.NoError(suite.T(), err)
	collectionsToMarkExecuted, err := stdmap.NewTimes(100)
	require.NoError(suite.T(), err)
	blocksToMarkExecuted, err := stdmap.NewTimes(100)
	require.NoError(suite.T(), err)

	rpcEng := rpc.New(log, suite.proto.state, rpc.Config{}, nil, nil, suite.blocks, suite.headers, suite.collections,
		suite.transactions, suite.receipts, suite.results, flow.Testnet, metrics.NewNoopCollector(), 0, 0, false, false, nil, nil)

	eng, err := New(log, net, suite.proto.state, suite.me, suite.request, suite.blocks, suite.headers, suite.collections,
		suite.transactions, suite.results, suite.receipts, metrics.NewNoopCollector(), collectionsToMarkFinalized, collectionsToMarkExecuted,
		blocksToMarkExecuted, rpcEng)
	require.NoError(suite.T(), err)

	suite.eng = eng

}

// TestOnFinalizedBlock checks that when a block is received, a request for each individual collection is made
func (suite *Suite) TestOnFinalizedBlock() {

	block := unittest.BlockFixture()
	block.SetPayload(unittest.PayloadFixture(
		unittest.WithGuarantees(unittest.CollectionGuaranteesFixture(4)...),
	))
	hotstuffBlock := hotmodel.Block{
		BlockID: block.ID(),
	}

	// we should query the block once and index the guarantee payload once
	suite.blocks.On("ByID", block.ID()).Return(&block, nil).Twice()
	for _, g := range block.Payload.Guarantees {
		collection := unittest.CollectionFixture(1)
		light := collection.Light()
		suite.collections.On("LightByID", g.CollectionID).Return(&light, nil).Twice()
	}

	// expect that the block storage is indexed with each of the collection guarantee
	suite.blocks.On("IndexBlockForCollections", block.ID(), flow.GetIDs(block.Payload.Guarantees)).Return(nil).Once()

	// for each of the guarantees, we should request the corresponding collection once
	needed := make(map[flow.Identifier]struct{})
	for _, guarantee := range block.Payload.Guarantees {
		needed[guarantee.ID()] = struct{}{}
	}
	suite.request.On("EntityByID", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			collID := args.Get(0).(flow.Identifier)
			_, pending := needed[collID]
			suite.Assert().True(pending, "collection should be pending (%x)", collID)
			delete(needed, collID)
		},
	)

	// process the block through the finalized callback
	suite.eng.OnFinalizedBlock(&hotstuffBlock)

	// wait for engine shutdown
	done := suite.eng.unit.Done()
	assert.Eventually(suite.T(), func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 20*time.Millisecond)

	// assert that the block was retrieved and all collections were requested
	suite.headers.AssertExpectations(suite.T())
	suite.request.AssertNumberOfCalls(suite.T(), "EntityByID", len(block.Payload.Guarantees))
}

// TestOnCollection checks that when a Collection is received, it is persisted
func (suite *Suite) TestOnCollection() {

	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the light collection and index its transactions
	suite.collections.On("StoreLightAndIndexByTransaction", &light).Return(nil).Once()

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	suite.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			suite.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)

	// process the block through the collection callback
	suite.eng.OnCollection(originID, &collection)

	// wait for engine to be done processing
	done := suite.eng.unit.Done()
	assert.Eventually(suite.T(), func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 20*time.Millisecond)

	// check that the collection was stored and indexed, and we stored all transactions
	suite.collections.AssertExpectations(suite.T())
	suite.transactions.AssertNumberOfCalls(suite.T(), "Store", len(collection.Transactions))
}

// TestExecutionResultsAreIndexed checks that execution results are properly indexedd
func (suite *Suite) TestExecutionResultsAreIndexed() {

	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the light collection and index its transactions
	suite.collections.On("StoreLightAndIndexByTransaction", &light).Return(nil).Once()

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	suite.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			suite.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)
	er1 := unittest.ExecutionReceiptFixture()
	er2 := unittest.ExecutionReceiptFixture()

	suite.receipts.On("Store", mock.Anything).Return(nil)
	suite.results.On("ForceIndex", mock.Anything, mock.Anything).Return(nil)
	suite.blocks.On("ByID", er1.ExecutionResult.BlockID).Return(nil, storerr.ErrNotFound)

	suite.receipts.On("Store", mock.Anything).Return(nil)
	suite.results.On("ForceIndex", mock.Anything, mock.Anything).Return(nil)
	suite.blocks.On("ByID", er2.ExecutionResult.BlockID).Return(nil, storerr.ErrNotFound)

	err := suite.eng.handleExecutionReceipt(originID, er1)
	require.NoError(suite.T(), err)

	err = suite.eng.handleExecutionReceipt(originID, er2)
	require.NoError(suite.T(), err)

	suite.receipts.AssertExpectations(suite.T())
	suite.results.AssertExpectations(suite.T())
	suite.receipts.AssertExpectations(suite.T())
}

// TestOnCollection checks that when a duplicate collection is received, the node doesn't
// crash but just ignores its transactions.
func (suite *Suite) TestOnCollectionDuplicate() {

	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the light collection and index its transactions
	suite.collections.On("StoreLightAndIndexByTransaction", &light).Return(storerr.ErrAlreadyExists).Once()

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	suite.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			suite.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)

	// process the block through the collection callback
	suite.eng.OnCollection(originID, &collection)

	// wait for engine to be done processing
	done := suite.eng.unit.Done()
	assert.Eventually(suite.T(), func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 20*time.Millisecond)

	// check that the collection was stored and indexed, and we stored all transactions
	suite.collections.AssertExpectations(suite.T())
	suite.transactions.AssertNotCalled(suite.T(), "Store", "should not store any transactions")
}

// TestRequestMissingCollections tests that the all missing collections are requested on the call to requestMissingCollections
func (suite *Suite) TestRequestMissingCollections() {

	blkCnt := 3
	startHeight := uint64(1000)
	blocks := make([]flow.Block, blkCnt)
	heightMap := make(map[uint64]*flow.Block, blkCnt)

	// generate the test blocks and collections
	var collIDs []flow.Identifier
	for i := 0; i < blkCnt; i++ {
		block := unittest.BlockFixture()
		block.SetPayload(unittest.PayloadFixture(
			unittest.WithGuarantees(unittest.CollectionGuaranteesFixture(4)...),
		))
		// some blocks may not be present hence add a gap
		height := startHeight + uint64(i)
		block.Header.Height = height
		blocks[i] = block
		heightMap[height] = &block
		for _, c := range block.Payload.Guarantees {
			collIDs = append(collIDs, c.CollectionID)
		}
	}

	// setup the block storage mock
	// each block should be queried by height
	suite.blocks.On("ByHeight", mock.IsType(uint64(0))).Return(
		func(h uint64) *flow.Block {
			// simulate a db lookup
			return heightMap[h]
		},
		func(h uint64) error {
			if _, ok := heightMap[h]; ok {
				return nil
			}
			return storerr.ErrNotFound
		})
	// consider collections are missing for all blocks
	suite.blocks.On("GetLastFullBlockHeight").Return(startHeight-1, nil)
	// consider the last test block as the head
	suite.proto.snapshot.On("Head").Return(blocks[blkCnt-1].Header, nil)

	// p is the probability of not receiving the collection before the next poll and it
	// helps simulate the slow trickle of the requested collections being received
	var p float32

	// rcvdColl is the map simulating the collection storage key-values
	rcvdColl := make(map[flow.Identifier]struct{})

	// for the first lookup call for each collection, it will be reported as missing from db
	// for the subsequent calls, it will be reported as present with the probability p
	suite.collections.On("LightByID", mock.Anything).Return(
		func(cID flow.Identifier) *flow.LightCollection {
			return nil // the actual collection object return is never really read
		},
		func(cID flow.Identifier) error {
			if _, ok := rcvdColl[cID]; ok {
				return nil
			}
			if rand.Float32() >= p {
				rcvdColl[cID] = struct{}{}
			}
			return storerr.ErrNotFound
		}).
		// simulate some db i/o contention
		After(time.Millisecond * time.Duration(rand.Intn(5)))

	// setup the requester engine mock
	// entityByID should be called once per collection
	for _, c := range collIDs {
		suite.request.On("EntityByID", c, mock.Anything).Return()
	}
	// force should be called once
	suite.request.On("Force").Return()

	assertExpectations := func() {
		suite.request.AssertExpectations(suite.T())
		suite.collections.AssertExpectations(suite.T())
		suite.proto.snapshot.AssertExpectations(suite.T())
		suite.blocks.AssertExpectations(suite.T())
	}

	// test 1 - collections are not received before timeout
	suite.Run("timeout before all missing collections are received", func() {

		// simulate that collection are never received
		p = 1

		// timeout after 3 db polls
		ctx, cancel := context.WithTimeout(context.Background(), 100*defaultCollectionCatchupDBPollInterval)
		defer cancel()

		err := suite.eng.requestMissingCollections(ctx)

		require.Error(suite.T(), err)
		require.Contains(suite.T(), err.Error(), "context deadline exceeded")

		assertExpectations()
	})
	// test 2 - all collections are eventually received before the deadline
	suite.Run("all missing collections are received", func() {

		// 90% of the time, collections are reported as not received when the collection storage is queried
		p = 0.9

		ctx, cancel := context.WithTimeout(context.Background(), defaultCollectionCatchupTimeout)
		defer cancel()

		err := suite.eng.requestMissingCollections(ctx)

		require.NoError(suite.T(), err)
		require.Len(suite.T(), rcvdColl, len(collIDs))

		assertExpectations()
	})
}

// TestUpdateLastFullBlockReceivedIndex tests that UpdateLastFullBlockReceivedIndex function keeps the FullBlockIndex
// upto date and request collections if blocks with missing collections exceed the threshold.
func (suite *Suite) TestUpdateLastFullBlockReceivedIndex() {
	blkCnt := 3
	collPerBlk := 10
	startHeight := uint64(1000)
	blocks := make([]flow.Block, blkCnt)
	heightMap := make(map[uint64]*flow.Block, blkCnt)
	collMap := make(map[flow.Identifier]*flow.LightCollection, blkCnt*collPerBlk)

	// generate the test blocks, cgs and collections
	for i := 0; i < blkCnt; i++ {
		guarantees := make([]*flow.CollectionGuarantee, collPerBlk)
		for j := 0; j < collPerBlk; j++ {
			coll := unittest.CollectionFixture(2).Light()
			collMap[coll.ID()] = &coll
			cg := unittest.CollectionGuaranteeFixture(func(cg *flow.CollectionGuarantee) {
				cg.CollectionID = coll.ID()
			})
			guarantees[j] = cg
		}
		block := unittest.BlockFixture()
		block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantees...)))
		// set the height
		height := startHeight + uint64(i)
		block.Header.Height = height
		blocks[i] = block
		heightMap[height] = &block
	}

	rootBlk := blocks[0]
	rootBlkHeight := rootBlk.Header.Height
	finalizedBlk := blocks[blkCnt-1]
	finalizedHeight := finalizedBlk.Header.Height

	// setup the block storage mock
	// each block should be queried by height
	suite.blocks.On("ByHeight", mock.IsType(uint64(0))).Return(
		func(h uint64) *flow.Block {
			// simulate a db lookup
			return heightMap[h]
		},
		func(h uint64) error {
			if _, ok := heightMap[h]; ok {
				return nil
			}
			return storerr.ErrNotFound
		})

	// blkMissingColl controls which collections are reported as missing by the collections storage mock
	blkMissingColl := make([]bool, blkCnt)
	for i := 0; i < blkCnt; i++ {
		blkMissingColl[i] = false
		for _, cg := range blocks[i].Payload.Guarantees {
			j := i
			suite.collections.On("LightByID", cg.CollectionID).Return(
				func(cID flow.Identifier) *flow.LightCollection {
					return collMap[cID]
				},
				func(cID flow.Identifier) error {
					if blkMissingColl[j] {
						return storerr.ErrNotFound
					}
					return nil
				})
		}
	}

	var lastFullBlockHeight uint64
	var rtnErr error
	suite.blocks.On("GetLastFullBlockHeight").Return(
		func() uint64 {
			return lastFullBlockHeight
		},
		func() error {
			return rtnErr
		})

	// consider the last test block as the head
	suite.proto.snapshot.On("Head").Return(finalizedBlk.Header, nil)

	suite.Run("full block height index is created and advanced if not present", func() {
		// simulate the absence of the full block height index
		lastFullBlockHeight = 0
		rtnErr = storerr.ErrNotFound
		suite.proto.params.On("Root").Return(rootBlk.Header, nil)
		suite.blocks.On("UpdateLastFullBlockHeight", finalizedHeight).Return(nil).Once()

		suite.eng.updateLastFullBlockReceivedIndex()

		suite.blocks.AssertExpectations(suite.T())
	})

	suite.Run("full block height index is advanced if newer full blocks are discovered", func() {
		rtnErr = nil
		block := blocks[1]
		lastFullBlockHeight = block.Header.Height
		suite.blocks.On("UpdateLastFullBlockHeight", finalizedHeight).Return(nil).Once()

		suite.eng.updateLastFullBlockReceivedIndex()

		suite.blocks.AssertExpectations(suite.T())
	})

	suite.Run("full block height index is not advanced beyond finalized blocks", func() {
		rtnErr = nil
		lastFullBlockHeight = finalizedHeight

		suite.eng.updateLastFullBlockReceivedIndex()
		suite.blocks.AssertExpectations(suite.T()) // not new call to UpdateLastFullBlockHeight should be made
	})

	suite.Run("missing collections are requested when count exceeds defaultMissingCollsForBlkThreshold", func() {
		// root block is the last complete block
		rtnErr = nil
		lastFullBlockHeight = rootBlkHeight

		// lower the block threshold to request missing collections
		defaultMissingCollsForBlkThreshold = 2

		// mark all blocks beyond the root block as incomplete
		for i := 1; i < blkCnt; i++ {
			blkMissingColl[i] = true
			// setup receive engine expectations
			for _, cg := range blocks[i].Payload.Guarantees {
				suite.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Once()
			}
		}

		suite.eng.updateLastFullBlockReceivedIndex()

		// assert that missing collections are requested
		suite.request.AssertExpectations(suite.T())

		// last full blk index is not advanced
		suite.blocks.AssertExpectations(suite.T()) // no new call to UpdateLastFullBlockHeight should be made
	})

	suite.Run("missing collections are requested when count exceeds defaultMissingCollsForHeightThreshold", func() {
		// root block is the last complete block
		rtnErr = nil
		lastFullBlockHeight = rootBlkHeight

		// lower the height threshold to request missing collections
		defaultMissingCollsForHeightThreshold = 1

		// raise the block threshold to ensure it does not trigger missing collection request
		defaultMissingCollsForBlkThreshold = blkCnt + 1

		// mark all blocks beyond the root block as incomplete
		for i := 1; i < blkCnt; i++ {
			blkMissingColl[i] = true
			// setup receive engine expectations
			for _, cg := range blocks[i].Payload.Guarantees {
				suite.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Once()
			}
		}

		suite.eng.updateLastFullBlockReceivedIndex()

		// assert that missing collections are requested
		suite.request.AssertExpectations(suite.T())

		// last full blk index is not advanced
		suite.blocks.AssertExpectations(suite.T()) // not new call to UpdateLastFullBlockHeight should be made
	})

	suite.Run("missing collections are not requested if defaultMissingCollsForBlkThreshold not reached", func() {
		// root block is the last complete block
		rtnErr = nil
		lastFullBlockHeight = rootBlkHeight

		// raise the thresholds to avoid requesting missing collections
		defaultMissingCollsForHeightThreshold = 3
		defaultMissingCollsForBlkThreshold = 3

		// mark all blocks beyond the root block as incomplete
		for i := 1; i < blkCnt; i++ {
			blkMissingColl[i] = true
		}

		suite.eng.updateLastFullBlockReceivedIndex()

		// assert that missing collections are not requested even though there are collections missing
		suite.request.AssertExpectations(suite.T())

		// last full blk index is not advanced
		suite.blocks.AssertExpectations(suite.T()) // not new call to UpdateLastFullBlockHeight should be made
	})
}
