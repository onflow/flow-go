package routes_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	extendedmock "github.com/onflow/flow-go/access/backends/extended/mock"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
	"github.com/onflow/flow-go/engine/access/rest/router"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type scheduledTxURLParams struct {
	limit  string
	cursor string
	status string
	expand string
}

func scheduledTxsURL(t *testing.T, params scheduledTxURLParams) string {
	u, err := url.ParseRequestURI("/experimental/v1/scheduled")
	require.NoError(t, err)
	q := u.Query()
	if params.limit != "" {
		q.Add("limit", params.limit)
	}
	if params.cursor != "" {
		q.Add("cursor", params.cursor)
	}
	if params.status != "" {
		q.Add("status", params.status)
	}
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func scheduledTxByIDURL(t *testing.T, id uint64, params scheduledTxURLParams) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/scheduled/transaction/%d", id))
	require.NoError(t, err)
	if params.expand != "" {
		q := u.Query()
		q.Add("expand", params.expand)
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func scheduledTxsByAddrURL(t *testing.T, address string, params scheduledTxURLParams) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/accounts/%s/scheduled", address))
	require.NoError(t, err)
	q := u.Query()
	if params.limit != "" {
		q.Add("limit", params.limit)
	}
	if params.cursor != "" {
		q.Add("cursor", params.cursor)
	}
	if params.status != "" {
		q.Add("status", params.status)
	}
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// testEncodeScheduledTxCursor encodes a cursor the same way the handler does, for use in
// test assertions and inputs.
func testEncodeScheduledTxCursor(t *testing.T, id uint64) string {
	data, err := request.EncodeScheduledTxCursor(&accessmodel.ScheduledTransactionCursor{ID: id})
	require.NoError(t, err)
	return data
}

func TestGetScheduledTransactions(t *testing.T) {
	handlerOwner := unittest.AddressFixture()

	tx1CreatedAt := uint64(1700000000000) // Unix ms
	tx1CreatedID := unittest.IdentifierFixture()
	tx1 := accessmodel.ScheduledTransaction{
		ID:                               100,
		Priority:                         0, // high
		Timestamp:                        1000000,
		ExecutionEffort:                  500,
		Fees:                             250,
		TransactionHandlerOwner:          handlerOwner,
		TransactionHandlerTypeIdentifier: "A.0000.MyScheduler.Handler",
		TransactionHandlerUUID:           7,
		Status:                           accessmodel.ScheduledTxStatusScheduled,
		CreatedTransactionID:             tx1CreatedID,
		CreatedAt:                        tx1CreatedAt,
	}
	tx2CreatedAt := uint64(1699999000000)   // Unix ms
	tx2CompletedAt := uint64(1700001000000) // Unix ms
	tx2CreatedID := unittest.IdentifierFixture()
	tx2ExecutedID := unittest.IdentifierFixture()
	tx2 := accessmodel.ScheduledTransaction{
		ID:                               99,
		Priority:                         1, // medium
		Timestamp:                        999000,
		ExecutionEffort:                  200,
		Fees:                             100,
		TransactionHandlerOwner:          handlerOwner,
		TransactionHandlerTypeIdentifier: "A.0000.MyScheduler.Handler",
		TransactionHandlerUUID:           8,
		Status:                           accessmodel.ScheduledTxStatusExecuted,
		CreatedTransactionID:             tx2CreatedID,
		ExecutedTransactionID:            tx2ExecutedID,
		CreatedAt:                        tx2CreatedAt,
		CompletedAt:                      tx2CompletedAt,
	}

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx1, tx2},
			NextCursor:   &accessmodel.ScheduledTransactionCursor{ID: 99},
		}

		backend.On("GetScheduledTransactions",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		expectedNextCursor := testEncodeScheduledTxCursor(t, 99)
		tx1CreatedAtStr := time.UnixMilli(int64(tx1CreatedAt)).UTC().Format(time.RFC3339Nano)
		tx2CreatedAtStr := time.UnixMilli(int64(tx2CreatedAt)).UTC().Format(time.RFC3339Nano)
		tx2CompletedAtStr := time.UnixMilli(int64(tx2CompletedAt)).UTC().Format(time.RFC3339Nano)
		expected := fmt.Sprintf(`{
			"scheduled_transactions": [
				{
					"id": "100",
					"status": "scheduled",
					"priority": "high",
					"timestamp": "1000000",
					"execution_effort": "500",
					"fees": "250",
					"transaction_handler_owner": "%s",
					"transaction_handler_type_identifier": "A.0000.MyScheduler.Handler",
					"transaction_handler_uuid": "7",
					"created_transaction_id": "%s",
					"created_at": "%s",
					"_expandable": {
						"handler_contract": "/experimental/v1/contracts/A.0000.MyScheduler?expand=code"
					}
				},
				{
					"id": "99",
					"status": "executed",
					"priority": "medium",
					"timestamp": "999000",
					"execution_effort": "200",
					"fees": "100",
					"transaction_handler_owner": "%s",
					"transaction_handler_type_identifier": "A.0000.MyScheduler.Handler",
					"transaction_handler_uuid": "8",
					"created_transaction_id": "%s",
					"executed_transaction_id": "%s",
					"created_at": "%s",
					"completed_at": "%s",
					"_expandable": {
						"handler_contract": "/experimental/v1/contracts/A.0000.MyScheduler?expand=code",
						"transaction": "/v1/transactions/%s",
						"result": "/v1/transaction_results/%s"
					}
				}
			],
			"next_cursor": "%s"
		}`, handlerOwner.String(), tx1CreatedID.String(), tx1CreatedAtStr,
			handlerOwner.String(), tx2CreatedID.String(), tx2ExecutedID.String(), tx2CreatedAtStr, tx2CompletedAtStr,
			tx2ExecutedID.String(), tx2ExecutedID.String(), expectedNextCursor)

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx1},
		}

		backend.On("GetScheduledTransactions",
			mocktestify.Anything,
			uint32(10),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{limit: "10"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), "next_cursor")
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx2},
		}

		backend.On("GetScheduledTransactions",
			mocktestify.Anything,
			uint32(0),
			&accessmodel.ScheduledTransactionCursor{ID: 100},
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{
			cursor: testEncodeScheduledTxCursor(t, 100),
		}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with status filter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{},
		}

		backend.On("GetScheduledTransactions",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{
				Statuses: []accessmodel.ScheduledTransactionStatus{accessmodel.ScheduledTxStatusScheduled},
			},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{status: "scheduled"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid limit", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{limit: "abc"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid limit")
	})

	t.Run("invalid cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{cursor: "!notbase64!"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("invalid status", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{status: "unknown"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid status")
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetScheduledTransactions",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		req, err := http.NewRequest(http.MethodGet, scheduledTxsURL(t, scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})
}

func TestGetScheduledTransaction(t *testing.T) {
	handlerOwner := unittest.AddressFixture()

	txCreatedID := unittest.IdentifierFixture()
	tx := &accessmodel.ScheduledTransaction{
		ID:                               42,
		Priority:                         0, // high
		Timestamp:                        2000000,
		ExecutionEffort:                  750,
		Fees:                             300,
		TransactionHandlerOwner:          handlerOwner,
		TransactionHandlerTypeIdentifier: "A.0000.MyScheduler.Handler",
		TransactionHandlerUUID:           3,
		Status:                           accessmodel.ScheduledTxStatusScheduled,
		CreatedTransactionID:             txCreatedID,
	}

	t.Run("happy path", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetScheduledTransaction",
			mocktestify.Anything,
			uint64(42),
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(tx, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxByIDURL(t, 42, scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		expected := fmt.Sprintf(`{
			"id": "42",
			"status": "scheduled",
			"priority": "high",
			"timestamp": "2000000",
			"execution_effort": "750",
			"fees": "300",
			"transaction_handler_owner": "%s",
			"transaction_handler_type_identifier": "A.0000.MyScheduler.Handler",
			"transaction_handler_uuid": "3",
			"created_transaction_id": "%s",
			"_expandable": {
				"handler_contract": "/experimental/v1/contracts/A.0000.MyScheduler?expand=code"
			}
		}`, handlerOwner.String(), txCreatedID.String())

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("invalid ID - non-numeric", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, "/experimental/v1/scheduled/transaction/notanumber", nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid scheduled transaction ID")
	})

	t.Run("with handler_contract expand", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		txWithContract := &accessmodel.ScheduledTransaction{
			ID:                               42,
			Priority:                         0,
			Timestamp:                        2000000,
			ExecutionEffort:                  750,
			Fees:                             300,
			TransactionHandlerOwner:          handlerOwner,
			TransactionHandlerTypeIdentifier: "A.0000.MyScheduler.Handler",
			TransactionHandlerUUID:           3,
			Status:                           accessmodel.ScheduledTxStatusScheduled,
			CreatedTransactionID:             txCreatedID,
			HandlerContract: &accessmodel.ContractDeployment{
				Address:      flow.HexToAddress("0000"),
				ContractName: "MyScheduler",
				Code:         []byte("pub contract MyScheduler {}"),
				CodeHash:     accessmodel.CadenceCodeHash([]byte("pub contract MyScheduler {}")),
			},
		}

		backend.On("GetScheduledTransaction",
			mocktestify.Anything,
			uint64(42),
			extended.ScheduledTransactionExpandOptions{HandlerContract: true},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(txWithContract, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxByIDURL(t, 42, scheduledTxURLParams{expand: "handler_contract"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		expected := fmt.Sprintf(`{
			"id": "42",
			"status": "scheduled",
			"priority": "high",
			"timestamp": "2000000",
			"execution_effort": "750",
			"fees": "300",
			"transaction_handler_owner": "%s",
			"transaction_handler_type_identifier": "A.0000.MyScheduler.Handler",
			"transaction_handler_uuid": "3",
			"created_transaction_id": "%s",
			"handler_contract": {
				"contract_id": "A.0000000000000000.MyScheduler",
				"address": "0000000000000000",
				"block_height": "0",
				"transaction_id": "0000000000000000000000000000000000000000000000000000000000000000",
				"tx_index": "0",
				"event_index": "0",
				"code": "cHViIGNvbnRyYWN0IE15U2NoZWR1bGVyIHt9",
				"code_hash": "383198cd7e974ca055c4137bdd1fa44934882f569e7f0c353254e0e7ce8a50fb",
				"_expandable": {
					"transaction": "/v1/transactions/0000000000000000000000000000000000000000000000000000000000000000",
					"result": "/v1/transaction_results/0000000000000000000000000000000000000000000000000000000000000000"
				}
			},
			"_expandable": {}
		}`, handlerOwner.String(), txCreatedID.String())

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("backend returns not found", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetScheduledTransaction",
			mocktestify.Anything,
			uint64(999),
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.NotFound, "scheduled transaction 999 not found"))

		req, err := http.NewRequest(http.MethodGet, scheduledTxByIDURL(t, 999, scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestGetScheduledTransactionsByAddress(t *testing.T) {
	address := unittest.AddressFixture()
	handlerOwner := unittest.AddressFixture()

	tx1CreatedID := unittest.IdentifierFixture()
	tx1 := accessmodel.ScheduledTransaction{
		ID:                               50,
		Priority:                         0, // high
		Timestamp:                        5000000,
		ExecutionEffort:                  300,
		Fees:                             150,
		TransactionHandlerOwner:          handlerOwner,
		TransactionHandlerTypeIdentifier: "A.0000.MyScheduler.Handler",
		TransactionHandlerUUID:           5,
		Status:                           accessmodel.ScheduledTxStatusScheduled,
		CreatedTransactionID:             tx1CreatedID,
	}
	tx2CreatedID := unittest.IdentifierFixture()
	tx2CancelledID := unittest.IdentifierFixture()
	tx2 := accessmodel.ScheduledTransaction{
		ID:                               49,
		Priority:                         2, // low
		Timestamp:                        4500000,
		ExecutionEffort:                  100,
		Fees:                             50,
		TransactionHandlerOwner:          handlerOwner,
		TransactionHandlerTypeIdentifier: "A.0000.MyScheduler.Handler",
		TransactionHandlerUUID:           6,
		Status:                           accessmodel.ScheduledTxStatusCancelled,
		CreatedTransactionID:             tx2CreatedID,
		CancelledTransactionID:           tx2CancelledID,
	}

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx1, tx2},
			NextCursor:   &accessmodel.ScheduledTransactionCursor{ID: 49},
		}

		backend.On("GetScheduledTransactionsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, address.String(), scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), testEncodeScheduledTxCursor(t, 49))
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx1},
		}

		backend.On("GetScheduledTransactionsByAddress",
			mocktestify.Anything,
			address,
			uint32(5),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, address.String(), scheduledTxURLParams{limit: "5"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), "next_cursor")
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx2},
		}

		backend.On("GetScheduledTransactionsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			&accessmodel.ScheduledTransactionCursor{ID: 50},
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, address.String(), scheduledTxURLParams{
			cursor: testEncodeScheduledTxCursor(t, 50),
		}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("address with 0x prefix", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{tx1},
		}

		backend.On("GetScheduledTransactionsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, "0x"+address.String(), scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid address", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, "invalid", scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid address")
	})

	t.Run("invalid cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, address.String(), scheduledTxURLParams{cursor: "badcursor"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("backend returns not found", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetScheduledTransactionsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.NotFound, "no transactions for address %s", address))

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, address.String(), scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetScheduledTransactionsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ScheduledTransactionCursor)(nil),
			extended.ScheduledTransactionFilter{},
			extended.ScheduledTransactionExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		req, err := http.NewRequest(http.MethodGet, scheduledTxsByAddrURL(t, address.String(), scheduledTxURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})
}
