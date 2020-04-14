package access

import (
	"context"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"
	"github.com/dapperlabs/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/access/ingestion"
	accessmock "github.com/dapperlabs/flow-go/engine/access/mock"
	"github.com/dapperlabs/flow-go/engine/access/rpc/handler"
	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	networkmock "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	state              *protocol.State
	snapshot           *protocol.Snapshot
	log                zerolog.Logger
	net                *mockmodule.Network
	collClient         *accessmock.AccessAPIClient
	execClient         *accessmock.ExecutionAPIClient
	collectionsConduit *networkmock.Conduit
	me                 *mockmodule.Local
}

// TestAccess tests scenarios which exercise multiple API calls using both the RPC handler and the ingest engine
// and using a real badger storage.
func TestAccess(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.collClient = new(accessmock.AccessAPIClient)
	suite.execClient = new(accessmock.ExecutionAPIClient)
	suite.net = new(mockmodule.Network)
	suite.collectionsConduit = &networkmock.Conduit{}
	suite.me = new(mockmodule.Local)
	obsIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))
	suite.me.On("NodeID").Return(obsIdentity.NodeID)
}

func (suite *Suite) TestSendAndGetTransaction() {

	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		transaction := unittest.TransactionFixture(func(t *flow.Transaction) {
			t.Nonce = 0
			t.ComputeLimit = 0
		})

		// create storage
		collections := bstorage.NewCollections(db)
		transactions := bstorage.NewTransactions(db)
		handler := handler.NewHandler(suite.log, suite.state, nil, suite.collClient, nil, nil, collections, transactions)

		expected := convert.TransactionToMessage(transaction.TransactionBody)
		sendReq := &access.SendTransactionRequest{
			Transaction: expected,
		}
		sendResp := access.SendTransactionResponse{}
		suite.collClient.On("SendTransaction", mock.Anything, mock.Anything).Return(&sendResp, nil).Once()

		// Send transaction
		resp, err := handler.SendTransaction(context.Background(), sendReq)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)

		id := transaction.ID()
		getReq := &access.GetTransactionRequest{
			Id: id[:],
		}

		// Get transaction
		gResp, err := handler.GetTransaction(context.Background(), getReq)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), gResp)

		actual := gResp.Transaction

		// the transaction should be reported as pending
		expected.Status = entities.TransactionStatus_STATUS_PENDING
		require.Equal(suite.T(), expected, actual)
	})
}

func (suite *Suite) TestGetBlockByIDAndHeight() {

	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		// test block1 get by ID
		block1 := unittest.BlockFixture()
		// test block2 get by height
		block2 := unittest.BlockFixture()
		block2.Height = 2

		blocks := bstorage.NewBlocks(db)
		headers := bstorage.NewHeaders(db)
		require.NoError(suite.T(), blocks.Store(&block1))
		require.NoError(suite.T(), blocks.Store(&block2))

		// the follower logic should update height index on the block storage when a block is finalized
		err := db.Update(operation.InsertNumber(block2.Height, block2.ID()))
		require.NoError(suite.T(), err)

		handler := handler.NewHandler(suite.log, suite.state, nil, suite.collClient, blocks, headers, nil, nil)

		assertHeaderResp := func(resp *access.BlockHeaderResponse, err error, header *flow.Header) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := *resp.Block
			expected, _ := convert.BlockHeaderToMessage(header)
			require.Equal(suite.T(), expected, actual)
		}

		assertBlockResp := func(resp *access.BlockResponse, err error, block *flow.Block) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := resp.Block
			expected, _ := convert.BlockToMessage(block)
			require.Equal(suite.T(), expected, actual)
		}

		suite.Run("get header 1 by ID", func() {
			// get header by ID
			id := block1.ID()
			req := &access.GetBlockHeaderByIDRequest{
				Id: id[:],
			}

			resp, err := handler.GetBlockHeaderByID(context.Background(), req)

			// assert it is indeed block1
			assertHeaderResp(resp, err, &block1.Header)
		})

		suite.Run("get block 1 by ID", func() {
			id := block1.ID()
			// get block details by ID
			req := &access.GetBlockByIDRequest{
				Id: id[:],
			}

			resp, err := handler.GetBlockByID(context.Background(), req)

			assertBlockResp(resp, err, &block1)
		})

		suite.Run("get header 2 by height", func() {

			// get header by height
			req := &access.GetBlockHeaderByHeightRequest{
				Height: block2.Height,
			}

			resp, err := handler.GetBlockHeaderByHeight(context.Background(), req)

			assertHeaderResp(resp, err, &block2.Header)
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &access.GetBlockByHeightRequest{
				Height: block2.Height,
			}

			resp, err := handler.GetBlockByHeight(context.Background(), req)

			assertBlockResp(resp, err, &block2)
		})
	})
}

// TestGetSealedTransaction tests that transactions status of transaction that belongs to a sealed blocked
// is reported as sealed
func (suite *Suite) TestGetSealedTransaction() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		// create block -> collection -> transactions and seal
		block, collection, seal := suite.createChain()

		// setup mocks
		originID := unittest.IdentifierFixture()
		suite.net.On("Register", uint8(engine.CollectionProvider), mock.Anything).
			Return(suite.collectionsConduit, nil).
			Once()
		colIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))
		suite.snapshot.On("Identities", mock.Anything).Return(colIdentities, nil).Once()
		suite.collectionsConduit.On("Submit", mock.Anything, mock.Anything).Return(nil).Times(len(block.Guarantees))
		metrics := &mockmodule.Metrics{}

		// initialize storage
		blocks := bstorage.NewBlocks(db)
		headers := bstorage.NewHeaders(db)
		collections := bstorage.NewCollections(db)
		transactions := bstorage.NewTransactions(db)

		// create the ingest engine
		ingestEng, err := ingestion.New(suite.log, suite.net, suite.state, metrics, suite.me, blocks, headers, collections, transactions)
		require.NoError(suite.T(), err)

		// create the handler (called by the grpc engine)
		handler := handler.NewHandler(suite.log, suite.state, nil, suite.collClient, blocks, headers, collections, transactions)

		// 1. Assume that follower engine updated the block storage and the protocol state. The block is reported as sealed
		err = blocks.Store(&block)
		require.NoError(suite.T(), err)
		suite.snapshot.On("Seal").Return(seal, nil).Once()

		// 2. Ingest engine was notified by the follower engine about a new block.
		// Follower engine --> Ingest engine
		mb := &model.Block{
			BlockID: block.ID(),
		}
		ingestEng.OnFinalizedBlock(mb)
		time.Sleep(1 * time.Second)

		// 3. Ingest engine requests all collections of the block
		suite.collectionsConduit.AssertExpectations(suite.T())

		// 4. Ingest engine receives the requested collection
		cr := &messages.CollectionResponse{Collection: collection}
		err = ingestEng.Process(originID, cr)
		require.NoError(suite.T(), err)

		// 5. client requests a transaction
		tx := collection.Transactions[0]
		id := tx.ID()
		getReq := &access.GetTransactionRequest{
			Id: id[:],
		}
		gResp, err := handler.GetTransaction(context.Background(), getReq)
		require.NoError(suite.T(), err)
		// assert that the transaction is reported as Sealed
		require.Equal(suite.T(), entities.TransactionStatus_STATUS_SEALED, gResp.Transaction.Status)
	})
}

// TestExecuteScript tests the three execute Script related calls to make sure that the execution api is called with
// the correct block id
func (suite *Suite) TestExecuteScript() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {

		// initialize storage
		blocks := bstorage.NewBlocks(db)
		headers := bstorage.NewHeaders(db)

		// create a block and a seal pointing to that block
		lastBlock := unittest.BlockFixture()
		lastBlock.Height = 2
		seal := flow.Seal{
			BlockID: lastBlock.ID(),
		}
		err := blocks.Store(&lastBlock)
		require.NoError(suite.T(), err)
		err = db.Update(operation.InsertNumber(lastBlock.Height, lastBlock.ID()))
		require.NoError(suite.T(), err)
		suite.snapshot.On("Seal").Return(seal, nil).Once()

		// create another block as a predecessor of the block created earlier
		prevBlock := unittest.BlockFixture()
		prevBlock.Height = lastBlock.Height - 1
		err = blocks.Store(&prevBlock)
		require.NoError(suite.T(), err)
		err = db.Update(operation.InsertNumber(prevBlock.Height, prevBlock.ID()))
		require.NoError(suite.T(), err)

		ctx := context.Background()

		handler := handler.NewHandler(suite.log, suite.state, suite.execClient, suite.collClient, blocks, headers, nil, nil)

		script := []byte("dummy script")

		// setupExecClientMock sets up the mock the execution client and returns the access response to expect
		setupExecClientMock := func(blockID flow.Identifier) *access.ExecuteScriptResponse {
			id := blockID[:]
			executionReq := execution.ExecuteScriptAtBlockIDRequest{
				BlockId: id,
				Script:  script,
			}
			executionResp := execution.ExecuteScriptResponse{
				Value: []byte{9, 10, 11},
			}

			suite.execClient.On("ExecuteScriptAtBlockID", ctx, &executionReq).Return(&executionResp, nil).Once()

			expectedResp := access.ExecuteScriptResponse{
				Value: executionResp.GetValue(),
			}
			return &expectedResp
		}

		assertResult := func(err error, expected interface{}, actual interface{}) {
			suite.Require().NoError(err)
			suite.Require().Equal(expected, actual)
			suite.execClient.AssertExpectations(suite.T())
		}

		suite.Run("execute script at latest block", func() {
			expectedResp := setupExecClientMock(lastBlock.ID())
			req := access.ExecuteScriptAtLatestBlockRequest{
				Script: script,
			}
			actualResp, err := handler.ExecuteScriptAtLatestBlock(ctx, &req)
			assertResult(err, expectedResp, actualResp)
		})

		suite.Run("execute script at block id", func() {
			expectedResp := setupExecClientMock(prevBlock.ID())
			id := prevBlock.ID()
			req := access.ExecuteScriptAtBlockIDRequest{
				BlockId: id[:],
				Script:  script,
			}
			actualResp, err := handler.ExecuteScriptAtBlockID(ctx, &req)
			assertResult(err, expectedResp, actualResp)
		})

		suite.Run("execute script at block height", func() {
			expectedResp := setupExecClientMock(prevBlock.ID())
			req := access.ExecuteScriptAtBlockHeightRequest{
				BlockHeight: prevBlock.Height,
				Script:      script,
			}
			actualResp, err := handler.ExecuteScriptAtBlockHeight(ctx, &req)
			assertResult(err, expectedResp, actualResp)
		})
	})
}

func (suite *Suite) createChain() (flow.Block, flow.Collection, flow.Seal) {
	collection := unittest.CollectionFixture(10)
	cg := &flow.CollectionGuarantee{
		CollectionID: collection.ID(),
		Signature:    crypto.Signature([]byte("signature A")),
	}
	block := unittest.BlockFixture()
	block.Guarantees = []*flow.CollectionGuarantee{cg}

	seal := flow.Seal{
		BlockID: block.ID(),
	}

	return block, collection, seal
}
