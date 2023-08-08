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

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func (suite *Suite) WithPreConfiguredState(f func(snap protocol.Snapshot)) {
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

		// setup AtHeight mock returns for state
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeisght", epoch1.Range()).Return(state.AtHeight(height))
		}

		snap := state.AtHeight(epoch1.Range()[0])
		suite.state.On("Final").Return(snap).Once()
		suite.communicator.On("CallAvailableNode",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil)

		f(snap)
	})

}

func (suite *Suite) TestGetTransactionResultReturnsUnknown() {
	suite.WithPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})
		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.blocks.
			On("ByID", block.ID()).
			Return(&block, nil).
			Once()

		receipt := unittest.ExecutionReceiptFixture()
		identity := unittest.IdentityFixture()
		identity.Role = flow.RoleExecution

		l := flow.ExecutionReceiptList{receipt}

		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(l, nil)

		backend := New(
			suite.state,
			suite.colClient,
			nil,
			suite.blocks,
			nil,
			nil,
			suite.transactions,
			suite.receipts,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
			nil,
			*suite.communicator,
			false,
		)
		res, err := backend.GetTransactionResult(context.Background(), tx.ID(), block.ID(), coll.ID())
		suite.Require().NoError(err)
		suite.Require().Equal(res.Status, flow.TransactionStatusUnknown)
	})
}

func (suite *Suite) TestGetTransactionResultReturnsTransactionError() {
	suite.WithPreConfiguredState(func(snap protocol.Snapshot) {
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

		receipt := unittest.ExecutionReceiptFixture()
		identity := unittest.IdentityFixture()
		identity.Role = flow.RoleExecution

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		l := flow.ExecutionReceiptList{receipt}

		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(l, nil)

		backend := New(
			suite.state,
			suite.colClient,
			nil,
			suite.blocks,
			nil,
			nil,
			suite.transactions,
			suite.receipts,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
			nil,
			*suite.communicator,
			false,
		)
		_, err := backend.GetTransactionResult(context.Background(), tx.ID(), block.ID(), coll.ID())
		suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to find: %v", fmt.Errorf("some other error")))

	})
}

func (suite *Suite) TestGetTransactionResultReturnsValidTransactionResult() {
	suite.WithPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.blocks.
			On("ByID", block.ID()).
			Return(&block, nil).
			Once()

		suite.communicator.On("CallAvailableNode",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil)

		receipt := unittest.ExecutionReceiptFixture()
		identity := unittest.IdentityFixture()
		identity.Role = flow.RoleExecution

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		l := flow.ExecutionReceiptList{receipt}

		transactionResultResponse := access.TransactionResultResponse{
			Status:     entities.TransactionStatus_EXECUTED,
			StatusCode: uint32(entities.TransactionStatus_EXECUTED),
		}

		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(l, nil)

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.Anything).
			Return(&transactionResultResponse, nil)

		backend := New(
			suite.state,
			suite.colClient,
			[]access.AccessAPIClient{suite.historicalAccessClient},
			suite.blocks,
			nil,
			nil,
			suite.transactions,
			suite.receipts,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
			nil,
			*suite.communicator,
			false,
		)
		resp, err := backend.GetTransactionResult(context.Background(), tx.ID(), block.ID(), coll.ID())
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)
	})
}

func (suite *Suite) TestGetTransactionResultFromCache() {
	suite.WithPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.blocks.
			On("ByID", block.ID()).
			Return(&block, nil).
			Once()

		suite.communicator.On("CallAvailableNode",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil)

		receipt := unittest.ExecutionReceiptFixture()
		identity := unittest.IdentityFixture()
		identity.Role = flow.RoleExecution

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		l := flow.ExecutionReceiptList{receipt}

		transactionResultResponse := access.TransactionResultResponse{
			Status:     entities.TransactionStatus_EXECUTED,
			StatusCode: uint32(entities.TransactionStatus_EXECUTED),
		}

		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(l, nil)

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.Anything).
			Return(&transactionResultResponse, nil).Once()

		backend := New(
			suite.state,
			suite.colClient,
			[]access.AccessAPIClient{suite.historicalAccessClient},
			suite.blocks,
			nil,
			nil,
			suite.transactions,
			suite.receipts,
			nil,
			suite.chainID,
			metrics.NewNoopCollector(),
			nil,
			false,
			DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			DefaultSnapshotHistoryLimit,
			nil,
			*suite.communicator,
			false,
		)

		resp, err := backend.GetTransactionResult(context.Background(), tx.ID(), block.ID(), coll.ID())
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)

		resp2, err := backend.GetTransactionResult(context.Background(), tx.ID(), block.ID(), coll.ID())
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp2.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp2.StatusCode)

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}
