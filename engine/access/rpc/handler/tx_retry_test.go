package handler

import (
	"context"

	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/model/flow"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	realstorage "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestTransactionRetry tests that the retry mechanism will send retries at specific times
func (suite *Suite) TestTransactionRetry() {

	// ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	// Height needs to be at least DefaultTransactionExpiry before we start doing retries
	block.Header.Height = flow.DefaultTransactionExpiry + 1
	transactionBody.SetReferenceBlockID(block.ID())
	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block
	suite.snapshot.On("Head").Return(headBlock.Header, nil)
	snapshotAtBlock := new(protocol.Snapshot)
	snapshotAtBlock.On("Head").Return(block.Header, nil)
	suite.state.On("AtBlockID", block.ID()).Return(snapshotAtBlock, nil)

	// collection storage returns a not found error
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(nil, realstorage.ErrNotFound)

	// txID := transactionBody.ID()
	// blockID := block.ID()
	// Setup Handler + Retry
	handler := NewHandler(suite.log, suite.state, suite.execClient, suite.colClient, suite.blocks, suite.headers, suite.collections, suite.transactions)
	retry := newRetry().SetHandler(handler)
	handler.retry = retry

	retry.RegisterTransaction(block.Header.Height, transactionBody)

	suite.colClient.On("SendTransaction", mock.Anything, mock.Anything).Return(&access.SendTransactionResponse{}, nil)

	// Don't retry on every height
	retry.Retry(block.Header.Height + 1)

	suite.colClient.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Retry every `retryFrequency`
	retry.Retry(block.Header.Height + retryFrequency)

	suite.colClient.AssertNumberOfCalls(suite.T(), "SendTransaction", 1)

	// do not retry if expired
	retry.Retry(block.Header.Height + retryFrequency + flow.DefaultTransactionExpiry)

	// Should've still only been called once
	suite.colClient.AssertNumberOfCalls(suite.T(), "SendTransaction", 1)

	suite.assertAllExpectations()
}

// TestSuccessfulTransactionsDontRetry tests that the retry mechanism will send retries at specific times
func (suite *Suite) TestSuccessfulTransactionsDontRetry() {

	ctx := context.Background()
	collection := unittest.CollectionFixture(1)
	transactionBody := collection.Transactions[0]
	block := unittest.BlockFixture()
	// Height needs to be at least DefaultTransactionExpiry before we start doing retries
	block.Header.Height = flow.DefaultTransactionExpiry + 1
	transactionBody.SetReferenceBlockID(block.ID())
	headBlock := unittest.BlockFixture()
	headBlock.Header.Height = block.Header.Height - 1 // head is behind the current block
	suite.snapshot.On("Head").Return(headBlock.Header, nil)

	light := collection.Light()
	// transaction storage returns the corresponding transaction
	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil)
	// collection storage returns the corresponding collection
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(&light, nil)
	// block storage returns the corresponding block
	suite.blocks.On("ByCollectionID", collection.ID()).Return(&block, nil)

	txID := transactionBody.ID()
	blockID := block.ID()
	exeEventReq := execution.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}
	exeEventResp := execution.GetTransactionResultResponse{
		Events: nil,
	}

	req := &access.GetTransactionRequest{
		Id: txID[:],
	}

	// Setup Handler + Retry
	handler := NewHandler(suite.log, suite.state, suite.execClient, suite.colClient, suite.blocks, suite.headers, suite.collections, suite.transactions)
	retry := newRetry().SetHandler(handler)
	handler.retry = retry

	retry.RegisterTransaction(block.Header.Height, transactionBody)

	suite.colClient.On("SendTransaction", mock.Anything, mock.Anything).Return(&access.SendTransactionResponse{}, nil)

	// return not found to return finalized status
	suite.execClient.On("GetTransactionResult", ctx, &exeEventReq).Return(&exeEventResp, status.Errorf(codes.NotFound, "not found")).Once()
	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	resp, err := handler.GetTransactionResult(ctx, req)
	suite.checkResponse(resp, err)

	// status should be finalized since the sealed blocks is smaller in height
	suite.Assert().Equal(entities.TransactionStatus_FINALIZED, resp.GetStatus())

	// Don't retry now now that block is finalized
	retry.Retry(block.Header.Height + 1)

	suite.colClient.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Don't retry now now that block is finalized
	retry.Retry(block.Header.Height + retryFrequency)

	suite.colClient.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Don't retry now now that block is finalized
	retry.Retry(block.Header.Height + retryFrequency + flow.DefaultTransactionExpiry)

	// Should've still should not be called
	suite.colClient.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	suite.assertAllExpectations()
}
