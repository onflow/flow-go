package access_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/cmd/build"
	hsmock "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/access/ingestion"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/rpc/backend/events"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
)

type Suite struct {
	suite.Suite
	state                *protocol.State
	sealedSnapshot       *protocol.Snapshot
	finalSnapshot        *protocol.Snapshot
	epochQuery           *protocol.EpochQuery
	params               *protocol.Params
	signerIndicesDecoder *hsmock.BlockSignerDecoder
	signerIds            flow.IdentifierList
	log                  zerolog.Logger
	net                  *mocknetwork.Network
	request              *mockmodule.Requester
	collClient           *accessmock.AccessAPIClient
	execClient           *accessmock.ExecutionAPIClient
	me                   *mockmodule.Local
	rootBlock            *flow.Header
	sealedBlock          *flow.Header
	finalizedBlock       *flow.Header
	chainID              flow.ChainID
	metrics              *metrics.NoopCollector
	finalizedHeaderCache module.FinalizedHeaderCache
	backend              *backend.Backend
	sporkID              flow.Identifier
	protocolStateVersion uint64
	lockManager          lockctx.Manager
}

// TestAccess tests scenarios which exercise multiple API calls using both the RPC handler and the ingest engine
// and using a real badger storage
func TestAccess(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.lockManager = storage.NewTestingLockManager()
	suite.log = zerolog.New(os.Stderr)
	suite.net = new(mocknetwork.Network)
	suite.state = new(protocol.State)
	suite.finalSnapshot = new(protocol.Snapshot)
	suite.sealedSnapshot = new(protocol.Snapshot)
	suite.sporkID = unittest.IdentifierFixture()
	suite.protocolStateVersion = unittest.Uint64InRange(10, 30)

	suite.rootBlock = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0))
	suite.sealedBlock = suite.rootBlock
	suite.finalizedBlock = unittest.BlockHeaderWithParentFixture(suite.sealedBlock)

	suite.epochQuery = new(protocol.EpochQuery)
	suite.state.On("Sealed").Return(suite.sealedSnapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.finalSnapshot, nil).Maybe()
	suite.finalSnapshot.On("Epochs").Return(suite.epochQuery).Maybe()
	suite.sealedSnapshot.On("Head").Return(
		func() *flow.Header {
			return suite.sealedBlock
		},
		nil,
	).Maybe()
	suite.finalSnapshot.On("Head").Return(
		func() *flow.Header {
			return suite.finalizedBlock
		},
		nil,
	).Maybe()

	pstate := protocol.NewKVStoreReader(suite.T())
	pstate.On("GetProtocolStateVersion").Return(suite.protocolStateVersion, nil).Maybe()
	suite.finalSnapshot.On("ProtocolState").Return(pstate, nil).Maybe()

	suite.params = new(protocol.Params)
	suite.params.On("FinalizedRoot").Return(suite.rootBlock, nil)
	suite.params.On("SporkID").Return(suite.sporkID, nil)
	suite.params.On("SporkRootBlockHeight").Return(suite.rootBlock.Height, nil)
	suite.params.On("SealedRoot").Return(suite.rootBlock, nil)
	suite.state.On("Params").Return(suite.params).Maybe()
	suite.collClient = new(accessmock.AccessAPIClient)
	suite.execClient = new(accessmock.ExecutionAPIClient)

	suite.request = new(mockmodule.Requester)
	suite.request.On("EntityByID", mock.Anything, mock.Anything)

	suite.me = new(mockmodule.Local)

	suite.signerIds = unittest.IdentifierListFixture(4)
	suite.signerIndicesDecoder = new(hsmock.BlockSignerDecoder)
	suite.signerIndicesDecoder.On("DecodeSignerIDs", mock.Anything).Return(suite.signerIds, nil).Maybe()

	accessIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))
	suite.me.
		On("NodeID").
		Return(accessIdentity.NodeID)

	suite.chainID = flow.Testnet
	suite.metrics = metrics.NewNoopCollector()
	suite.finalizedHeaderCache = mocks.NewFinalizedHeaderCache(suite.T(), suite.state)
}

func (suite *Suite) RunTest(
	f func(handler *rpc.Handler, db storage.DB, all *store.All),
) {
	unittest.RunWithPebbleDB(suite.T(), func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		all := store.InitAll(metrics.NewNoopCollector(), db)

		var err error
		suite.backend, err = backend.New(backend.Params{
			State:                suite.state,
			CollectionRPC:        suite.collClient,
			Blocks:               all.Blocks,
			Headers:              all.Headers,
			Collections:          all.Collections,
			Transactions:         all.Transactions,
			ExecutionResults:     all.Results,
			ExecutionReceipts:    all.Receipts,
			ChainID:              suite.chainID,
			AccessMetrics:        suite.metrics,
			MaxHeightRange:       events.DefaultMaxHeightRange,
			Log:                  suite.log,
			SnapshotHistoryLimit: backend.DefaultSnapshotHistoryLimit,
			Communicator:         node_communicator.NewNodeCommunicator(false),
			EventQueryMode:       query_mode.IndexQueryModeExecutionNodesOnly,
			ScriptExecutionMode:  query_mode.IndexQueryModeExecutionNodesOnly,
			TxResultQueryMode:    query_mode.IndexQueryModeExecutionNodesOnly,
		})
		require.NoError(suite.T(), err)

		handler := rpc.NewHandler(
			suite.backend,
			suite.chainID.Chain(),
			suite.finalizedHeaderCache,
			suite.me,
			subscription.DefaultMaxGlobalStreams,
			rpc.WithBlockSignerDecoder(suite.signerIndicesDecoder),
		)
		f(handler, db, all)
	})
}

func (suite *Suite) TestSendAndGetTransaction() {
	suite.RunTest(func(handler *rpc.Handler, _ storage.DB, _ *store.All) {
		referenceBlock := unittest.BlockHeaderFixture()
		transaction := unittest.TransactionFixture(
			func(t *flow.Transaction) {
				t.ReferenceBlockID = referenceBlock.ID()
			})

		refSnapshot := new(protocol.Snapshot)

		suite.state.
			On("AtBlockID", referenceBlock.ID()).
			Return(refSnapshot, nil)

		refSnapshot.
			On("Head").
			Return(referenceBlock, nil).
			Twice()

		suite.finalSnapshot.
			On("Head").
			Return(referenceBlock, nil).
			Once()

		expected := convert.TransactionToMessage(transaction.TransactionBody)
		sendReq := &accessproto.SendTransactionRequest{
			Transaction: expected,
		}
		sendResp := accessproto.SendTransactionResponse{}

		suite.collClient.
			On("SendTransaction", mock.Anything, mock.Anything).
			Return(&sendResp, nil).
			Once()

		// Send transaction
		resp, err := handler.SendTransaction(context.Background(), sendReq)
		suite.Require().NoError(err)
		suite.Require().NotNil(resp)

		id := transaction.ID()
		getReq := &accessproto.GetTransactionRequest{
			Id: id[:],
		}

		// Get transaction
		gResp, err := handler.GetTransaction(context.Background(), getReq)
		suite.Require().NoError(err)
		suite.Require().NotNil(gResp)

		actual := gResp.GetTransaction()
		suite.Require().Equal(expected, actual)
	})
}

func (suite *Suite) TestSendExpiredTransaction() {
	suite.RunTest(func(handler *rpc.Handler, _ storage.DB, _ *store.All) {
		referenceBlock := suite.finalizedBlock

		transaction := unittest.TransactionFixture(
			func(t *flow.Transaction) {
				t.ReferenceBlockID = referenceBlock.ID()
			})
		// create latest block that is past the expiry window
		latestBlock := unittest.BlockHeaderFixture()
		latestBlock.Height = referenceBlock.Height + flow.DefaultTransactionExpiry*2

		refSnapshot := new(protocol.Snapshot)

		suite.state.
			On("AtBlockID", referenceBlock.ID()).
			Return(refSnapshot, nil)

		refSnapshot.
			On("Head").
			Return(referenceBlock, nil).
			Twice()

		// Advancing final state to expire ref block
		suite.finalizedBlock = latestBlock

		req := &accessproto.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(transaction.TransactionBody),
		}

		_, err := handler.SendTransaction(context.Background(), req)
		suite.Require().Error(err)
	})
}

// TestSendTransactionToRandomCollectionNode tests that collection nodes are chosen from the appropriate cluster when
// forwarding transactions by sending two transactions bound for two different collection clusters.
func (suite *Suite) TestSendTransactionToRandomCollectionNode() {
	unittest.RunWithPebbleDB(suite.T(), func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)

		// create a transaction
		referenceBlock := unittest.BlockHeaderFixture()
		transaction := unittest.TransactionFixture(
			func(t *flow.Transaction) {
				t.ReferenceBlockID = referenceBlock.ID()
			})

		// setup the state and finalSnapshot mock expectations
		suite.state.On("AtBlockID", referenceBlock.ID()).Return(suite.finalSnapshot, nil)
		suite.finalSnapshot.On("Head").Return(referenceBlock, nil)

		// create storage
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		// create collection node cluster
		count := 2
		collNodes := unittest.IdentityListFixture(count, unittest.WithRole(flow.RoleCollection)).ToSkeleton()
		assignments := unittest.ClusterAssignment(uint(count), collNodes)
		clusters, err := factory.NewClusterList(assignments, collNodes)
		suite.Require().Nil(err)
		collNode1 := clusters[0][0]
		collNode2 := clusters[1][0]
		epoch := new(protocol.CommittedEpoch)
		suite.epochQuery.On("Current").Return(epoch, nil)
		epoch.On("Clustering").Return(clusters, nil)

		// create two transactions bound for each of the cluster
		cluster1 := clusters[0]
		cluster1tx := unittest.AlterTransactionForCluster(transaction.TransactionBody, clusters, cluster1, func(transaction *flow.TransactionBody) {})
		tx1 := convert.TransactionToMessage(cluster1tx)
		sendReq1 := &accessproto.SendTransactionRequest{
			Transaction: tx1,
		}
		cluster2 := clusters[1]
		cluster2tx := unittest.AlterTransactionForCluster(transaction.TransactionBody, clusters, cluster2, func(transaction *flow.TransactionBody) {})
		tx2 := convert.TransactionToMessage(cluster2tx)
		sendReq2 := &accessproto.SendTransactionRequest{
			Transaction: tx2,
		}
		sendResp := accessproto.SendTransactionResponse{}

		// create mock access api clients for each of the collection node expecting the correct transaction once
		col1ApiClient := new(accessmock.AccessAPIClient)
		col1ApiClient.On("SendTransaction", mock.Anything, sendReq1).Return(&sendResp, nil).Once()
		col2ApiClient := new(accessmock.AccessAPIClient)
		col2ApiClient.On("SendTransaction", mock.Anything, sendReq2).Return(&sendResp, nil).Once()

		// create a mock connection factory
		connFactory := connectionmock.NewConnectionFactory(suite.T())
		connFactory.On("GetAccessAPIClient", collNode1.Address, nil).Return(col1ApiClient, &mocks.MockCloser{}, nil)
		connFactory.On("GetAccessAPIClient", collNode2.Address, nil).Return(col2ApiClient, &mocks.MockCloser{}, nil)

		bnd, err := backend.New(backend.Params{State: suite.state,
			Collections:          collections,
			Transactions:         transactions,
			ChainID:              suite.chainID,
			AccessMetrics:        metrics,
			ConnFactory:          connFactory,
			MaxHeightRange:       events.DefaultMaxHeightRange,
			Log:                  suite.log,
			SnapshotHistoryLimit: backend.DefaultSnapshotHistoryLimit,
			Communicator:         node_communicator.NewNodeCommunicator(false),
			EventQueryMode:       query_mode.IndexQueryModeExecutionNodesOnly,
			ScriptExecutionMode:  query_mode.IndexQueryModeExecutionNodesOnly,
			TxResultQueryMode:    query_mode.IndexQueryModeExecutionNodesOnly,
		})
		require.NoError(suite.T(), err)

		handler := rpc.NewHandler(bnd, suite.chainID.Chain(), suite.finalizedHeaderCache, suite.me, subscription.DefaultMaxGlobalStreams)

		// Send transaction 1
		resp, err := handler.SendTransaction(context.Background(), sendReq1)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)

		// Send transaction 2
		resp, err = handler.SendTransaction(context.Background(), sendReq2)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)

		// verify that a collection node in the correct cluster was contacted exactly once
		col1ApiClient.AssertExpectations(suite.T())
		col2ApiClient.AssertExpectations(suite.T())
		epoch.AssertNumberOfCalls(suite.T(), "Clustering", 2)

		// additionally do a GetTransaction request for the two transactions
		getTx := func(tx flow.TransactionBody) {
			id := tx.ID()
			getReq := &accessproto.GetTransactionRequest{
				Id: id[:],
			}
			gResp, err := handler.GetTransaction(context.Background(), getReq)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), gResp)
			actual := gResp.GetTransaction()
			expected := convert.TransactionToMessage(tx)
			require.Equal(suite.T(), expected, actual)
		}

		getTx(cluster1tx)
		getTx(cluster1tx)
	})
}

func (suite *Suite) TestGetBlockByIDAndHeight() {
	suite.RunTest(func(handler *rpc.Handler, db storage.DB, all *store.All) {

		// test block1 get by ID
		block1 := unittest.BlockFixture()
		proposal1 := unittest.ProposalFromBlock(block1)
		// test block2 get by height
		block2 := unittest.BlockFixture(
			unittest.Block.WithHeight(2),
		)
		proposal2 := unittest.ProposalFromBlock(block2)

		lctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			if err := all.Blocks.BatchStore(lctx, rw, proposal1); err != nil {
				return err
			}
			if err := all.Blocks.BatchStore(lctx, rw, proposal2); err != nil {
				return err
			}
			return nil
		}))
		lctx.Release()

		fctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), fctx.AcquireLock(storage.LockFinalizeBlock))
		// the follower logic should update height index on the block storage when a block is finalized
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(fctx, rw, block2.Height, block2.ID())
		}))
		fctx.Release()

		assertHeaderResp := func(
			resp *accessproto.BlockHeaderResponse,
			err error,
			header *flow.Header,
		) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := resp.Block
			expectedMessage, err := convert.BlockHeaderToMessage(header, suite.signerIds)
			require.NoError(suite.T(), err)
			require.Empty(suite.T(), cmp.Diff(expectedMessage, actual, protocmp.Transform()))
			expectedBlockHeader, err := convert.MessageToBlockHeader(actual)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), expectedBlockHeader, header)
		}

		assertBlockResp := func(
			resp *accessproto.BlockResponse,
			err error,
			block *flow.Block,
		) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := resp.Block
			expectedMessage, err := convert.BlockToMessage(block, suite.signerIds)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), expectedMessage, actual)
			expectedBlock, err := convert.MessageToBlock(resp.Block)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), expectedBlock.ID(), block.ID())
		}

		assertLightBlockResp := func(
			resp *accessproto.BlockResponse,
			err error,
			block *flow.Block,
		) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := resp.Block
			expectedMessage := convert.BlockToMessageLight(block)
			require.Equal(suite.T(), expectedMessage, actual)
		}

		suite.finalSnapshot.On("Head").Return(block1.ToHeader(), nil)
		suite.Run("get header 1 by ID", func() {
			// get header by ID
			id := block1.ID()
			req := &accessproto.GetBlockHeaderByIDRequest{
				Id: id[:],
			}

			resp, err := handler.GetBlockHeaderByID(context.Background(), req)

			// assert it is indeed block1
			assertHeaderResp(resp, err, block1.ToHeader())
		})

		suite.Run("get block 1 by ID", func() {
			id := block1.ID()
			// get block details by ID
			req := &accessproto.GetBlockByIDRequest{
				Id:                id[:],
				FullBlockResponse: true,
			}

			resp, err := handler.GetBlockByID(context.Background(), req)

			assertBlockResp(resp, err, block1)
		})

		suite.Run("get block light 1 by ID", func() {
			id := block1.ID()
			// get block details by ID
			req := &accessproto.GetBlockByIDRequest{
				Id: id[:],
			}

			resp, err := handler.GetBlockByID(context.Background(), req)

			assertLightBlockResp(resp, err, block1)
		})

		suite.Run("get header 2 by height", func() {

			// get header by height
			req := &accessproto.GetBlockHeaderByHeightRequest{
				Height: block2.Height,
			}

			resp, err := handler.GetBlockHeaderByHeight(context.Background(), req)

			assertHeaderResp(resp, err, block2.ToHeader())
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &accessproto.GetBlockByHeightRequest{
				Height:            block2.Height,
				FullBlockResponse: true,
			}

			resp, err := handler.GetBlockByHeight(context.Background(), req)

			assertBlockResp(resp, err, block2)
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &accessproto.GetBlockByHeightRequest{
				Height: block2.Height,
			}

			resp, err := handler.GetBlockByHeight(context.Background(), req)

			assertLightBlockResp(resp, err, block2)
		})
	})
}

func (suite *Suite) TestGetExecutionResultByBlockID() {
	suite.RunTest(func(handler *rpc.Handler, db storage.DB, all *store.All) {

		// test block1 get by ID
		nonexistingID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()

		er := unittest.ExecutionResultFixture(
			unittest.WithExecutionResultBlockID(blockID),
			unittest.WithServiceEvents(3))

		require.NoError(suite.T(), all.Results.Store(er))
		require.NoError(suite.T(), all.Results.Index(blockID, er.ID()))

		assertResp := func(
			resp *accessproto.ExecutionResultForBlockIDResponse,
			err error,
			executionResult *flow.ExecutionResult,
		) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			er := resp.ExecutionResult

			require.Len(suite.T(), er.Chunks, len(executionResult.Chunks))
			require.Len(suite.T(), er.ServiceEvents, len(executionResult.ServiceEvents))

			assert.Equal(suite.T(), executionResult.BlockID, convert.MessageToIdentifier(er.BlockId))
			assert.Equal(suite.T(), executionResult.PreviousResultID, convert.MessageToIdentifier(er.PreviousResultId))
			assert.Equal(suite.T(), executionResult.ExecutionDataID, convert.MessageToIdentifier(er.ExecutionDataId))

			for i, chunk := range executionResult.Chunks {
				assert.Equal(suite.T(), chunk.BlockID[:], er.Chunks[i].BlockId)
				assert.Equal(suite.T(), chunk.Index, er.Chunks[i].Index)
				assert.Equal(suite.T(), uint32(chunk.CollectionIndex), er.Chunks[i].CollectionIndex)
				assert.Equal(suite.T(), chunk.StartState[:], er.Chunks[i].StartState)
				assert.Equal(suite.T(), chunk.EventCollection[:], er.Chunks[i].EventCollection)
				assert.Equal(suite.T(), chunk.TotalComputationUsed, er.Chunks[i].TotalComputationUsed)
				assert.Equal(suite.T(), uint32(chunk.NumberOfTransactions), er.Chunks[i].NumberOfTransactions)
				assert.Equal(suite.T(), chunk.EndState[:], er.Chunks[i].EndState)
			}

			for i, serviceEvent := range executionResult.ServiceEvents {
				assert.Equal(suite.T(), serviceEvent.Type.String(), er.ServiceEvents[i].Type)
				event := serviceEvent.Event
				marshalledEvent, err := json.Marshal(event)
				require.NoError(suite.T(), err)
				assert.Equal(suite.T(), marshalledEvent, er.ServiceEvents[i].Payload)
			}
			parsedExecResult, err := convert.MessageToExecutionResult(resp.ExecutionResult)
			require.NoError(suite.T(), err)
			assert.Equal(suite.T(), parsedExecResult.ID(), executionResult.ID())
		}

		suite.Run("nonexisting block", func() {
			req := &accessproto.GetExecutionResultForBlockIDRequest{
				BlockId: nonexistingID[:],
			}

			resp, err := handler.GetExecutionResultForBlockID(context.Background(), req)

			require.Error(suite.T(), err)
			require.Nil(suite.T(), resp)
		})

		suite.Run("some block", func() {
			// get header by ID
			req := &accessproto.GetExecutionResultForBlockIDRequest{
				BlockId: blockID[:],
			}

			resp, err := handler.GetExecutionResultForBlockID(context.Background(), req)

			require.NoError(suite.T(), err)

			assertResp(resp, err, er)
		})

	})
}

// TestGetSealedTransaction tests that transactions status of transaction that belongs to a sealed block
// is reported as sealed
func (suite *Suite) TestGetSealedTransaction() {
	unittest.RunWithPebbleDB(suite.T(), func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		all := store.InitAll(metrics.NewNoopCollector(), db)
		enIdentities := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
		enNodeIDs := enIdentities.NodeIDs()

		// create block -> collection -> transactions
		proposal, collection := suite.createChain()
		block := proposal.Block

		// setup mocks
		conduit := new(mocknetwork.Conduit)
		suite.net.On("Register", channels.ReceiveReceipts, mock.Anything).Return(conduit, nil).
			Once()
		suite.request.On("Request", mock.Anything, mock.Anything).Return()

		colIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))
		allIdentities := append(colIdentities, enIdentities...)

		suite.finalSnapshot.On("Identities", mock.Anything).Return(allIdentities, nil).Once()

		exeEventResp := execproto.GetTransactionResultResponse{
			Events: nil,
		}

		// generate receipts
		executionReceipts := unittest.ReceiptsForBlockFixture(&block, enNodeIDs)

		// assume execution node returns an empty list of events
		suite.execClient.On("GetTransactionResult", mock.Anything, mock.Anything).Return(&exeEventResp, nil)

		// create a mock connection factory
		connFactory := connectionmock.NewConnectionFactory(suite.T())
		connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mocks.MockCloser{}, nil)

		// initialize storage
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		collectionsToMarkFinalized := stdmap.NewTimes(100)
		collectionsToMarkExecuted := stdmap.NewTimes(100)
		blocksToMarkExecuted := stdmap.NewTimes(100)
		blockTransactions := stdmap.NewIdentifierMap(100)

		execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
			suite.log,
			suite.state,
			all.Receipts,
			enNodeIDs,
			nil,
		)

		bnd, err := backend.New(backend.Params{
			State:                      suite.state,
			CollectionRPC:              suite.collClient,
			Blocks:                     all.Blocks,
			Headers:                    all.Headers,
			Collections:                collections,
			Transactions:               transactions,
			ExecutionReceipts:          all.Receipts,
			ExecutionResults:           all.Results,
			ChainID:                    suite.chainID,
			AccessMetrics:              suite.metrics,
			ConnFactory:                connFactory,
			MaxHeightRange:             events.DefaultMaxHeightRange,
			Log:                        suite.log,
			SnapshotHistoryLimit:       backend.DefaultSnapshotHistoryLimit,
			Communicator:               node_communicator.NewNodeCommunicator(false),
			TxResultQueryMode:          query_mode.IndexQueryModeExecutionNodesOnly,
			EventQueryMode:             query_mode.IndexQueryModeExecutionNodesOnly,
			ScriptExecutionMode:        query_mode.IndexQueryModeExecutionNodesOnly,
			ExecNodeIdentitiesProvider: execNodeIdentitiesProvider,
		})
		require.NoError(suite.T(), err)

		handler := rpc.NewHandler(bnd, suite.chainID.Chain(), suite.finalizedHeaderCache, suite.me, subscription.DefaultMaxGlobalStreams)

		collectionExecutedMetric, err := indexer.NewCollectionExecutedMetricImpl(
			suite.log,
			metrics,
			collectionsToMarkFinalized,
			collectionsToMarkExecuted,
			blocksToMarkExecuted,
			collections,
			all.Blocks,
			blockTransactions,
		)
		require.NoError(suite.T(), err)

		progress, err := store.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight).Initialize(suite.rootBlock.Height)
		require.NoError(suite.T(), err)
		lastFullBlockHeight, err := counters.NewPersistentStrictMonotonicCounter(progress)
		require.NoError(suite.T(), err)

		// create the ingest engine
		processedHeight := store.NewConsumerProgress(db, module.ConsumeProgressIngestionEngineBlockHeight)

		collectionSyncer := ingestion.NewCollectionSyncer(
			suite.log,
			module.CollectionExecutedMetric(collectionExecutedMetric),
			suite.request,
			suite.state,
			all.Blocks,
			collections,
			transactions,
			lastFullBlockHeight,
			suite.lockManager,
		)

		ingestEng, err := ingestion.New(
			suite.log,
			suite.net,
			suite.state,
			suite.me,
			all.Blocks,
			all.Results,
			all.Receipts,
			processedHeight,
			collectionSyncer,
			collectionExecutedMetric,
			nil,
		)
		require.NoError(suite.T(), err)

		// 1. Assume that follower engine updated the block storage and the protocol state. The block is reported as sealed
		lctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return all.Blocks.BatchStore(lctx, rw, proposal)
		}))
		lctx.Release()

		fctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), fctx.AcquireLock(storage.LockFinalizeBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(fctx, rw, block.Height, block.ID())
		}))
		fctx.Release()

		suite.sealedBlock = block.ToHeader()

		background, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctx := irrecoverable.NewMockSignalerContext(suite.T(), background)
		ingestEng.Start(ctx)
		<-ingestEng.Ready()

		// 2. Ingest engine was notified by the follower engine about a new block.
		// Follower engine --> Ingest engine
		mb := &model.Block{
			BlockID: block.ID(),
		}
		ingestEng.OnFinalizedBlock(mb)

		// 3. Request engine is used to request missing collection
		suite.request.On("EntityByID", collection.ID(), mock.Anything).Return()
		// 4. Indexer IndexCollection receives the requested collection and all the execution receipts
		// Create a lock context for indexing
		indexLctx := suite.lockManager.NewContext()
		lockErr := indexLctx.AcquireLock(storage.LockInsertCollection)
		require.NoError(suite.T(), lockErr)
		defer indexLctx.Release()

		err = indexer.IndexCollection(indexLctx, collection, collections, suite.log, module.CollectionExecutedMetric(collectionExecutedMetric))
		require.NoError(suite.T(), err)

		for _, r := range executionReceipts {
			err = ingestEng.Process(channels.ReceiveReceipts, enNodeIDs[0], r)
			require.NoError(suite.T(), err)
		}

		// 5. Client requests a transaction
		tx := collection.Transactions[0]
		txID := tx.ID()
		getReq := &accessproto.GetTransactionRequest{
			Id: txID[:],
		}
		gResp, err := handler.GetTransactionResult(context.Background(), getReq)
		require.NoError(suite.T(), err)
		// assert that the transaction is reported as Sealed
		require.Equal(suite.T(), entitiesproto.TransactionStatus_SEALED, gResp.GetStatus())
	})
}

// TestGetTransactionResult tests different approaches to using the GetTransactionResult query, including using
// transaction ID, block ID, and collection ID.
func (suite *Suite) TestGetTransactionResult() {
	unittest.RunWithPebbleDB(suite.T(), func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		all := store.InitAll(metrics.NewNoopCollector(), db)
		originID := unittest.IdentifierFixture()

		*suite.state = protocol.State{}

		// create block -> collection -> transactions
		proposal, collection := suite.createChain()
		block := proposal.Block
		proposalNegative, collectionNegative := suite.createChain()
		blockNegative := proposalNegative.Block
		blockId := block.ID()
		blockNegativeId := blockNegative.ID()

		finalSnapshot := new(protocol.Snapshot)
		finalSnapshot.On("Head").Return(suite.finalizedBlock, nil)

		suite.state.On("Params").Return(suite.params)
		suite.state.On("Final").Return(finalSnapshot)
		suite.state.On("Sealed").Return(suite.sealedSnapshot)
		sealedBlock := unittest.Block.Genesis(flow.Emulator).ToHeader()
		// specifically for this test we will consider that sealed block is far behind finalized, so we get EXECUTED status
		suite.sealedSnapshot.On("Head").Return(sealedBlock, nil)

		lctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return all.Blocks.BatchStore(lctx, rw, proposal)
		}))
		lctx.Release()

		lctx2 := suite.lockManager.NewContext()
		require.NoError(suite.T(), lctx2.AcquireLock(storage.LockInsertBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return all.Blocks.BatchStore(lctx2, rw, proposalNegative)
		}))
		lctx2.Release()

		suite.state.On("AtBlockID", blockId).Return(suite.sealedSnapshot)

		colIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))
		enIdentities := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

		enNodeIDs := enIdentities.NodeIDs()
		allIdentities := append(colIdentities, enIdentities...)
		finalSnapshot.On("Identities", mock.Anything).Return(allIdentities, nil)

		suite.state.On("AtBlockID", blockNegativeId).Return(suite.sealedSnapshot)
		suite.sealedSnapshot.On("Identities", mock.Anything).Return(allIdentities, nil)

		// assume execution node returns an empty list of events
		suite.execClient.On("GetTransactionResult", mock.Anything, mock.Anything).Return(&execproto.GetTransactionResultResponse{
			Events: nil,
		}, nil)

		// setup mocks
		conduit := new(mocknetwork.Conduit)
		suite.net.On("Register", channels.ReceiveReceipts, mock.Anything).Return(conduit, nil).Once()
		suite.request.On("Request", mock.Anything, mock.Anything).Return()

		// create a mock connection factory
		connFactory := connectionmock.NewConnectionFactory(suite.T())
		connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mocks.MockCloser{}, nil)

		// initialize storage
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		_, err := collections.Store(collectionNegative)
		require.NoError(suite.T(), err)
		collectionsToMarkFinalized := stdmap.NewTimes(100)
		collectionsToMarkExecuted := stdmap.NewTimes(100)
		blocksToMarkExecuted := stdmap.NewTimes(100)
		blockTransactions := stdmap.NewIdentifierMap(100)

		execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
			suite.log,
			suite.state,
			all.Receipts,
			enNodeIDs,
			nil,
		)

		bnd, err := backend.New(backend.Params{State: suite.state,
			CollectionRPC:              suite.collClient,
			Blocks:                     all.Blocks,
			Headers:                    all.Headers,
			Collections:                collections,
			Transactions:               transactions,
			ExecutionReceipts:          all.Receipts,
			ExecutionResults:           all.Results,
			ChainID:                    suite.chainID,
			AccessMetrics:              suite.metrics,
			ConnFactory:                connFactory,
			MaxHeightRange:             events.DefaultMaxHeightRange,
			Log:                        suite.log,
			SnapshotHistoryLimit:       backend.DefaultSnapshotHistoryLimit,
			Communicator:               node_communicator.NewNodeCommunicator(false),
			TxResultQueryMode:          query_mode.IndexQueryModeExecutionNodesOnly,
			EventQueryMode:             query_mode.IndexQueryModeExecutionNodesOnly,
			ScriptExecutionMode:        query_mode.IndexQueryModeExecutionNodesOnly,
			ExecNodeIdentitiesProvider: execNodeIdentitiesProvider,
		})
		require.NoError(suite.T(), err)

		handler := rpc.NewHandler(bnd, suite.chainID.Chain(), suite.finalizedHeaderCache, suite.me, subscription.DefaultMaxGlobalStreams)

		collectionExecutedMetric, err := indexer.NewCollectionExecutedMetricImpl(
			suite.log,
			metrics,
			collectionsToMarkFinalized,
			collectionsToMarkExecuted,
			blocksToMarkExecuted,
			collections,
			all.Blocks,
			blockTransactions,
		)
		require.NoError(suite.T(), err)

		processedHeightInitializer := store.NewConsumerProgress(db, module.ConsumeProgressIngestionEngineBlockHeight)

		lastFullBlockHeightProgress, err := store.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight).
			Initialize(suite.rootBlock.Height)
		require.NoError(suite.T(), err)

		lastFullBlockHeight, err := counters.NewPersistentStrictMonotonicCounter(lastFullBlockHeightProgress)
		require.NoError(suite.T(), err)

		collectionSyncer := ingestion.NewCollectionSyncer(
			suite.log,
			module.CollectionExecutedMetric(collectionExecutedMetric),
			suite.request,
			suite.state,
			all.Blocks,
			collections,
			transactions,
			lastFullBlockHeight,
			suite.lockManager,
		)

		ingestEng, err := ingestion.New(
			suite.log,
			suite.net,
			suite.state,
			suite.me,
			all.Blocks,
			all.Results,
			all.Receipts,
			processedHeightInitializer,
			collectionSyncer,
			collectionExecutedMetric,
			nil,
		)
		require.NoError(suite.T(), err)

		background, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctx := irrecoverable.NewMockSignalerContext(suite.T(), background)
		ingestEng.Start(ctx)
		<-ingestEng.Ready()

		processExecutionReceipts := func(
			block *flow.Block,
			collection *flow.Collection,
			enNodeIDs flow.IdentifierList,
			originID flow.Identifier,
			ingestEng *ingestion.Engine,
		) {
			executionReceipts := unittest.ReceiptsForBlockFixture(block, enNodeIDs)
			// Ingest engine was notified by the follower engine about a new block.
			// Follower engine --> Ingest engine
			mb := &model.Block{
				BlockID: block.ID(),
			}
			ingestEng.OnFinalizedBlock(mb)

			// Indexer IndexCollection receives the requested collection and all the execution receipts
			// Create a lock context for indexing
			indexLctx := suite.lockManager.NewContext()
			lockErr := indexLctx.AcquireLock(storage.LockInsertCollection)
			require.NoError(suite.T(), lockErr)
			defer indexLctx.Release()

			err = indexer.IndexCollection(indexLctx, collection, collections, suite.log, module.CollectionExecutedMetric(collectionExecutedMetric))
			require.NoError(suite.T(), err)

			for _, r := range executionReceipts {
				err = ingestEng.Process(channels.ReceiveReceipts, enNodeIDs[0], r)
				require.NoError(suite.T(), err)
			}
		}
		fctx2 := suite.lockManager.NewContext()
		require.NoError(suite.T(), fctx2.AcquireLock(storage.LockFinalizeBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(fctx2, rw, block.Height, block.ID())
		}))
		fctx2.Release()
		finalSnapshot.On("Head").Return(block.ToHeader(), nil)

		processExecutionReceipts(&block, collection, enNodeIDs, originID, ingestEng)
		processExecutionReceipts(&blockNegative, collectionNegative, enNodeIDs, originID, ingestEng)

		txId := collection.Transactions[0].ID()
		collectionId := collection.ID()
		txIdNegative := collectionNegative.Transactions[0].ID()
		collectionIdNegative := collectionNegative.ID()

		assertTransactionResult := func(
			resp *accessproto.TransactionResultResponse,
			err error,
		) {
			require.NoError(suite.T(), err)
			actualTxId := flow.HashToID(resp.TransactionId)
			require.Equal(suite.T(), txId, actualTxId)
			actualBlockId := flow.HashToID(resp.BlockId)
			require.Equal(suite.T(), blockId, actualBlockId)
			actualCollectionId := flow.HashToID(resp.CollectionId)
			require.Equal(suite.T(), collectionId, actualCollectionId)
		}

		// Test behaviour with transactionId provided
		// POSITIVE
		suite.Run("Get transaction result by transaction ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id: txId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			assertTransactionResult(resp, err)
		})

		// Test behaviour with blockId provided
		suite.Run("Get transaction result by block ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:      txId[:],
				BlockId: blockId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			assertTransactionResult(resp, err)
		})

		suite.Run("Get transaction result with wrong transaction ID and correct block ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:      txIdNegative[:],
				BlockId: blockId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			require.Error(suite.T(), err)
			require.Nil(suite.T(), resp)
		})

		suite.Run("Get transaction result with wrong block ID and correct transaction ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:      txId[:],
				BlockId: blockNegativeId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			require.Error(suite.T(), err)
			require.Contains(suite.T(), err.Error(), "failed to find: transaction not in block")
			require.Nil(suite.T(), resp)
		})

		// Test behaviour with collectionId provided
		suite.Run("Get transaction result by collection ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:           txId[:],
				CollectionId: collectionId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			assertTransactionResult(resp, err)
		})

		suite.Run("Get transaction result with wrong collection ID but correct transaction ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:           txId[:],
				CollectionId: collectionIdNegative[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			require.Error(suite.T(), err)
			require.Nil(suite.T(), resp)
		})

		suite.Run("Get transaction result with wrong transaction ID and correct collection ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:           txIdNegative[:],
				CollectionId: collectionId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			require.Error(suite.T(), err)
			require.Nil(suite.T(), resp)
		})

		// Test behaviour with blockId and collectionId provided
		suite.Run("Get transaction result by block ID and collection ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:           txId[:],
				BlockId:      blockId[:],
				CollectionId: collectionId[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			assertTransactionResult(resp, err)
		})

		suite.Run("Get transaction result by block ID with wrong collection ID", func() {
			getReq := &accessproto.GetTransactionRequest{
				Id:           txId[:],
				BlockId:      blockId[:],
				CollectionId: collectionIdNegative[:],
			}
			resp, err := handler.GetTransactionResult(context.Background(), getReq)
			require.Error(suite.T(), err)
			require.Nil(suite.T(), resp)
		})
	})
}

// TestExecuteScript tests the three execute Script related calls to make sure that the execution api is called with
// the correct block id
func (suite *Suite) TestExecuteScript() {
	unittest.RunWithPebbleDB(suite.T(), func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		all := store.InitAll(metrics.NewNoopCollector(), db)
		identities := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
		suite.sealedSnapshot.On("Identities", mock.Anything).Return(identities, nil)
		suite.finalSnapshot.On("Identities", mock.Anything).Return(identities, nil)

		// create a mock connection factory
		connFactory := connectionmock.NewConnectionFactory(suite.T())
		connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mocks.MockCloser{}, nil)

		execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
			suite.log,
			suite.state,
			all.Receipts,
			nil,
			identities.NodeIDs(),
		)

		var err error
		suite.backend, err = backend.New(backend.Params{
			State:                      suite.state,
			CollectionRPC:              suite.collClient,
			Blocks:                     all.Blocks,
			Headers:                    all.Headers,
			Collections:                all.Collections,
			Transactions:               all.Transactions,
			ExecutionReceipts:          all.Receipts,
			ExecutionResults:           all.Results,
			ChainID:                    suite.chainID,
			AccessMetrics:              suite.metrics,
			ConnFactory:                connFactory,
			MaxHeightRange:             events.DefaultMaxHeightRange,
			Log:                        suite.log,
			SnapshotHistoryLimit:       backend.DefaultSnapshotHistoryLimit,
			Communicator:               node_communicator.NewNodeCommunicator(false),
			EventQueryMode:             query_mode.IndexQueryModeExecutionNodesOnly,
			ScriptExecutionMode:        query_mode.IndexQueryModeExecutionNodesOnly,
			TxResultQueryMode:          query_mode.IndexQueryModeExecutionNodesOnly,
			ExecNodeIdentitiesProvider: execNodeIdentitiesProvider,
		})
		require.NoError(suite.T(), err)

		handler := rpc.NewHandler(suite.backend, suite.chainID.Chain(), suite.finalizedHeaderCache, suite.me, subscription.DefaultMaxGlobalStreams)

		// initialize metrics related storage
		metrics := metrics.NewNoopCollector()
		collectionsToMarkFinalized := stdmap.NewTimes(100)
		collectionsToMarkExecuted := stdmap.NewTimes(100)
		blocksToMarkExecuted := stdmap.NewTimes(100)
		blockTransactions := stdmap.NewIdentifierMap(100)

		collectionExecutedMetric, err := indexer.NewCollectionExecutedMetricImpl(
			suite.log,
			metrics,
			collectionsToMarkFinalized,
			collectionsToMarkExecuted,
			blocksToMarkExecuted,
			all.Collections,
			all.Blocks,
			blockTransactions,
		)
		require.NoError(suite.T(), err)

		conduit := new(mocknetwork.Conduit)
		suite.net.On("Register", channels.ReceiveReceipts, mock.Anything).Return(conduit, nil).
			Once()

		processedHeightInitializer := store.NewConsumerProgress(db, module.ConsumeProgressIngestionEngineBlockHeight)

		lastFullBlockHeightInitializer := store.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight)
		lastFullBlockHeightProgress, err := lastFullBlockHeightInitializer.Initialize(suite.rootBlock.Height)
		require.NoError(suite.T(), err)

		lastFullBlockHeight, err := counters.NewPersistentStrictMonotonicCounter(lastFullBlockHeightProgress)
		require.NoError(suite.T(), err)

		collectionSyncer := ingestion.NewCollectionSyncer(
			suite.log,
			module.CollectionExecutedMetric(collectionExecutedMetric),
			suite.request,
			suite.state,
			all.Blocks,
			all.Collections,
			all.Transactions,
			lastFullBlockHeight,
			suite.lockManager,
		)

		ingestEng, err := ingestion.New(
			suite.log,
			suite.net,
			suite.state,
			suite.me,
			all.Blocks,
			all.Results,
			all.Receipts,
			processedHeightInitializer,
			collectionSyncer,
			collectionExecutedMetric,
			nil,
		)
		require.NoError(suite.T(), err)

		// create another block as a predecessor of the block created earlier
		prevBlock := unittest.BlockWithParentFixture(suite.finalizedBlock)

		// create a block and a seal pointing to that block
		lastBlock := unittest.BlockWithParentFixture(prevBlock.ToHeader())
		lctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), lctx.AcquireLock(storage.LockInsertBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return all.Blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(lastBlock))
		}))
		lctx.Release()
		require.NoError(suite.T(), err)

		fctx := suite.lockManager.NewContext()
		require.NoError(suite.T(), fctx.AcquireLock(storage.LockFinalizeBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(fctx, rw, lastBlock.Height, lastBlock.ID())
		}))
		fctx.Release()
		require.NoError(suite.T(), err)
		// update latest sealed block
		suite.sealedBlock = lastBlock.ToHeader()
		// create execution receipts for each of the execution node and the last block
		executionReceipts := unittest.ReceiptsForBlockFixture(lastBlock, identities.NodeIDs())
		// notify the ingest engine about the receipts
		for _, r := range executionReceipts {
			err = ingestEng.Process(channels.ReceiveReceipts, unittest.IdentifierFixture(), r)
			require.NoError(suite.T(), err)
		}

		fctx2 := suite.lockManager.NewContext()
		require.NoError(suite.T(), fctx2.AcquireLock(storage.LockInsertBlock))
		require.NoError(suite.T(), fctx2.AcquireLock(storage.LockFinalizeBlock))
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := all.Blocks.BatchStore(fctx2, rw, unittest.ProposalFromBlock(prevBlock))
			if err != nil {
				return err
			}

			return operation.IndexFinalizedBlockByHeight(fctx2, rw, prevBlock.Height, prevBlock.ID())
		}))
		fctx2.Release()

		// create execution receipts for each of the execution node and the previous block
		executionReceipts = unittest.ReceiptsForBlockFixture(prevBlock, identities.NodeIDs())
		// notify the ingest engine about the receipts
		for _, r := range executionReceipts {
			err = ingestEng.Process(channels.ReceiveReceipts, unittest.IdentifierFixture(), r)
			require.NoError(suite.T(), err)
		}

		ctx := context.Background()

		script := []byte("dummy script")

		// setupExecClientMock sets up the mock the execution client and returns the access response to expect
		setupExecClientMock := func(blockID flow.Identifier) *accessproto.ExecuteScriptResponse {
			id := blockID[:]
			executionReq := execproto.ExecuteScriptAtBlockIDRequest{
				BlockId: id,
				Script:  script,
			}
			executionResp := execproto.ExecuteScriptAtBlockIDResponse{
				Value: []byte{9, 10, 11},
			}

			suite.execClient.On("ExecuteScriptAtBlockID", ctx, &executionReq).Return(&executionResp, nil).Once()

			finalizedHeader := suite.finalizedHeaderCache.Get()
			finalizedHeaderId := finalizedHeader.ID()
			nodeId := suite.me.NodeID()

			expectedResp := accessproto.ExecuteScriptResponse{
				Value: executionResp.GetValue(),
				Metadata: &entitiesproto.Metadata{
					LatestFinalizedBlockId: finalizedHeaderId[:],
					LatestFinalizedHeight:  finalizedHeader.Height,
					NodeId:                 nodeId[:],
				},
			}
			return &expectedResp
		}

		assertResult := func(err error, expected interface{}, actual interface{}) {
			suite.Require().NoError(err)
			suite.Require().Equal(expected, actual)
			suite.execClient.AssertExpectations(suite.T())
		}

		suite.Run("execute script at latest block", func() {
			suite.state.
				On("AtBlockID", lastBlock.ID()).
				Return(suite.sealedSnapshot, nil)

			expectedResp := setupExecClientMock(lastBlock.ID())
			req := accessproto.ExecuteScriptAtLatestBlockRequest{
				Script: script,
			}
			actualResp, err := handler.ExecuteScriptAtLatestBlock(ctx, &req)
			assertResult(err, expectedResp, actualResp)
		})

		suite.Run("execute script at block id", func() {
			suite.state.
				On("AtBlockID", prevBlock.ID()).
				Return(suite.sealedSnapshot, nil)

			expectedResp := setupExecClientMock(prevBlock.ID())
			id := prevBlock.ID()
			req := accessproto.ExecuteScriptAtBlockIDRequest{
				BlockId: id[:],
				Script:  script,
			}
			actualResp, err := handler.ExecuteScriptAtBlockID(ctx, &req)
			assertResult(err, expectedResp, actualResp)
		})

		suite.Run("execute script at block height", func() {
			suite.state.
				On("AtBlockID", prevBlock.ID()).
				Return(suite.sealedSnapshot, nil)

			expectedResp := setupExecClientMock(prevBlock.ID())
			req := accessproto.ExecuteScriptAtBlockHeightRequest{
				BlockHeight: prevBlock.Height,
				Script:      script,
			}
			actualResp, err := handler.ExecuteScriptAtBlockHeight(ctx, &req)
			assertResult(err, expectedResp, actualResp)
		})
	})
}

// TestAPICallNodeVersionInfo tests the GetNodeVersionInfo query and check response returns correct node version
// information
func (suite *Suite) TestAPICallNodeVersionInfo() {
	suite.RunTest(func(handler *rpc.Handler, db storage.DB, all *store.All) {
		req := &accessproto.GetNodeVersionInfoRequest{}
		resp, err := handler.GetNodeVersionInfo(context.Background(), req)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)

		respNodeVersionInfo := resp.Info
		suite.Require().Equal(respNodeVersionInfo, &entitiesproto.NodeVersionInfo{
			Semver:               build.Version(),
			Commit:               build.Commit(),
			SporkId:              suite.sporkID[:],
			ProtocolVersion:      0,
			ProtocolStateVersion: uint64(suite.protocolStateVersion),
		})
	})
}

// TestLastFinalizedBlockHeightResult tests on example of the GetBlockHeaderByID function that the LastFinalizedBlock
// field in the response matches the finalized header from cache. It also tests that the LastFinalizedBlock field is
// updated correctly when a block with a greater height is finalized.
func (suite *Suite) TestLastFinalizedBlockHeightResult() {
	suite.RunTest(func(handler *rpc.Handler, db storage.DB, all *store.All) {
		block := unittest.BlockWithParentFixture(suite.finalizedBlock)
		proposal := unittest.ProposalFromBlock(block)
		newFinalizedBlock := unittest.BlockWithParentFixture(block.ToHeader())

		lctx := suite.lockManager.NewContext()
		err := lctx.AcquireLock(storage.LockInsertBlock)
		require.NoError(suite.T(), err)

		// store new block
		require.NoError(suite.T(), db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return all.Blocks.BatchStore(lctx, rw, proposal)
		}))
		lctx.Release()

		assertFinalizedBlockHeader := func(resp *accessproto.BlockHeaderResponse, err error) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)

			finalizedHeaderId := suite.finalizedBlock.ID()
			nodeId := suite.me.NodeID()

			require.Equal(suite.T(), &entitiesproto.Metadata{
				LatestFinalizedBlockId: finalizedHeaderId[:],
				LatestFinalizedHeight:  suite.finalizedBlock.Height,
				NodeId:                 nodeId[:],
			}, resp.Metadata)
		}

		id := block.ID()
		req := &accessproto.GetBlockHeaderByIDRequest{
			Id: id[:],
		}

		resp, err := handler.GetBlockHeaderByID(context.Background(), req)
		assertFinalizedBlockHeader(resp, err)

		suite.finalizedBlock = newFinalizedBlock.ToHeader()

		resp, err = handler.GetBlockHeaderByID(context.Background(), req)
		assertFinalizedBlockHeader(resp, err)
	})
}

func (suite *Suite) createChain() (*flow.Proposal, *flow.Collection) {
	collection := unittest.CollectionFixture(10)
	refBlockID := unittest.IdentifierFixture()
	// prepare cluster committee members
	clusterCommittee := unittest.IdentityListFixture(32 * 4).Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	// guarantee signers must be cluster committee members, so that access will fetch collection from
	// the signers that are specified by guarantee.SignerIndices
	indices, err := signature.EncodeSignersToIndices(clusterCommittee.NodeIDs(), clusterCommittee.NodeIDs())
	require.NoError(suite.T(), err)
	guarantee := &flow.CollectionGuarantee{
		CollectionID:     collection.ID(),
		Signature:        crypto.Signature([]byte("signature A")),
		ReferenceBlockID: refBlockID,
		SignerIndices:    indices,
	}
	block := unittest.BlockWithParentAndPayload(
		suite.finalizedBlock,
		unittest.PayloadFixture(unittest.WithGuarantees(guarantee)),
	)
	proposal := unittest.ProposalFromBlock(block)

	cluster := new(protocol.Cluster)
	cluster.On("Members").Return(clusterCommittee.ToSkeleton(), nil)
	epoch := new(protocol.CommittedEpoch)
	epoch.On("ClusterByChainID", mock.Anything).Return(cluster, nil)
	epochs := new(protocol.EpochQuery)
	epochs.On("Current").Return(epoch, nil)
	snap := new(protocol.Snapshot)
	snap.On("Epochs").Return(epochs).Maybe()
	snap.On("Params").Return(suite.params).Maybe()
	snap.On("Head").Return(block.ToHeader(), nil).Maybe()

	suite.state.On("AtBlockID", refBlockID).Return(snap)

	return proposal, &collection
}
