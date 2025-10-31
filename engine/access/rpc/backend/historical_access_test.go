package backend

import (
	"context"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHistoricalTransactionResult tests to see if the historical transaction status can be retrieved
func (suite *Suite) TestHistoricalTransactionResult() {
	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	txID := transactionBody.ID()

	suite.collections.
		On("LightByTransactionID", txID).
		Return(nil, storage.ErrNotFound)

	// transaction storage returns the corresponding transaction
	suite.transactions.
		On("ByID", txID).
		Return(nil, storage.ErrNotFound)

	accessEventReq := accessproto.GetTransactionRequest{
		Id: txID[:],
	}

	accessEventResp := accessproto.TransactionResultResponse{
		Status: entities.TransactionStatus(flow.TransactionStatusSealed),
		Events: nil,
	}

	params := suite.defaultBackendParams()
	params.HistoricalAccessNodes = []accessproto.AccessAPIClient{suite.historicalAccessClient}

	backend, err := New(params)
	suite.Require().NoError(err)

	// Successfully return the transaction from the historical node
	suite.historicalAccessClient.
		On("GetTransactionResult", ctx, &accessEventReq).
		Return(&accessEventResp, nil).
		Once()

	// Make the call for the transaction result
	result, err := backend.GetTransactionResult(
		ctx,
		txID,
		flow.ZeroID,
		flow.ZeroID,
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

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
		Return(nil, storage.ErrNotFound)

	accessEventReq := accessproto.GetTransactionRequest{
		Id: txID[:],
	}

	accessEventResp := accessproto.TransactionResponse{
		Transaction: convert.TransactionToMessage(*transactionBody),
	}

	params := suite.defaultBackendParams()
	params.HistoricalAccessNodes = []accessproto.AccessAPIClient{suite.historicalAccessClient}

	backend, err := New(params)
	suite.Require().NoError(err)

	// Successfully return the transaction from the historical node
	suite.historicalAccessClient.
		On("GetTransaction", ctx, &accessEventReq).
		Return(&accessEventResp, nil).
		Once()

	// Make the call for the transaction result
	tx, err := backend.GetTransaction(ctx, txID)
	suite.Require().NoError(err)
	suite.Require().NotNil(tx)

	suite.assertAllExpectations()
}
