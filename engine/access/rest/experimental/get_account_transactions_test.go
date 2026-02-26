package experimental_test

import (
	"encoding/base64"
	"encoding/json"
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

	extendedmock "github.com/onflow/flow-go/access/backends/extended/mock"
	"github.com/onflow/flow-go/engine/access/rest/router"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/utils/unittest"
)

func accountTransactionsURL(t *testing.T, address string, limit string, cursor string) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/accounts/%s/transactions", address))
	require.NoError(t, err)
	q := u.Query()
	if limit != "" {
		q.Add("limit", limit)
	}
	if cursor != "" {
		q.Add("cursor", cursor)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// testEncodeCursor encodes a cursor the same way the handler does, for use in test assertions and inputs.
func testEncodeCursor(height uint64, txIndex uint32) string {
	data, _ := json.Marshal(struct {
		BlockHeight      uint64 `json:"h"`
		TransactionIndex uint32 `json:"i"`
	}{height, txIndex})
	return base64.RawURLEncoding.EncodeToString(data)
}

func TestGetAccountTransactions(t *testing.T) {
	address := unittest.AddressFixture()
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{
					Address:          address,
					BlockHeight:      1000,
					BlockTimestamp:   1700000000000,
					TransactionID:    txID1,
					TransactionIndex: 3,
					Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer},
				},
				{
					Address:          address,
					BlockHeight:      999,
					BlockTimestamp:   1699999000000,
					TransactionID:    txID2,
					TransactionIndex: 0,
					Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleInteracted},
				},
			},
			NextCursor: &accessmodel.AccountTransactionCursor{
				BlockHeight:      999,
				TransactionIndex: 0,
			},
		}

		backend.On("GetAccountTransactions",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.AccountTransactionCursor)(nil),
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything,
		).Return(page, nil)

		reqURL := accountTransactionsURL(t, address.String(), "", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		ts1 := time.UnixMilli(1700000000000).UTC().Format(time.RFC3339Nano)
		ts2 := time.UnixMilli(1699999000000).UTC().Format(time.RFC3339Nano)
		expectedCursorStr := testEncodeCursor(999, 0)
		expected := fmt.Sprintf(`{
			"transactions": [
				{
					"block_height": "1000",
					"timestamp": "%s",
					"transaction_id": "%s",
					"transaction_index": "3",
					"roles": ["authorizer"]
				},
				{
					"block_height": "999",
					"timestamp": "%s",
					"transaction_id": "%s",
					"transaction_index": "0",
					"roles": ["interacted"]
				}
			],
			"next_cursor": "%s"
		}`, ts1, txID1.String(), ts2, txID2.String(), expectedCursorStr)

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{
					Address:          address,
					BlockHeight:      500,
					BlockTimestamp:   1698000000000,
					TransactionID:    txID1,
					TransactionIndex: 1,
					Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer},
				},
			},
			NextCursor: nil,
		}

		backend.On("GetAccountTransactions",
			mocktestify.Anything,
			address,
			uint32(10),
			(*accessmodel.AccountTransactionCursor)(nil),
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything,
		).Return(page, nil)

		reqURL := accountTransactionsURL(t, address.String(), "10", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		ts := time.UnixMilli(1698000000000).UTC().Format(time.RFC3339Nano)
		expected := fmt.Sprintf(`{
			"transactions": [
				{
					"block_height": "500",
					"timestamp": "%s",
					"transaction_id": "%s",
					"transaction_index": "1",
					"roles": ["authorizer"]
				}
			]
		}`, ts, txID1.String())

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{
					Address:          address,
					BlockHeight:      900,
					TransactionID:    txID1,
					TransactionIndex: 2,
					Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleInteracted},
				},
			},
			NextCursor: nil,
		}

		expectedCursor := &accessmodel.AccountTransactionCursor{
			BlockHeight:      1000,
			TransactionIndex: 3,
		}

		backend.On("GetAccountTransactions",
			mocktestify.Anything,
			address,
			uint32(0),
			expectedCursor,
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything,
		).Return(page, nil)

		reqURL := accountTransactionsURL(t, address.String(), "", testEncodeCursor(1000, 3))
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid address", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountTransactionsURL(t, "invalid", "", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid address")
	})

	t.Run("invalid cursor format", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountTransactionsURL(t, address.String(), "", "badcursor")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("backend returns not found", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetAccountTransactions",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.AccountTransactionCursor)(nil),
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything,
		).Return(nil, status.Errorf(codes.NotFound, "no transactions found for account %s", address))

		reqURL := accountTransactionsURL(t, address.String(), "", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetAccountTransactions",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.AccountTransactionCursor)(nil),
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		reqURL := accountTransactionsURL(t, address.String(), "", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})

	t.Run("invalid limit", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountTransactionsURL(t, address.String(), "abc", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid limit")
	})
}

func TestParseCursorRoundtrip(t *testing.T) {
	// Verify that addresses are properly validated
	t.Run("address with 0x prefix works", func(t *testing.T) {
		address := unittest.AddressFixture()
		txID := unittest.IdentifierFixture()
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{
					Address:          address,
					BlockHeight:      100,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer},
				},
			},
		}

		backend.On("GetAccountTransactions",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.AccountTransactionCursor)(nil),
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything,
		).Return(page, nil)

		// Use 0x prefix
		reqURL := accountTransactionsURL(t, "0x"+address.String(), "", "")
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
