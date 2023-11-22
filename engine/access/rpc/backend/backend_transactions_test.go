package backend

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	acc "github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func (suite *Suite) withPreConfiguredState(f func(snap protocol.Snapshot)) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)

		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtHeight mock returns for State
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", epoch1.Range()).Return(state.AtHeight(height))
		}

		snap := state.AtHeight(epoch1.Range()[0])
		suite.state.On("Final").Return(snap).Once()
		suite.communicator.On("CallAvailableNode",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil).Once()

		f(snap)
	})

}

// TestGetTransactionResultReturnsUnknown returns unknown result when tx not found
func (suite *Suite) TestGetTransactionResultReturnsUnknown() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})
		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		params := suite.defaultBackendParams()
		params.Communicator = suite.communicator

		backend, err := New(params)
		suite.Require().NoError(err)

		res, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(res.Status, flow.TransactionStatusUnknown)
	})
}

// TestGetTransactionResultReturnsTransactionError returns error from transaction storage
func (suite *Suite) TestGetTransactionResultReturnsTransactionError() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, fmt.Errorf("some other error"))

		suite.blocks.
			On("ByID", block.ID()).
			Return(&block, nil).
			Once()

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		params := suite.defaultBackendParams()
		params.Communicator = suite.communicator

		backend, err := New(params)
		suite.Require().NoError(err)

		_, err = backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to find: %v", fmt.Errorf("some other error")))

	})
}

// TestGetTransactionResultReturnsValidTransactionResultFromHistoricNode tests lookup in historic nodes
func (suite *Suite) TestGetTransactionResultReturnsValidTransactionResultFromHistoricNode() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		transactionResultResponse := access.TransactionResultResponse{
			Status:     entities.TransactionStatus_EXECUTED,
			StatusCode: uint32(entities.TransactionStatus_EXECUTED),
		}

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.Anything).
			Return(&transactionResultResponse, nil).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator

		backend, err := New(params)
		suite.Require().NoError(err)

		resp, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)
	})
}

func (suite *Suite) withGetTransactionCachingTestSetup(f func(b *flow.Block, t *flow.Transaction)) {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		f(&block, &tx)
	})
}

// TestGetTransactionResultFromCache get historic transaction result from cache
func (suite *Suite) TestGetTransactionResultFromCache() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {
		transactionResultResponse := access.TransactionResultResponse{
			Status:     entities.TransactionStatus_EXECUTED,
			StatusCode: uint32(entities.TransactionStatus_EXECUTED),
		}

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*access.GetTransactionRequest")).
			Return(&transactionResultResponse, nil).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator
		params.TxResultCacheSize = 10

		backend, err := New(params)
		suite.Require().NoError(err)

		coll := flow.CollectionFromTransactions([]*flow.Transaction{tx})

		resp, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)

		resp2, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp2.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp2.StatusCode)

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}

// TestGetTransactionResultCacheNonExistent tests caches non existing result
func (suite *Suite) TestGetTransactionResultCacheNonExistent() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*access.GetTransactionRequest")).
			Return(nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", tx.ID())).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator
		params.TxResultCacheSize = 10

		backend, err := New(params)
		suite.Require().NoError(err)

		coll := flow.CollectionFromTransactions([]*flow.Transaction{tx})

		resp, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusUnknown, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusUnknown), resp.StatusCode)

		// ensure the unknown transaction is cached when not found anywhere
		txStatus := flow.TransactionStatusUnknown
		res, ok := backend.txResultCache.Get(tx.ID())
		suite.Require().True(ok)
		suite.Require().Equal(res, &acc.TransactionResult{
			Status:     txStatus,
			StatusCode: uint(txStatus),
		})

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}

// TestGetTransactionResultUnknownFromCache retrive unknown result from cache
func (suite *Suite) TestGetTransactionResultUnknownFromCache() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {
		suite.historicalAccessClient.
			On("GetTransactionResult", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*access.GetTransactionRequest")).
			Return(nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", tx.ID())).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator
		params.TxResultCacheSize = 10

		backend, err := New(params)
		suite.Require().NoError(err)

		coll := flow.CollectionFromTransactions([]*flow.Transaction{tx})

		resp, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusUnknown, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusUnknown), resp.StatusCode)

		// ensure the unknown transaction is cached when not found anywhere
		txStatus := flow.TransactionStatusUnknown
		res, ok := backend.txResultCache.Get(tx.ID())
		suite.Require().True(ok)
		suite.Require().Equal(res, &acc.TransactionResult{
			Status:     txStatus,
			StatusCode: uint(txStatus),
		})

		resp2, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusUnknown, resp2.Status)
		suite.Require().Equal(uint(flow.TransactionStatusUnknown), resp2.StatusCode)

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}
