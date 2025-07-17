package retrier_test

//
//import (
//	"context"
//
//	"github.com/onflow/flow/protobuf/go/flow/access"
//	"github.com/onflow/flow/protobuf/go/flow/entities"
//	"github.com/onflow/flow/protobuf/go/flow/execution"
//	"github.com/rs/zerolog"
//	"github.com/stretchr/testify/mock"
//	"github.com/stretchr/testify/suite"
//	"google.golang.org/grpc/codes"
//	"google.golang.org/grpc/status"
//
//	accessmock "github.com/onflow/flow-go/engine/access/mock"
//	"github.com/onflow/flow-go/engine/access/rpc/backend/retrier"
//	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions"
//	"github.com/onflow/flow-go/model/flow"
//	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
//	storagemock "github.com/onflow/flow-go/storage/mock"
//	"github.com/onflow/flow-go/utils/unittest"
//)
//
//type Suite struct {
//	suite.Suite
//
//	log      zerolog.Logger
//	state    *protocolmock.State
//	snapshot *protocolmock.Snapshot
//
//	blocks             *storagemock.Blocks
//	headers            *storagemock.Headers
//	collections        *storagemock.Collections
//	transactions       *storagemock.Transactions
//	receipts           *storagemock.ExecutionReceipts
//	results            *storagemock.ExecutionResults
//	transactionResults *storagemock.LightTransactionResults
//	events             *storagemock.Events
//	txErrorMessages    *storagemock.TransactionResultErrorMessages
//}
//
//func (suite *Suite) TestSuccessfulTransactionsDontRetry() {
//	ctx := context.Background()
//	collection := unittest.CollectionFixture(1)
//	transactionBody := collection.Transactions[0]
//	block := unittest.BlockFixture()
//	// Height needs to be at least DefaultTransactionExpiry before we start doing retries
//	block.Header.Height = flow.DefaultTransactionExpiry + 1
//	refBlock := unittest.BlockFixture()
//	refBlock.Header.Height = 2
//	transactionBody.SetReferenceBlockID(refBlock.ID())
//
//	block.SetPayload(
//		unittest.PayloadFixture(
//			unittest.WithGuarantees(
//				unittest.CollectionGuaranteesWithCollectionIDFixture([]*flow.Collection{&collection})...)))
//
//	light := collection.Light()
//	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
//	// transaction storage returns the corresponding transaction
//	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil)
//	// collection storage returns the corresponding collection
//	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(&light, nil)
//	suite.collections.On("LightByID", light.ID()).Return(&light, nil)
//	// block storage returns the corresponding block
//	suite.blocks.On("ByCollectionID", collection.ID()).Return(&block, nil)
//
//	txID := transactionBody.ID()
//	blockID := block.ID()
//	exeEventReq := execution.GetTransactionResultRequest{
//		BlockId:       blockID[:],
//		TransactionId: txID[:],
//	}
//	exeEventResp := execution.GetTransactionResultResponse{
//		Events: nil,
//	}
//
//	_, enIDs := suite.setupReceipts(&block)
//	suite.snapshot.On("Identities", mock.Anything).Return(enIDs, nil)
//
//	client := accessmock.NewAccessAPIClient(suite.T())
//	params := suite.defaultTransactionsParams()
//	params.StaticCollectionRPCClient = client
//	txBackend, err := transactions.NewTransactionsBackend(params)
//	suite.Require().NoError(err)
//
//	retry := retrier.NewRetrier(
//		suite.log,
//		suite.blocks,
//		suite.collections,
//		txBackend,
//		txBackend.txStatusDeriver,
//	)
//	retry.Activate()
//	retry.RegisterTransaction(block.Header.Height, transactionBody)
//
//	client.On("SendTransaction", mock.Anything, mock.Anything).Return(&access.SendTransactionResponse{}, nil)
//
//	// return not found to return finalized status
//	suite.execClient.On("GetTransactionResult", ctx, &exeEventReq).
//		Return(&exeEventResp, status.Errorf(codes.NotFound, "not found")).
//		Times(len(enIDs)) // should call each EN once
//
//	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
//	result, err := txBackend.GetTransactionResult(
//		ctx,
//		txID,
//		flow.ZeroID,
//		flow.ZeroID,
//		entities.EventEncodingVersion_JSON_CDC_V0,
//	)
//	suite.checkResponse(result, err)
//
//	// status should be finalized since the sealed Blocks is smaller in height
//	suite.Assert().Equal(flow.TransactionStatusFinalized, result.Status)
//
//	// Don't retry now that block is finalized
//	err = retry.Retry(block.Header.Height + 1)
//	suite.Require().NoError(err)
//
//	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)
//
//	// Don't retry now that block is finalized
//	err = retry.Retry(block.Header.Height + retrier.RetryFrequency)
//	suite.Require().NoError(err)
//
//	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)
//
//	// Don't retry now that block is finalized
//	err = retry.Retry(block.Header.Height + retrier.RetryFrequency + flow.DefaultTransactionExpiry)
//	suite.Require().NoError(err)
//
//	// Should've still should not be called
//	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)
//
//	suite.assertAllExpectations()
//}
