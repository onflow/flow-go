package ingestion

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/component"
	downloadermock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	// protocol state
	proto struct {
		state    *protocol.FollowerState
		snapshot *protocol.Snapshot
		params   *protocol.Params
	}

	me             *module.Local
	request        *module.Requester
	provider       *mocknetwork.Engine
	blocks         *storage.Blocks
	headers        *storage.Headers
	collections    *storage.Collections
	transactions   *storage.Transactions
	receipts       *storage.ExecutionReceipts
	results        *storage.ExecutionResults
	seals          *storage.Seals
	downloader     *downloadermock.Downloader
	sealedBlock    *flow.Header
	finalizedBlock *flow.Header
	log            zerolog.Logger

	eng    *Engine
	cancel context.CancelFunc
}

func TestIngestEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) TearDownTest() {
	s.cancel()
}

func (s *Suite) SetupTest() {
	s.log = zerolog.New(os.Stderr)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	obsIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))

	// mock out protocol state
	s.proto.state = new(protocol.FollowerState)
	s.proto.snapshot = new(protocol.Snapshot)
	s.proto.params = new(protocol.Params)
	s.finalizedBlock = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0))
	s.proto.state.On("Identity").Return(obsIdentity, nil)
	s.proto.state.On("Final").Return(s.proto.snapshot, nil)
	s.proto.state.On("Params").Return(s.proto.params)
	s.proto.snapshot.On("Head").Return(
		func() *flow.Header {
			return s.finalizedBlock
		},
		nil,
	).Maybe()

	s.me = new(module.Local)
	s.me.On("NodeID").Return(obsIdentity.NodeID)

	net := new(mocknetwork.Network)
	conduit := new(mocknetwork.Conduit)
	net.On("Register", channels.ReceiveReceipts, mock.Anything).
		Return(conduit, nil).
		Once()
	s.request = new(module.Requester)

	s.provider = new(mocknetwork.Engine)
	s.blocks = new(storage.Blocks)
	s.headers = new(storage.Headers)
	s.collections = new(storage.Collections)
	s.transactions = new(storage.Transactions)
	s.receipts = new(storage.ExecutionReceipts)
	s.results = new(storage.ExecutionResults)
	collectionsToMarkFinalized, err := stdmap.NewTimes(100)
	require.NoError(s.T(), err)
	collectionsToMarkExecuted, err := stdmap.NewTimes(100)
	require.NoError(s.T(), err)
	blocksToMarkExecuted, err := stdmap.NewTimes(100)
	require.NoError(s.T(), err)

	eng, err := New(s.log, net, s.proto.state, s.me, s.request, s.blocks, s.headers, s.collections,
		s.transactions, s.results, s.receipts, metrics.NewNoopCollector(), collectionsToMarkFinalized, collectionsToMarkExecuted,
		blocksToMarkExecuted)
	require.NoError(s.T(), err)

	s.blocks.On("GetLastFullBlockHeight").Once().Return(uint64(0), errors.New("do nothing"))

	irrecoverableCtx, _ := irrecoverable.WithSignaler(ctx)
	eng.ComponentManager.Start(irrecoverableCtx)
	<-eng.Ready()

	s.eng = eng
}

// TestOnFinalizedBlock checks that when a block is received, a request for each individual collection is made
func (s *Suite) TestOnFinalizedBlock() {
	s.blocks.On("GetLastFullBlockHeight").Return(uint64(0), nil).Once()

	block := unittest.BlockFixture()
	block.SetPayload(unittest.PayloadFixture(
		unittest.WithGuarantees(unittest.CollectionGuaranteesFixture(4)...),
		unittest.WithExecutionResults(unittest.ExecutionResultFixture()),
	))

	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	refBlockID := unittest.IdentifierFixture()
	for _, guarantee := range block.Payload.Guarantees {
		guarantee.ReferenceBlockID = refBlockID
		// guarantee signers must be cluster committee members, so that access will fetch collection from
		// the signers that are specified by guarantee.SignerIndices
		indices, err := signature.EncodeSignersToIndices(clusterCommittee.NodeIDs(), clusterCommittee.NodeIDs())
		require.NoError(s.T(), err)
		guarantee.SignerIndices = indices
	}

	hotstuffBlock := hotmodel.Block{
		BlockID: block.ID(),
	}

	// we should query the block once and index the guarantee payload once
	s.blocks.On("ByID", block.ID()).Return(&block, nil).Twice()
	for _, g := range block.Payload.Guarantees {
		collection := unittest.CollectionFixture(1)
		light := collection.Light()
		s.collections.On("LightByID", g.CollectionID).Return(&light, nil).Twice()
	}

	// expect that the block storage is indexed with each of the collection guarantee
	s.blocks.On("IndexBlockForCollections", block.ID(), []flow.Identifier(flow.GetIDs(block.Payload.Guarantees))).Return(nil).Once()

	cluster := new(protocol.Cluster)
	cluster.On("Members").Return(clusterCommittee, nil)
	epoch := new(protocol.Epoch)
	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs := new(protocol.EpochQuery)
	epochs.On("Current").Return(epoch)
	snap := new(protocol.Snapshot)
	snap.On("Epochs").Return(epochs)
	s.proto.state.On("AtBlockID", refBlockID).Return(snap)
	s.results.On("Index", mock.Anything, mock.Anything).Return(nil)

	// for each of the guarantees, we should request the corresponding collection once
	needed := make(map[flow.Identifier]struct{})
	for _, guarantee := range block.Payload.Guarantees {
		needed[guarantee.ID()] = struct{}{}
	}

	wg := sync.WaitGroup{}
	wg.Add(4)

	s.request.On("EntityByID", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			collID := args.Get(0).(flow.Identifier)
			_, pending := needed[collID]
			s.Assert().True(pending, "collection should be pending (%x)", collID)
			delete(needed, collID)
			wg.Done()
		},
	)

	// process the block through the finalized callback
	s.eng.OnFinalizedBlock(&hotstuffBlock)
	s.Assertions.Eventually(func() bool {
		wg.Wait()
		return true
	}, time.Millisecond*20, time.Millisecond)

	// assert that the block was retrieved and all collections were requested
	s.headers.AssertExpectations(s.T())
	s.request.AssertNumberOfCalls(s.T(), "EntityByID", len(block.Payload.Guarantees))
	s.request.AssertNumberOfCalls(s.T(), "Index", len(block.Payload.Seals))
}

// TestOnCollection checks that when a Collection is received, it is persisted
func (s *Suite) TestOnCollection() {
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the light collection and index its transactions
	s.collections.On("StoreLightAndIndexByTransaction", &light).Return(nil).Once()

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	s.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			s.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)

	err := indexer.HandleCollection(&collection, s.collections, s.transactions, s.log)
	require.NoError(s.T(), err)

	// check that the collection was stored and indexed, and we stored all transactions
	s.collections.AssertExpectations(s.T())
	s.transactions.AssertNumberOfCalls(s.T(), "Store", len(collection.Transactions))
}

// TestExecutionReceiptsAreIndexed checks that execution receipts are properly indexed
func (s *Suite) TestExecutionReceiptsAreIndexed() {

	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the light collection and index its transactions
	s.collections.On("StoreLightAndIndexByTransaction", &light).Return(nil).Once()
	block := &flow.Block{
		Header:  &flow.Header{Height: 0},
		Payload: &flow.Payload{Guarantees: []*flow.CollectionGuarantee{}},
	}
	s.blocks.On("ByID", mock.Anything).Return(block, nil)

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	s.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			s.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)
	er1 := unittest.ExecutionReceiptFixture()
	er2 := unittest.ExecutionReceiptFixture()

	s.receipts.On("Store", mock.Anything).Return(nil)
	s.blocks.On("ByID", er1.ExecutionResult.BlockID).Return(nil, storerr.ErrNotFound)

	s.receipts.On("Store", mock.Anything).Return(nil)
	s.blocks.On("ByID", er2.ExecutionResult.BlockID).Return(nil, storerr.ErrNotFound)

	err := s.eng.handleExecutionReceipt(originID, er1)
	require.NoError(s.T(), err)

	err = s.eng.handleExecutionReceipt(originID, er2)
	require.NoError(s.T(), err)

	s.receipts.AssertExpectations(s.T())
	s.results.AssertExpectations(s.T())
	s.receipts.AssertExpectations(s.T())
}

// TestOnCollectionDuplicate checks that when a duplicate collection is received, the node doesn't
// crash but just ignores its transactions.
func (s *Suite) TestOnCollectionDuplicate() {
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	// we should store the light collection and index its transactions
	s.collections.On("StoreLightAndIndexByTransaction", &light).Return(storerr.ErrAlreadyExists).Once()

	// for each transaction in the collection, we should store it
	needed := make(map[flow.Identifier]struct{})
	for _, txID := range light.Transactions {
		needed[txID] = struct{}{}
	}
	s.transactions.On("Store", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			tx := args.Get(0).(*flow.TransactionBody)
			_, pending := needed[tx.ID()]
			s.Assert().True(pending, "tx not pending (%x)", tx.ID())
		},
	)

	err := indexer.HandleCollection(&collection, s.collections, s.transactions, s.log)
	require.NoError(s.T(), err)

	// check that the collection was stored and indexed, and we stored all transactions
	s.collections.AssertExpectations(s.T())
	s.transactions.AssertNotCalled(s.T(), "Store", "should not store any transactions")
}

// TestRequestMissingCollections tests that the all missing collections are requested on the call to requestMissingCollections
func (s *Suite) TestRequestMissingCollections() {

	blkCnt := 3
	startHeight := uint64(1000)
	blocks := make([]flow.Block, blkCnt)
	heightMap := make(map[uint64]*flow.Block, blkCnt)

	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()

	// generate the test blocks and collections
	var collIDs []flow.Identifier
	refBlockID := unittest.IdentifierFixture()
	for i := 0; i < blkCnt; i++ {
		block := unittest.BlockFixture()
		block.SetPayload(unittest.PayloadFixture(
			unittest.WithGuarantees(
				unittest.CollectionGuaranteesFixture(4, unittest.WithCollRef(refBlockID))...),
		))
		// some blocks may not be present hence add a gap
		height := startHeight + uint64(i)
		block.Header.Height = height
		blocks[i] = block
		heightMap[height] = &block
		for _, c := range block.Payload.Guarantees {
			collIDs = append(collIDs, c.CollectionID)
			c.ReferenceBlockID = refBlockID

			// guarantee signers must be cluster committee members, so that access will fetch collection from
			// the signers that are specified by guarantee.SignerIndices
			indices, err := signature.EncodeSignersToIndices(clusterCommittee.NodeIDs(), clusterCommittee.NodeIDs())
			require.NoError(s.T(), err)
			c.SignerIndices = indices
		}
	}

	// setup the block storage mock
	// each block should be queried by height
	s.blocks.On("ByHeight", mock.IsType(uint64(0))).Return(
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
	s.blocks.On("GetLastFullBlockHeight").Return(startHeight-1, nil)
	// consider the last test block as the head
	s.finalizedBlock = blocks[blkCnt-1].Header

	// p is the probability of not receiving the collection before the next poll and it
	// helps simulate the slow trickle of the requested collections being received
	var p float32

	// rcvdColl is the map simulating the collection storage key-values
	rcvdColl := make(map[flow.Identifier]struct{})

	// for the first lookup call for each collection, it will be reported as missing from db
	// for the subsequent calls, it will be reported as present with the probability p
	s.collections.On("LightByID", mock.Anything).Return(
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
		s.request.On("EntityByID", c, mock.Anything).Return()
	}
	// force should be called once
	s.request.On("Force").Return()

	cluster := new(protocol.Cluster)
	cluster.On("Members").Return(clusterCommittee, nil)
	epoch := new(protocol.Epoch)
	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs := new(protocol.EpochQuery)
	epochs.On("Current").Return(epoch)
	snap := new(protocol.Snapshot)
	snap.On("Epochs").Return(epochs)
	s.proto.state.On("AtBlockID", refBlockID).Return(snap)

	assertExpectations := func() {
		s.request.AssertExpectations(s.T())
		s.collections.AssertExpectations(s.T())
		s.proto.snapshot.AssertExpectations(s.T())
		s.blocks.AssertExpectations(s.T())
	}

	// test 1 - collections are not received before timeout
	s.Run("timeout before all missing collections are received", func() {

		// simulate that collection are never received
		p = 1

		// timeout after 3 db polls
		ctx, cancel := context.WithTimeout(context.Background(), 100*defaultCollectionCatchupDBPollInterval)
		defer cancel()

		err := s.eng.requestMissingCollections(ctx)

		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "context deadline exceeded")

		assertExpectations()
	})
	// test 2 - all collections are eventually received before the deadline
	s.Run("all missing collections are received", func() {

		// 90% of the time, collections are reported as not received when the collection storage is queried
		p = 0.9

		ctx, cancel := context.WithTimeout(context.Background(), defaultCollectionCatchupTimeout)
		defer cancel()

		err := s.eng.requestMissingCollections(ctx)

		require.NoError(s.T(), err)
		require.Len(s.T(), rcvdColl, len(collIDs))

		assertExpectations()
	})
}

// TestProcessBackgroundCalls tests that updateLastFullBlockReceivedIndex and checkMissingCollections
// function calls keep the FullBlockIndex up-to-date and request collections if blocks with missing
// collections exceed the threshold.
func (s *Suite) TestProcessBackgroundCalls() {
	blkCnt := 3
	collPerBlk := 10
	startHeight := uint64(1000)
	blocks := make([]flow.Block, blkCnt)
	heightMap := make(map[uint64]*flow.Block, blkCnt)
	collMap := make(map[flow.Identifier]*flow.LightCollection, blkCnt*collPerBlk)

	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()

	refBlockID := unittest.IdentifierFixture()
	// generate the test blocks, cgs and collections
	for i := 0; i < blkCnt; i++ {
		guarantees := make([]*flow.CollectionGuarantee, collPerBlk)
		for j := 0; j < collPerBlk; j++ {
			coll := unittest.CollectionFixture(2).Light()
			collMap[coll.ID()] = &coll
			cg := unittest.CollectionGuaranteeFixture(func(cg *flow.CollectionGuarantee) {
				cg.CollectionID = coll.ID()
				cg.ReferenceBlockID = refBlockID
			})

			// guarantee signers must be cluster committee members, so that access will fetch collection from
			// the signers that are specified by guarantee.SignerIndices
			indices, err := signature.EncodeSignersToIndices(clusterCommittee.NodeIDs(), clusterCommittee.NodeIDs())
			require.NoError(s.T(), err)
			cg.SignerIndices = indices
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
	s.blocks.On("ByHeight", mock.IsType(uint64(0))).Return(
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

	cluster := new(protocol.Cluster)
	cluster.On("Members").Return(clusterCommittee, nil)
	epoch := new(protocol.Epoch)
	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs := new(protocol.EpochQuery)
	epochs.On("Current").Return(epoch)
	snap := new(protocol.Snapshot)
	snap.On("Epochs").Return(epochs)
	s.proto.state.On("AtBlockID", refBlockID).Return(snap)

	// blkMissingColl controls which collections are reported as missing by the collections storage mock
	blkMissingColl := make([]bool, blkCnt)
	for i := 0; i < blkCnt; i++ {
		blkMissingColl[i] = false
		for _, cg := range blocks[i].Payload.Guarantees {
			j := i
			s.collections.On("LightByID", cg.CollectionID).Return(
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

	// consider the last test block as the head
	s.finalizedBlock = finalizedBlk.Header

	s.Run("full block height index is advanced if newer full blocks are discovered", func() {
		block := blocks[1]
		s.blocks.On("UpdateLastFullBlockHeight", finalizedHeight).Return(nil).Once()
		s.blocks.On("GetLastFullBlockHeight").Return(func() (uint64, error) {
			return block.Header.Height, nil
		}).Once()

		err := s.eng.updateLastFullBlockReceivedIndex()
		s.Require().NoError(err)

		s.blocks.AssertExpectations(s.T())
	})

	s.Run("full block height index is not advanced beyond finalized blocks", func() {
		s.blocks.On("GetLastFullBlockHeight").Return(func() (uint64, error) {
			return finalizedHeight, nil
		}).Once()

		err := s.eng.updateLastFullBlockReceivedIndex()
		s.Require().NoError(err)

		s.blocks.AssertExpectations(s.T()) // not new call to UpdateLastFullBlockHeight should be made
	})

	s.Run("missing collections are requested when count exceeds defaultMissingCollsForBlkThreshold", func() {
		// root block is the last complete block
		s.blocks.On("GetLastFullBlockHeight").Return(func() (uint64, error) {
			return rootBlkHeight, nil
		}).Once()

		// lower the block threshold to request missing collections
		defaultMissingCollsForBlkThreshold = 2

		// mark all blocks beyond the root block as incomplete
		for i := 1; i < blkCnt; i++ {
			blkMissingColl[i] = true
			// setup receive engine expectations
			for _, cg := range blocks[i].Payload.Guarantees {
				s.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Once()
			}
		}

		err := s.eng.checkMissingCollections()
		s.Require().NoError(err)

		// assert that missing collections are requested
		s.request.AssertExpectations(s.T())

		// last full blk index is not advanced
		s.blocks.AssertExpectations(s.T()) // no new call to UpdateLastFullBlockHeight should be made
	})

	s.Run("missing collections are requested when count exceeds defaultMissingCollsForAgeThreshold", func() {
		// root block is the last complete block
		s.blocks.On("GetLastFullBlockHeight").Return(func() (uint64, error) {
			return rootBlkHeight, nil
		}).Once()

		// lower the height threshold to request missing collections
		defaultMissingCollsForAgeThreshold = 1

		// raise the block threshold to ensure it does not trigger missing collection request
		defaultMissingCollsForBlkThreshold = blkCnt + 1

		// mark all blocks beyond the root block as incomplete
		for i := 1; i < blkCnt; i++ {
			blkMissingColl[i] = true
			// setup receive engine expectations
			for _, cg := range blocks[i].Payload.Guarantees {
				s.request.On("EntityByID", cg.CollectionID, mock.Anything).Return().Once()
			}
		}

		err := s.eng.checkMissingCollections()
		s.Require().NoError(err)

		// assert that missing collections are requested
		s.request.AssertExpectations(s.T())

		// last full blk index is not advanced
		s.blocks.AssertExpectations(s.T()) // not new call to UpdateLastFullBlockHeight should be made
	})

	s.Run("missing collections are not requested if defaultMissingCollsForBlkThreshold not reached", func() {
		// root block is the last complete block
		s.blocks.On("GetLastFullBlockHeight").Return(func() (uint64, error) {
			return rootBlkHeight, nil
		}).Once()

		// raise the thresholds to avoid requesting missing collections
		defaultMissingCollsForAgeThreshold = 3
		defaultMissingCollsForBlkThreshold = 3

		// mark all blocks beyond the root block as incomplete
		for i := 1; i < blkCnt; i++ {
			blkMissingColl[i] = true
		}

		err := s.eng.checkMissingCollections()
		s.Require().NoError(err)

		// assert that missing collections are not requested even though there are collections missing
		s.request.AssertExpectations(s.T())

		// last full blk index is not advanced
		s.blocks.AssertExpectations(s.T()) // not new call to UpdateLastFullBlockHeight should be made
	})
}

func (s *Suite) TestComponentShutdown() {
	// start then shut down the engine
	unittest.AssertClosesBefore(s.T(), s.eng.Ready(), 10*time.Millisecond)
	s.cancel()
	unittest.AssertClosesBefore(s.T(), s.eng.Done(), 10*time.Millisecond)

	err := s.eng.ProcessLocal(&flow.ExecutionReceipt{})
	s.Assert().ErrorIs(err, component.ErrComponentShutdown)
}
