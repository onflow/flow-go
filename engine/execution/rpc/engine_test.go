package rpc

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	mockEng "github.com/onflow/flow-go/engine/execution/mock"
	"github.com/onflow/flow-go/engine/execution/scripts"
	"github.com/onflow/flow-go/model/flow"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	log        zerolog.Logger
	events     *storage.Events
	exeResults *storage.ExecutionResults
	txResults  *storage.TransactionResults
	commits    *storage.Commits
	headers    *storage.Headers
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.events = storage.NewEvents(suite.T())
	suite.exeResults = storage.NewExecutionResults(suite.T())
	suite.txResults = storage.NewTransactionResults(suite.T())
	suite.commits = storage.NewCommits(suite.T())
	suite.headers = storage.NewHeaders(suite.T())
}

// TestExecuteScriptAtBlockID tests the ExecuteScriptAtBlockID API call
func (suite *Suite) TestExecuteScriptAtBlockID() {
	// setup handler
	mockEngine := mockEng.NewScriptExecutor(suite.T())
	handler := &handler{
		engine:  mockEngine,
		chain:   flow.Mainnet,
		commits: suite.commits,
	}

	// setup dummy request/response
	ctx := context.Background()
	mockIdentifier := unittest.IdentifierFixture()
	script := []byte("dummy script")
	arguments := [][]byte(nil)
	executionReq := execution.ExecuteScriptAtBlockIDRequest{
		BlockId: mockIdentifier[:],
		Script:  script,
	}
	scriptExecValue := []byte{9, 10, 11}
	executionResp := execution.ExecuteScriptAtBlockIDResponse{
		Value: scriptExecValue,
	}

	suite.Run("happy path with successful script execution", func() {
		suite.commits.On("ByBlockID", mockIdentifier).Return(nil, nil).Once()
		mockEngine.On("ExecuteScriptAtBlockID", ctx, script, arguments, mockIdentifier).
			Return(scriptExecValue, nil).Once()
		response, err := handler.ExecuteScriptAtBlockID(ctx, &executionReq)
		suite.Require().NoError(err)
		suite.Require().Equal(&executionResp, response)
		mockEngine.AssertExpectations(suite.T())
	})

	suite.Run("valid request for unknown block", func() {
		suite.commits.On("ByBlockID", mockIdentifier).Return(nil, realstorage.ErrNotFound).Once()
		_, err := handler.ExecuteScriptAtBlockID(ctx, &executionReq)
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.NotFound, ""))
	})

	suite.Run("valid request with script execution failure", func() {
		suite.commits.On("ByBlockID", mockIdentifier).Return(nil, nil).Once()
		mockEngine.On("ExecuteScriptAtBlockID", ctx, script, arguments, mockIdentifier).
			Return(nil, status.Error(codes.InvalidArgument, "")).Once()
		_, err := handler.ExecuteScriptAtBlockID(ctx, &executionReq)
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	suite.Run("invalid request with nil blockID", func() {
		executionReqWithNilBlock := execution.ExecuteScriptAtBlockIDRequest{
			BlockId: nil,
			Script:  script,
		}
		_, err := handler.ExecuteScriptAtBlockID(ctx, &executionReqWithNilBlock)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

}

// TestGetEventsForBlockIDs tests the GetEventsForBlockIDs API call
func (suite *Suite) TestGetEventsForBlockIDs() {

	totalBlocks := 10
	eventsPerBlock := 10

	blockIDs := make([][]byte, totalBlocks)
	expectedResult := make([]*execution.GetEventsForBlockIDsResponse_Result, totalBlocks)

	// setup the events storage mock
	for i := range blockIDs {
		block := unittest.BlockFixture()
		block.Header.Height = uint64(i)
		id := block.ID()
		blockIDs[i] = id[:]
		eventsForBlock := make([]flow.Event, eventsPerBlock)
		eventMessages := make([]*entities.Event, eventsPerBlock)
		for j := range eventsForBlock {
			e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture(), 0)
			eventsForBlock[j] = e
			eventMessages[j] = convert.EventToMessage(e)
		}
		// expect one call to lookup result for each block ID
		suite.commits.On("ByBlockID", id).Return(nil, nil).Once()

		// expect one call to lookup events for each block ID
		suite.events.On("ByBlockIDEventType", id, flow.EventAccountCreated).Return(eventsForBlock, nil).Once()

		// expect one call to lookup each block
		suite.headers.On("ByBlockID", id).Return(block.Header, nil).Once()

		// create the expected result for this block
		expectedResult[i] = &execution.GetEventsForBlockIDsResponse_Result{
			BlockId:     id[:],
			BlockHeight: block.Header.Height,
			Events:      eventMessages,
		}
	}

	// create the handler
	handler := &handler{
		headers:            suite.headers,
		events:             suite.events,
		exeResults:         suite.exeResults,
		transactionResults: suite.txResults,
		commits:            suite.commits,
		chain:              flow.Mainnet,
		maxBlockRange:      DefaultMaxBlockRange,
	}

	concoctReq := func(errType string, blockIDs [][]byte) *execution.GetEventsForBlockIDsRequest {
		return &execution.GetEventsForBlockIDsRequest{
			Type:     errType,
			BlockIds: blockIDs,
		}
	}

	// happy path - valid requests receives a valid response
	suite.Run("happy path", func() {

		// create a valid API request
		req := concoctReq(string(flow.EventAccountCreated), blockIDs)

		// execute the GetEventsForBlockIDs call
		resp, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		actualResult := resp.GetResults()
		suite.Require().ElementsMatch(expectedResult, actualResult)
	})

	// failure path - empty even type in the request results in an error
	suite.Run("request with empty event type", func() {

		// create an API request with empty even type
		req := concoctReq("", blockIDs)

		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	// failure path - empty block ids in request results in an error
	suite.Run("request with empty block IDs", func() {

		// create an API request with empty block ids
		req := concoctReq(string(flow.EventAccountCreated), nil)

		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	// failure path - non-existent block id in request results in an error
	suite.Run("request with non-existent block ID", func() {

		id := unittest.IdentifierFixture()

		// expect a storage call for the invalid id but return an error
		suite.commits.On("ByBlockID", id).Return(nil, realstorage.ErrNotFound).Once()

		// create an API request with the invalid block id
		req := concoctReq(string(flow.EventAccountCreated), [][]byte{id[:]})

		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.NotFound, ""))
	})

	// request for too many blocks - receives a InvalidArgument error
	suite.Run("request for too many blocks", func() {

		// update range so it's smaller than list of blockIDs
		handler.maxBlockRange = totalBlocks / 2

		// create a valid API request
		req := concoctReq(string(flow.EventAccountCreated), blockIDs)

		// execute the GetEventsForBlockIDs call
		_, err := handler.GetEventsForBlockIDs(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})
}

// Test GetAccountAtBlockID tests the GetAccountAtBlockID API call
func (suite *Suite) TestGetAccountAtBlockID() {

	id := unittest.IdentifierFixture()
	serviceAddress := flow.Mainnet.Chain().ServiceAddress()

	serviceAccount := flow.Account{
		Address: serviceAddress,
	}

	mockEngine := mockEng.NewScriptExecutor(suite.T())

	// create the handler
	handler := &handler{
		engine:  mockEngine,
		chain:   flow.Mainnet,
		commits: suite.commits,
	}

	createReq := func(id []byte, address []byte) *execution.GetAccountAtBlockIDRequest {
		return &execution.GetAccountAtBlockIDRequest{
			Address: address,
			BlockId: id,
		}
	}

	suite.Run("happy path with valid request", func() {

		// setup mock expectations
		suite.commits.On("ByBlockID", id).Return(nil, nil).Once()

		mockEngine.On("GetAccount", mock.Anything, serviceAddress, id).Return(&serviceAccount, nil).Once()

		req := createReq(id[:], serviceAddress.Bytes())

		resp, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().NoError(err)
		actualAccount := resp.GetAccount()
		expectedAccount, err := convert.AccountToMessage(&serviceAccount)
		suite.Require().NoError(err)
		suite.Require().Empty(
			cmp.Diff(expectedAccount, actualAccount, protocmp.Transform()))
	})

	suite.Run("invalid request with unknown block id", func() {

		// setup mock expectations
		suite.commits.On("ByBlockID", id).Return(nil, realstorage.ErrNotFound).Once()

		req := createReq(id[:], serviceAddress.Bytes())

		_, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.NotFound, ""))
	})

	suite.Run("invalid request with nil block id", func() {

		req := createReq(nil, serviceAddress.Bytes())

		_, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	suite.Run("invalid request with nil root address", func() {

		req := createReq(id[:], nil)

		_, err := handler.GetAccountAtBlockID(context.Background(), req)

		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	suite.Run("valid request for unavailable data", func() {
		suite.commits.On("ByBlockID", id).Return(nil, nil).Once()

		expectedErr := fmt.Errorf(
			"failed to execute script at block (%s): %w (%s). "+
				"this error usually happens if the reference "+
				"block for this script is not set to a recent block.",
			id,
			scripts.ErrStateCommitmentPruned,
			unittest.IdentifierFixture(),
		)

		mockEngine.On("GetAccount", mock.Anything, serviceAddress, id).Return(nil, expectedErr).Once()

		req := createReq(id[:], serviceAddress.Bytes())

		resp, err := handler.GetAccountAtBlockID(context.Background(), req)
		suite.Assert().Nil(resp)
		suite.Assert().Equal(codes.OutOfRange, status.Code(err))
	})
}

// Test GetRegisterAtBlockID tests the GetRegisterAtBlockID API call
func (suite *Suite) TestGetRegisterAtBlockID() {

	id := unittest.IdentifierFixture()
	serviceAddress := flow.Mainnet.Chain().ServiceAddress()
	validKey := []byte("exists")

	mockEngine := mockEng.NewScriptExecutor(suite.T())

	// create the handler
	handler := &handler{
		engine: mockEngine,
		chain:  flow.Mainnet,
	}

	createReq := func(id, owner, key []byte) *execution.GetRegisterAtBlockIDRequest {
		return &execution.GetRegisterAtBlockIDRequest{
			RegisterOwner: owner,
			RegisterKey:   key,
			BlockId:       id,
		}
	}

	suite.Run("happy path with valid request", func() {

		// setup mock expectations
		mockEngine.On("GetRegisterAtBlockID", mock.Anything, serviceAddress.Bytes(), validKey, id).Return([]uint8{1}, nil).Once()

		req := createReq(id[:], serviceAddress.Bytes(), validKey)
		resp, err := handler.GetRegisterAtBlockID(context.Background(), req)

		suite.Require().NoError(err)
		value := resp.GetValue()
		suite.Require().NoError(err)
		suite.Require().True(len(value) > 0)
	})

	suite.Run("invalid request with bad address", func() {
		badOwner := []byte("\uFFFD")
		// return error
		mockEngine.On("GetRegisterAtBlockID", mock.Anything, badOwner, validKey, id).Return(nil, errors.New("error")).Once()

		req := createReq(id[:], badOwner, validKey)
		_, err := handler.GetRegisterAtBlockID(context.Background(), req)
		suite.Require().Error(err)
	})
}

// TestGetTransactionResult tests the GetTransactionResult and GetTransactionResultByIndex API calls
func (suite *Suite) TestGetTransactionResult() {

	totalEvents := 10
	block := unittest.BlockFixture()
	tx := unittest.TransactionFixture()
	bID := block.ID()
	txID := tx.ID()
	txIndex := rand.Uint32()

	// setup the events storage mock
	eventsForTx := make([]flow.Event, totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j := range eventsForTx {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture(), 0)
		eventsForTx[j] = e
		eventMessages[j] = convert.EventToMessage(e)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", bID, txID).Return(eventsForTx, nil)

	// create the handler
	createHandler := func(txResults *storage.TransactionResults) *handler {
		handler := &handler{
			headers:            suite.headers,
			events:             suite.events,
			transactionResults: txResults,
			commits:            suite.commits,
			chain:              flow.Mainnet,
		}
		return handler
	}

	// concoctReq creates a GetEventsForBlockIDTransactionIDRequest
	concoctReq := func(bID []byte, tID []byte) *execution.GetTransactionResultRequest {
		return &execution.GetTransactionResultRequest{
			BlockId:       bID,
			TransactionId: tID,
		}
	}

	// concoctIndexReq creates a GetTransactionByIndexRequest
	concoctIndexReq := func(bID []byte, tIndex uint32) *execution.GetTransactionByIndexRequest {
		return &execution.GetTransactionByIndexRequest{
			BlockId: bID,
			Index:   uint32(tIndex),
		}
	}

	assertEqual := func(expected, actual *execution.GetTransactionResultResponse) {
		suite.Require().Equal(expected.GetStatusCode(), actual.GetStatusCode())
		suite.Require().Equal(expected.GetErrorMessage(), actual.GetErrorMessage())
		suite.Require().ElementsMatch(expected.GetEvents(), actual.GetEvents())
	}

	// happy path - valid requests receives all events for the given transaction
	suite.Run("happy path with valid events and no transaction error", func() {

		// create the expected result
		expectedResult := &execution.GetTransactionResultResponse{
			StatusCode:   0,
			ErrorMessage: "",
			Events:       eventMessages,
		}

		// expect a call to lookup transaction result by block ID and transaction ID, return a result with no error
		txResults := storage.NewTransactionResults(suite.T())
		txResult := flow.TransactionResult{
			TransactionID: flow.Identifier{},
			ErrorMessage:  "",
		}
		txResults.On("ByBlockIDTransactionID", bID, txID).Return(&txResult, nil).Once()

		handler := createHandler(txResults)

		// create a valid API request
		req := concoctReq(bID[:], txID[:])

		// execute the GetTransactionResult call
		actualResult, err := handler.GetTransactionResult(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		// check that all fields in response are as expected
		assertEqual(expectedResult, actualResult)
	})

	// happy path - valid requests receives all events for the given transaction by index
	suite.Run("index happy path with valid events and no transaction error", func() {

		// create the expected result
		expectedResult := &execution.GetTransactionResultResponse{
			StatusCode:   0,
			ErrorMessage: "",
			Events:       eventMessages,
		}

		// expect a call to lookup transaction result by block ID and transaction ID, return a result with no error
		txResults := storage.NewTransactionResults(suite.T())
		txResult := flow.TransactionResult{
			TransactionID: flow.Identifier{},
			ErrorMessage:  "",
		}
		txResults.On("ByBlockIDTransactionIndex", bID, txIndex).Return(&txResult, nil).Once()

		// expect a call to lookup events by block ID and tx index
		suite.events.On("ByBlockIDTransactionIndex", bID, txIndex).Return(eventsForTx, nil).Once()

		handler := createHandler(txResults)

		// create a valid API request
		req := concoctIndexReq(bID[:], txIndex)

		// execute the GetTransactionResult call
		actualResult, err := handler.GetTransactionResultByIndex(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		// check that all fields in response are as expected
		assertEqual(expectedResult, actualResult)
	})

	// happy path - valid requests receives all events and an error for the given transaction
	suite.Run("happy path with valid events and a transaction error", func() {

		// create the expected result
		expectedResult := &execution.GetTransactionResultResponse{
			StatusCode:   1,
			ErrorMessage: "runtime error",
			Events:       eventMessages,
		}

		// setup the storage to return a transaction error
		txResults := storage.NewTransactionResults(suite.T())
		txResult := flow.TransactionResult{
			TransactionID: txID,
			ErrorMessage:  "runtime error",
		}
		txResults.On("ByBlockIDTransactionID", bID, txID).Return(&txResult, nil).Once()

		handler := createHandler(txResults)

		// create a valid API request
		req := concoctReq(bID[:], txID[:])

		// execute the GetEventsForBlockIDTransactionID call
		actualResult, err := handler.GetTransactionResult(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		// check that all fields in response are as expected
		assertEqual(expectedResult, actualResult)
	})

	// happy path - valid requests receives all events and an error for the given transaction
	suite.Run("index happy path with valid events and a transaction error", func() {

		// create the expected result
		expectedResult := &execution.GetTransactionResultResponse{
			StatusCode:   1,
			ErrorMessage: "runtime error",
			Events:       eventMessages,
		}

		// setup the storage to return a transaction error
		txResults := storage.NewTransactionResults(suite.T())
		txResult := flow.TransactionResult{
			TransactionID: txID,
			ErrorMessage:  "runtime error",
		}
		txResults.On("ByBlockIDTransactionIndex", bID, txIndex).Return(&txResult, nil).Once()

		// expect a call to lookup events by block ID and tx index
		suite.events.On("ByBlockIDTransactionIndex", bID, txIndex).Return(eventsForTx, nil).Once()

		handler := createHandler(txResults)

		// create a valid API request
		req := concoctIndexReq(bID[:], txIndex)

		// execute the GetEventsForBlockIDTransactionID call
		actualResult, err := handler.GetTransactionResultByIndex(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		// check that all fields in response are as expected
		assertEqual(expectedResult, actualResult)

		// check that appropriate storage calls were made
	})

	// failure path - nil transaction ID in the request results in an error
	suite.Run("request with nil tx ID", func() {

		// create an API request with transaction ID as nil
		req := concoctReq(bID[:], nil)

		txResults := storage.NewTransactionResults(suite.T())
		handler := createHandler(txResults)

		_, err := handler.GetTransactionResult(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	// failure path - nil block id in the request results in an error
	suite.Run("request with nil block ID", func() {

		// create an API request with a nil block id
		req := concoctReq(nil, txID[:])

		txResults := storage.NewTransactionResults(suite.T())
		handler := createHandler(txResults)

		_, err := handler.GetTransactionResult(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	// failure path - nil block id in the index request results in an error
	suite.Run("index request with nil block ID", func() {

		// create an API request with a nil block id
		req := concoctIndexReq(nil, txIndex)

		txResults := storage.NewTransactionResults(suite.T())
		handler := createHandler(txResults)

		_, err := handler.GetTransactionResultByIndex(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	// failure path - non-existent transaction ID in request results in an error
	suite.Run("request with non-existent transaction ID", func() {

		wrongTxID := unittest.IdentifierFixture()

		// create an API request with the invalid transaction ID
		req := concoctReq(bID[:], wrongTxID[:])

		// expect a storage call for the invalid tx ID but return an error
		txResults := storage.NewTransactionResults(suite.T())
		txResults.On("ByBlockIDTransactionID", bID, wrongTxID).Return(nil, status.Error(codes.Internal, "")).Once()

		handler := createHandler(txResults)

		_, err := handler.GetTransactionResult(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.Internal, ""))
	})

	// failure path - non-existent transaction index in request results in an error
	suite.Run("request with non-existent transaction index", func() {

		wrongTxIndex := txIndex + 1

		// create an API request with the invalid transaction ID
		req := concoctIndexReq(bID[:], wrongTxIndex)

		// expect a storage call for the invalid tx ID but return an error
		txResults := storage.NewTransactionResults(suite.T())
		txResults.On("ByBlockIDTransactionIndex", bID, wrongTxIndex).Return(nil, status.Error(codes.Internal, "")).Once()

		handler := createHandler(txResults)

		_, err := handler.GetTransactionResultByIndex(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.Internal, ""))
	})
}

// TestGetTransactionResultsByBlock tests TestGetTransactionResultsByBlockID API calls
func (suite *Suite) TestGetTransactionResultsByBlockID() {

	totalEvents := 10
	block := unittest.BlockFixture()
	tx := unittest.TransactionFixture()
	bID := block.ID()
	nonexistingBlockID := unittest.IdentifierFixture()
	tx1ID := tx.ID()
	tx2ID := tx.ID()
	//txIndex := rand.Uint32()

	// setup the events storage mock
	eventsForTx1 := make([]flow.Event, totalEvents-3)
	eventsForTx2 := make([]flow.Event, totalEvents-len(eventsForTx1))
	eventsForBlock := make([]flow.Event, totalEvents)

	convertedEventsForTx1 := make([]*entities.Event, len(eventsForTx1))
	convertedEventsForTx2 := make([]*entities.Event, len(eventsForTx2))

	for j := 0; j < len(eventsForTx1); j++ {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(0), uint32(j), tx1ID, 0)
		eventsForTx1[j] = e
		eventsForBlock[j] = e
		convertedEventsForTx1[j] = convert.EventToMessage(e)
	}
	for j := 0; j < len(eventsForTx2); j++ {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(1), uint32(j), tx2ID, 0)
		eventsForTx2[j] = e
		eventsForBlock[len(eventsForTx1)+j] = e
		convertedEventsForTx2[j] = convert.EventToMessage(e)
	}

	// create the handler
	createHandler := func(txResults *storage.TransactionResults) *handler {
		handler := &handler{
			headers:            suite.headers,
			events:             suite.events,
			transactionResults: txResults,
			commits:            suite.commits,
			chain:              flow.Mainnet,
		}
		return handler
	}

	// concoctReq creates a GetTransactionResultsByBlockIDRequest
	concoctReq := func(bID []byte) *execution.GetTransactionsByBlockIDRequest {
		return &execution.GetTransactionsByBlockIDRequest{
			BlockId: bID,
		}
	}

	assertEqual := func(expected, actual *execution.GetTransactionResultsResponse) {

		suite.Require().Len(expected.TransactionResults, len(actual.TransactionResults))

		for i, txResult := range actual.TransactionResults {
			suite.Require().Equal(txResult.GetStatusCode(), actual.TransactionResults[i].GetStatusCode())
			suite.Require().Equal(txResult.GetErrorMessage(), actual.TransactionResults[i].GetErrorMessage())
			suite.Require().ElementsMatch(txResult.GetEvents(), actual.TransactionResults[i].GetEvents())
		}
	}

	// happy path - valid requests receives all events for the given transaction
	suite.Run("happy path with valid events and no transaction error", func() {

		suite.commits.On("ByBlockID", bID).Return(nil, nil).Once()

		// expect a call to lookup events by block ID and transaction ID
		suite.events.On("ByBlockID", bID).Return(eventsForBlock, nil).Once()

		// create the expected result
		expectedResult := &execution.GetTransactionResultsResponse{
			TransactionResults: []*execution.GetTransactionResultResponse{
				{
					StatusCode:   0,
					ErrorMessage: "",
					Events:       convertedEventsForTx1,
				},
				{
					StatusCode:   0,
					ErrorMessage: "",
					Events:       convertedEventsForTx1,
				},
			},
		}

		// expect a call to lookup transaction result by block ID return a result with no error
		txResultsMock := storage.NewTransactionResults(suite.T())
		txResults := []flow.TransactionResult{
			{
				TransactionID: tx1ID,
				ErrorMessage:  "",
			},
			{
				TransactionID: tx2ID,
				ErrorMessage:  "",
			},
		}
		txResultsMock.On("ByBlockID", bID).Return(txResults, nil).Once()

		handler := createHandler(txResultsMock)

		// create a valid API request
		req := concoctReq(bID[:])

		// execute the GetTransactionResult call
		actualResult, err := handler.GetTransactionResultsByBlockID(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		// check that all fields in response are as expected
		assertEqual(expectedResult, actualResult)
	})

	// happy path - valid requests receives all events and an error for the given transaction
	suite.Run("happy path with valid events and a transaction error", func() {

		suite.commits.On("ByBlockID", bID).Return(nil, nil).Once()

		// expect a call to lookup events by block ID and transaction ID
		suite.events.On("ByBlockID", bID).Return(eventsForBlock, nil).Once()

		// create the expected result
		expectedResult := &execution.GetTransactionResultsResponse{
			TransactionResults: []*execution.GetTransactionResultResponse{
				{
					StatusCode:   0,
					ErrorMessage: "",
					Events:       convertedEventsForTx1,
				},
				{
					StatusCode:   1,
					ErrorMessage: "runtime error",
					Events:       convertedEventsForTx2,
				},
			},
		}

		// expect a call to lookup transaction result by block ID return a result with no error
		txResultsMock := storage.NewTransactionResults(suite.T())
		txResults := []flow.TransactionResult{
			{
				TransactionID: tx1ID,
				ErrorMessage:  "",
			},
			{
				TransactionID: tx2ID,
				ErrorMessage:  "runtime error",
			},
		}
		txResultsMock.On("ByBlockID", bID).Return(txResults, nil).Once()

		handler := createHandler(txResultsMock)

		// create a valid API request
		req := concoctReq(bID[:])

		// execute the GetTransactionResult call
		actualResult, err := handler.GetTransactionResultsByBlockID(context.Background(), req)

		// check that a successful response is received
		suite.Require().NoError(err)

		// check that all fields in response are as expected
		assertEqual(expectedResult, actualResult)
	})

	// failure path - nil block id in the request results in an error
	suite.Run("request with nil block ID", func() {

		// create an API request with a nil block id
		req := concoctReq(nil)

		txResults := storage.NewTransactionResults(suite.T())
		handler := createHandler(txResults)

		_, err := handler.GetTransactionResultsByBlockID(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.InvalidArgument, ""))
	})

	// failure path - nonexisting block id in the request results in not found error
	suite.Run("request with nonexisting block ID", func() {

		suite.commits.On("ByBlockID", nonexistingBlockID).Return(nil, realstorage.ErrNotFound).Once()

		txResultsMock := storage.NewTransactionResults(suite.T())
		handler := createHandler(txResultsMock)

		// create a valid API request
		req := concoctReq(nonexistingBlockID[:])

		// execute the GetTransactionResult call
		_, err := handler.GetTransactionResultsByBlockID(context.Background(), req)

		// check that an error was received
		suite.Require().Error(err)
		errors.Is(err, status.Error(codes.NotFound, ""))
	})
}
