package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	mockcommonmodels "github.com/onflow/flow-go/engine/access/rest/common/models/mock"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func getTransactionReq(id string, expandResult bool, blockIdQuery string, collectionIdQuery string) *http.Request {
	u, _ := url.Parse(fmt.Sprintf("/v1/transactions/%s", id))
	q := u.Query()

	if expandResult {
		// by default expand all since we test expanding with converters
		q.Add("expand", "result")
	}

	if blockIdQuery != "" {
		q.Add("block_id", blockIdQuery)
	}

	if collectionIdQuery != "" {
		q.Add("collection_id", collectionIdQuery)
	}

	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

func transactionResultURL(
	txID string,
	blockID string,
	collectionID string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) string {
	u, _ := url.Parse(fmt.Sprintf("/v1/transaction_results/%s", txID))
	q := u.Query()
	if blockID != "" {
		q.Add("block_id", blockID)
	}

	if collectionID != "" {
		q.Add("collection_id", collectionID)
	}

	if len(agreeingExecutorsCount) > 0 {
		q.Add(router.AgreeingExecutorsCountQueryParam, agreeingExecutorsCount)
	}

	if len(requiredExecutors) > 0 {
		q.Add(router.RequiredExecutorIdsQueryParam, strings.Join(requiredExecutors, ","))
	}

	if len(includeExecutorMetadata) > 0 {
		q.Add(router.IncludeExecutorMetadataQueryParam, includeExecutorMetadata)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// getTransactionResultReq builds an HTTP GET request for fetching a transaction result with optional query parameters.
func getTransactionResultReq(
	t *testing.T,
	txID string,
	blockID string,
	collectionID string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) *http.Request {
	req, err := http.NewRequest(
		"GET",
		transactionResultURL(txID, blockID, collectionID, agreeingExecutorsCount, requiredExecutors, includeExecutorMetadata),
		nil)
	require.NoError(t, err)

	return req
}

func createTransactionReq(body interface{}) *http.Request {
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/transactions", bytes.NewBuffer(jsonBody))
	return req
}

func TestGetTransactions(t *testing.T) {
	t.Run("get by ID without results", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		tx := unittest.TransactionFixture()
		req := getTransactionReq(tx.ID().String(), false, "", "")

		backend.On("GetTransaction", mock.Anything, tx.ID()).
			Return(&tx, nil).
			Once()

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"YWNjZXNzKGFsbCkgZnVuIG1haW4oKSB7fQ==",
              "arguments": [],
			   "reference_block_id":"%s",
			   "gas_limit":"10",
			   "payer":"8c5303eaa26202d6",
			   "proposal_key":{
				  "address":"8c5303eaa26202d6",
				  "key_index":"1",
				  "sequence_number":"0"
			   },
			   "authorizers":[
				  "8c5303eaa26202d6"
			   ],
              "payload_signatures": [],
			   "envelope_signatures":[
				  {
					 "address":"8c5303eaa26202d6",
					 "key_index":"1",
					 "signature":"%s"
				  }
			   ],
			   "_links":{
				  "_self":"/v1/transactions/%s"
			   },
				"_expandable": {
					"result": "/v1/transaction_results/%s"
				}
			}`,
			tx.ID(), tx.ReferenceBlockID, util.ToBase64(tx.EnvelopeSignatures[0].Signature), tx.ID(), tx.ID())

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("Get by ID with results", func(t *testing.T) {
		backend := accessmock.NewAPI(t)

		tx := unittest.TransactionBodyFixture()
		txID := tx.ID()
		txr := transactionResultFixture(tx, flow.TransactionStatusSealed, 0, "")

		backend.On("GetTransaction", mock.Anything, txID).
			Return(&tx, nil).
			Once()
		backend.On("GetTransactionResult", mock.Anything, txID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
			Return(txr, nil, nil).
			Once()

		req := getTransactionReq(txID.String(), true, "", "")

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"YWNjZXNzKGFsbCkgZnVuIG1haW4oKSB7fQ==",
              "arguments": [],
			   "reference_block_id":"%s",
			   "gas_limit":"10",
			   "payer":"8c5303eaa26202d6",
			   "proposal_key":{
				  "address":"8c5303eaa26202d6",
				  "key_index":"1",
				  "sequence_number":"0"
			   },
			   "authorizers":[
				  "8c5303eaa26202d6"
			   ],
              "payload_signatures": [],
			   "envelope_signatures":[
				  {
					 "address":"8c5303eaa26202d6",
					 "key_index":"1",
					 "signature":"%s"
				  }
			   ],
              "result": %s,
              "_expandable": {},
			   "_links":{
				  "_self":"/v1/transactions/%s"
			   }
			}`,
			tx.ID(),
			tx.ReferenceBlockID,
			util.ToBase64(tx.EnvelopeSignatures[0].Signature),
			expectedTransactionResultResponse(txr, models.SUCCESS_RESULT, nil),
			tx.ID(),
		)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		backend := accessmock.NewAPI(t)

		req := getTransactionReq("invalid", false, "", "")
		expected := `{"code":400, "message":"invalid ID format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by ID non-existing", func(t *testing.T) {
		backend := accessmock.NewAPI(t)

		tx := unittest.TransactionFixture()
		req := getTransactionReq(tx.ID().String(), false, "", "")

		backend.On("GetTransaction", mock.Anything, tx.ID()).
			Return(nil, status.Error(codes.NotFound, "transaction not found")).
			Once()

		expected := `{"code":404, "message":"Flow resource not found: transaction not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})
}

// TestGetTransactionResult verifies the get transaction result request
// for different lookup modes and optional executor metadata.
//
// Test cases:
//  1. Get transaction result by transaction ID with transaction ID.
//  2. Get transaction result by transaction ID with a block ID.
//  3. Get transaction result by transaction ID with a collection ID.
//  4. Get transaction result by transaction ID with executor metadata included.
//  5. Verify execution and status mapping for all supported transaction states.
func TestGetTransactionResult(t *testing.T) {
	tx := unittest.TransactionFixture()
	txr := transactionResultFixture(tx, flow.TransactionStatusSealed, 0, "")

	txID := txr.TransactionID
	blockID := txr.BlockID
	collectionID := txr.CollectionID

	t.Run("get by transaction ID", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		req := getTransactionResultReq(t, txID.String(), "", "", "", []string{}, "false")

		backend.On("GetTransactionResult", mock.Anything, txID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
			Return(txr, nil, nil).
			Once()

		expected := expectedTransactionResultResponse(txr, models.SUCCESS_RESULT, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by block ID", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		req := getTransactionResultReq(t, txID.String(), blockID.String(), "", "", []string{}, "false")

		backend.On("GetTransactionResult", mock.Anything, txID, blockID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
			Return(txr, nil, nil).
			Once()

		expected := expectedTransactionResultResponse(txr, models.SUCCESS_RESULT, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by collection ID", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		req := getTransactionResultReq(t, txID.String(), "", collectionID.String(), "", []string{}, "false")

		backend.On("GetTransactionResult", mock.Anything, txID, flow.ZeroID, collectionID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
			Return(txr, nil, nil).
			Once()

		expected := expectedTransactionResultResponse(txr, models.SUCCESS_RESULT, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by transaction ID with metadata", func(t *testing.T) {
		backend := accessmock.NewAPI(t)

		metadata := &accessmodel.ExecutorMetadata{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutorIDs:       unittest.IdentifierListFixture(2),
		}

		criteria := optimistic_sync.DefaultCriteria
		req := getTransactionResultReq(t, txID.String(), "", "", fmt.Sprintf("%d", criteria.AgreeingExecutorsCount), []string{}, "true")

		backend.On("GetTransactionResult", mock.Anything, txID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, criteria).
			Return(txr, metadata, nil).
			Once()

		expected := expectedTransactionResultResponse(txr, models.SUCCESS_RESULT, metadata)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get execution statuses", func(t *testing.T) {
		backend := accessmock.NewAPI(t)

		type testCase struct {
			name         string
			status       flow.TransactionStatus
			statusCode   uint
			errorMessage string
			execution    models.TransactionExecution
		}

		testCases := []testCase{
			{
				name:         "expired transaction",
				status:       flow.TransactionStatusExpired,
				statusCode:   0,
				errorMessage: "",
				execution:    models.FAILURE_RESULT,
			},
			{
				name:         "sealed transaction with execution error",
				status:       flow.TransactionStatusSealed,
				statusCode:   1,
				errorMessage: "cadence runtime exception",
				execution:    models.FAILURE_RESULT,
			},
			{
				name:         "finalized transaction",
				status:       flow.TransactionStatusFinalized,
				statusCode:   0,
				errorMessage: "",
				execution:    models.PENDING_RESULT,
			},
			{
				name:         "pending transaction",
				status:       flow.TransactionStatusPending,
				statusCode:   0,
				errorMessage: "",
				execution:    models.PENDING_RESULT,
			},
			{
				name:         "executed transaction",
				status:       flow.TransactionStatusExecuted,
				statusCode:   0,
				errorMessage: "",
				execution:    models.PENDING_RESULT,
			},
			{
				name:         "sealed successful transaction",
				status:       flow.TransactionStatusSealed,
				statusCode:   0,
				errorMessage: "",
				execution:    models.SUCCESS_RESULT,
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				txResult := transactionResultFixture(
					tx,
					test.status,
					test.statusCode,
					test.errorMessage,
				)
				req := getTransactionResultReq(t, txResult.TransactionID.String(), "", "", "", []string{}, "false")
				backend.On("GetTransactionResult", mock.Anything, txResult.TransactionID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
					Return(txResult, nil, nil).
					Once()

				expectedResp := expectedTransactionResultResponse(txResult, test.execution, nil)
				router.AssertOKResponse(t, req, expectedResp, backend)
			})
		}
	})
}

// TestGetTransactionResultErrors verifies that the GetTransactionResult endpoint
// correctly returns appropriate HTTP error codes and messages in various failure scenarios.
//
// Test cases:
//  1. A request with an invalid account address returns http.StatusBadRequest.
//  2. Simulated failure when fetching GetTransactionResult from backend
//     to ensure that any propagated errors are correctly returned.
//
// TODO(): These tests (test case 2 - error type) should be updated when error handling will be added.
func TestGetTransactionResultErrors(t *testing.T) {
	backend := accessmock.NewAPI(t)

	tx := unittest.TransactionFixture()
	txr := transactionResultFixture(tx, flow.TransactionStatusSealed, 0, "")
	txID := txr.TransactionID

	tests := []struct {
		name   string
		url    string
		setup  func()
		status int
		out    string
	}{
		{
			name:   "invalid transaction ID",
			url:    transactionResultURL("invalidID", "", "", "", []string{}, "false"),
			setup:  func() {},
			status: http.StatusBadRequest,
			out:    `{"code":400, "message":"invalid ID format"}`,
		},
		{
			name: "GetTransactionResult fails",
			url:  transactionResultURL(txID.String(), "", "", "", []string{}, "false"),
			setup: func() {

				backend.On("GetTransactionResult", mock.Anything, txID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
					Return(nil, nil, fmt.Errorf("backend error")).
					Once()
			},
			status: http.StatusInternalServerError,
			out:    `{"code":500, "message":"internal server error"}`,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup()
			req, _ := http.NewRequest("GET", test.url, nil)
			rr := router.ExecuteRequest(req, backend)

			require.Equal(t, test.status, rr.Code, fmt.Sprintf("test #%d failed: %v", i, test))
			require.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		})
	}
}

func TestGetScheduledTransactions(t *testing.T) {
	g := fixtures.NewGeneratorSuite()

	scheduledTxID := uint64(42)
	tx := scheduledTransactionFixture(t, g, scheduledTxID)
	txID := tx.ID()
	txr := transactionResultFixture(*tx, flow.TransactionStatusSealed, 0, "")

	link := mockcommonmodels.NewLinkGenerator(t)
	link.On("TransactionLink", txID).Return(fmt.Sprintf("/v1/transactions/%s", txID), nil)
	link.On("TransactionResultLink", txID).Return(fmt.Sprintf("/v1/transaction_results/%s", txID), nil)

	expectedWithoutResult := expectedTransactionResponse(t, tx, nil, link)
	expectedWithResult := expectedTransactionResponse(t, tx, txr, link)

	t.Run("get by scheduled transaction ID without results", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		backend.On("GetScheduledTransaction", mock.Anything, scheduledTxID).
			Return(tx, nil).
			Once()

		req := getTransactionReq(fmt.Sprint(scheduledTxID), false, "", "")
		router.AssertOKResponse(t, req, expectedWithoutResult, backend)
	})

	t.Run("get by scheduled transaction ID with results", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		backend.On("GetScheduledTransaction", mock.Anything, scheduledTxID).
			Return(tx, nil).
			Once()
		backend.On("GetScheduledTransactionResult", mock.Anything, scheduledTxID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil).
			Once()

		req := getTransactionReq(fmt.Sprint(scheduledTxID), true, "", "")

		router.AssertOKResponse(t, req, expectedWithResult, backend)
	})

	t.Run("get result by scheduled transaction ID", func(t *testing.T) {
		expectedTxResult := models.NewTransactionResult(txr, txID, link, nil, false)
		expectedResult, err := json.Marshal(expectedTxResult)
		require.NoError(t, err)

		backend := accessmock.NewAPI(t)
		backend.On("GetScheduledTransactionResult", mock.Anything, scheduledTxID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil).
			Once()

		req := getTransactionResultReq(t, fmt.Sprint(scheduledTxID), "", "", "", []string{}, "false")

		router.AssertOKResponse(t, req, string(expectedResult), backend)
	})

	// these are identical to the regular get transaction tests, except the tx body is a scheduled
	// transaction. Scheduled tx bodies contain a subset of the information in regular submitted tx,
	// so this ensures the endpoints return the expected responses.
	t.Run("get by transaction ID without results", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		backend.On("GetTransaction", mock.Anything, txID).
			Return(tx, nil).
			Once()

		req := getTransactionReq(txID.String(), false, "", "")

		router.AssertOKResponse(t, req, string(expectedWithoutResult), backend)
	})

	t.Run("get by transaction ID with results", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		backend.On("GetTransaction", mock.Anything, txID).
			Return(tx, nil).
			Once()
		backend.On("GetTransactionResult", mock.Anything, txID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0, mock.Anything).
			Return(txr, nil, nil).
			Once()

		req := getTransactionReq(txID.String(), true, "", "")
		router.AssertOKResponse(t, req, expectedWithResult, backend)
	})

	t.Run("get by ID non-existing", func(t *testing.T) {
		backend := accessmock.NewAPI(t)
		backend.On("GetScheduledTransaction", mock.Anything, scheduledTxID).
			Return(nil, status.Error(codes.NotFound, "transaction not found")).
			Once()

		req := getTransactionReq(fmt.Sprint(scheduledTxID), false, "", "")

		expected := `{"code":404, "message":"Flow resource not found: transaction not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})
}

func TestCreateTransaction(t *testing.T) {
	backend := accessmock.NewAPI(t)

	t.Run("create", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
		tx.Arguments = [][]uint8{}
		req := createTransactionReq(unittest.CreateSendTxHttpPayload(tx))

		backend.On("SendTransaction", mock.Anything, &tx).
			Return(nil).
			Once()

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"YWNjZXNzKGFsbCkgZnVuIG1haW4oKSB7fQ==",
			   "arguments": [],
			   "reference_block_id":"%s",
			   "gas_limit":"10",
			   "payer":"8c5303eaa26202d6",
			   "proposal_key":{
				  "address":"8c5303eaa26202d6",
				  "key_index":"1",
				  "sequence_number":"0"
			   },
			   "authorizers":[
				  "8c5303eaa26202d6"
			   ],
              "payload_signatures":[
				  {
					 "address":"8c5303eaa26202d6",
					 "key_index":"1",
					 "signature":"%s"
				  }
			   ],
			   "envelope_signatures":[
				  {
					 "address":"8c5303eaa26202d6",
					 "key_index":"1",
					 "signature":"%s"
				  }
			   ],
				"_expandable": {
					"result": "/v1/transaction_results/%s"
				},
			   "_links":{
				  "_self":"/v1/transactions/%s"
			   }
			}`,
			tx.ID(), tx.ReferenceBlockID, util.ToBase64(tx.PayloadSignatures[0].Signature), util.ToBase64(tx.EnvelopeSignatures[0].Signature), tx.ID(), tx.ID())
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("post invalid transaction", func(t *testing.T) {
		tests := []struct {
			inputField string
			inputValue string
			output     string
		}{
			{"reference_block_id", "-1", `{"code":400, "message":"invalid reference block ID: invalid ID format"}`},
			{"reference_block_id", "", `{"code":400, "message":"reference block not provided"}`},
			{"gas_limit", "-1", `{"code":400, "message":"invalid gas limit: value must be an unsigned 64 bit integer"}`},
			{"gas_limit", "18446744073709551616", `{"code":400, "message":"invalid gas limit: value overflows uint64 range"}`},
			{"payer", "yo", `{"code":400, "message":"invalid payer: invalid address"}`},
			{
				"proposal_key",
				"yo",
				`{"code":400, "message":"request body contains an invalid value for the \"proposal_key\" field (at position 461)"}`,
			},
			{
				"authorizers",
				"",
				`{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 32)"}`,
			},
			{
				"authorizers",
				"yo",
				`{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 34)"}`,
			},
			{
				"envelope_signatures",
				"",
				`{"code":400, "message":"request body contains an invalid value for the \"envelope_signatures\" field (at position 75)"}`,
			},
			{
				"payload_signatures",
				"",
				`{"code":400, "message":"request body contains an invalid value for the \"payload_signatures\" field (at position 292)"}`,
			},
		}

		for _, test := range tests {
			tx := unittest.TransactionBodyFixture()
			tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
			testTx := unittest.CreateSendTxHttpPayload(tx)
			testTx[test.inputField] = test.inputValue
			req := createTransactionReq(testTx)

			router.AssertResponse(t, req, http.StatusBadRequest, test.output, backend)
		}
	})
}

// transactionResultFixture constructs the Access API transaction result fixture
func transactionResultFixture(
	tx flow.TransactionBody,
	status flow.TransactionStatus,
	statusCode uint,
	errorMessage string,
) *accessmodel.TransactionResult {
	txID := tx.ID()
	collectionID := unittest.IdentifierFixture()
	return &accessmodel.TransactionResult{
		Status:     status,
		StatusCode: statusCode, // statusCode of 1 indicates an error and 0 indicates no error
		Events: []flow.Event{
			unittest.EventFixture(
				unittest.Event.WithTransactionID(txID),
			),
		},
		ErrorMessage:  errorMessage,
		BlockID:       tx.ReferenceBlockID,
		CollectionID:  collectionID,
		TransactionID: txID,
	}
}

// scheduledTransactionFixture constructs a transaction body for a scheduled transaction with the
// given scheduled transaction ID.
func scheduledTransactionFixture(t *testing.T, g *fixtures.GeneratorSuite, scheduledTxID uint64) *flow.TransactionBody {
	pendingEvent := g.PendingExecutionEvents().Fixture(
		fixtures.PendingExecutionEvent.WithID(scheduledTxID),
		fixtures.PendingExecutionEvent.WithPriority(1),
		fixtures.PendingExecutionEvent.WithExecutionEffort(5555),
	)

	scheduledTxs, err := blueprints.ExecuteCallbacksTransactions(g.ChainID().Chain(), []flow.Event{pendingEvent})
	require.NoError(t, err)
	return scheduledTxs[0]
}

// expectedTransactionResponse constructs the expected json transaction response for the given
// transaction body and transaction result.
func expectedTransactionResponse(t *testing.T, tx *flow.TransactionBody, txr *accessmodel.TransactionResult, link commonmodels.LinkGenerator) string {
	var expectedTxWithoutResult models.Transaction
	expectedTxWithoutResult.Build(tx, txr, link)

	expected, err := json.Marshal(expectedTxWithoutResult)
	require.NoError(t, err)

	return string(expected)
}

// expectedTransactionResultResponse returns the expected JSON transaction result response string.
// If metadata is provided, it includes the executor metadata fields.
func expectedTransactionResultResponse(
	txr *accessmodel.TransactionResult,
	execution models.TransactionExecution,
	metadata *accessmodel.ExecutorMetadata,
) string {
	metadataSection := expectedMetadata(metadata)
	eventsSection := expectedEventsResponse(txr.Events)

	return fmt.Sprintf(`{
			"block_id": "%s",
			"collection_id": "%s",
			"execution": "%s",
			"status": "%s",
			"status_code": %d,
			"error_message": "%s",
			"computation_used": "0",
			"events": %s,
			"_links": {
				"_self": "/v1/transaction_results/%s"
			}%s
		}`,
		txr.BlockID.String(),
		txr.CollectionID.String(),
		execution,
		cases.Title(language.English).String(strings.ToLower(txr.Status.String())),
		txr.StatusCode,
		txr.ErrorMessage,
		eventsSection,
		txr.TransactionID.String(),
		metadataSection,
	)
}

// expectedEventsResponse returns the expected JSON events response.
func expectedEventsResponse(events []flow.Event) string {
	if len(events) == 0 {
		return "[]"
	}

	items := make([]string, 0, len(events))
	for _, event := range events {
		items = append(items, fmt.Sprintf(`{
			"type": "%s",
			"transaction_id": "%s",
			"transaction_index": "%d",
			"event_index": "%d",
			"payload": "%s"
		}`,
			event.Type,
			event.TransactionID,
			event.TransactionIndex,
			event.EventIndex,
			util.ToBase64(event.Payload),
		))
	}

	return fmt.Sprintf("[%s]", strings.Join(items, ","))
}
