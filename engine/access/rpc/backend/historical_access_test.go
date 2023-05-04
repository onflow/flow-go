package backend

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHistoricalTransactionResult tests to see if the historical transaction status can be retrieved
func (suite *Suite) TestHistoricalTransactionResult() {

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]

	txID := transactionBody.ID()
	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", txID).
		Return(nil, status.Errorf(codes.NotFound, "not found on main node"))

	accessEventReq := accessproto.GetTransactionRequest{
		Id: txID[:],
	}

	accessEventResp := accessproto.TransactionResultResponse{
		Status: entities.TransactionStatus(flow.TransactionStatusSealed),
		Events: nil,
	}

	backend := New(suite.state,
		nil,
		[]accessproto.AccessAPIClient{suite.historicalAccessClient},
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.receipts,
		suite.results,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// Successfully return the transaction from the historical node
	suite.historicalAccessClient.
		On("GetTransactionResult", ctx, &accessEventReq).
		Return(&accessEventResp, nil).
		Once()

	// Make the call for the transaction result
	result, err := backend.GetTransactionResult(ctx, txID)
	suite.checkResponse(result, err)

	// status should be sealed
	suite.Assert().Equal(flow.TransactionStatusSealed, result.Status)

	suite.assertAllExpectations()
}

// TestHistoricalTransaction tests to see if the historical transaction can be retrieved
func (suite *Suite) TestHistoricalTransaction() {

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]

	txID := transactionBody.ID()
	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", txID).
		Return(nil, status.Errorf(codes.NotFound, "not found on main node"))

	accessEventReq := accessproto.GetTransactionRequest{
		Id: txID[:],
	}

	accessEventResp := accessproto.TransactionResponse{
		Transaction: convert.TransactionToMessage(*transactionBody),
	}

	backend := New(suite.state,
		nil,
		[]accessproto.AccessAPIClient{suite.historicalAccessClient},
		suite.blocks,
		suite.headers,
		suite.collections,
		suite.transactions,
		suite.receipts,
		suite.results,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// Successfully return the transaction from the historical node
	suite.historicalAccessClient.
		On("GetTransaction", ctx, &accessEventReq).
		Return(&accessEventResp, nil).
		Once()

	// Make the call for the transaction result
	tx, err := backend.GetTransaction(ctx, txID)
	suite.checkResponse(tx, err)

	suite.assertAllExpectations()
}
