package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func executeRequest(req *http.Request, backend *mock.API) *httptest.ResponseRecorder {
	var b bytes.Buffer
	logger := zerolog.New(&b)
	router := initRouter(backend, logger)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	return rr
}

func assertOKResponse(t *testing.T, req *http.Request, expectedRespBody string, backend *mock.API) {
	assertResponse(t, req, http.StatusOK, expectedRespBody, backend)
}

func assertResponse(t *testing.T, req *http.Request, status int, expectedRespBody string, backend *mock.API) {
	rr := executeRequest(req, backend)
	require.Equal(t, status, rr.Code)
	actualResponseBody := rr.Body.String()
	require.JSONEq(t, expectedRespBody, actualResponseBody, fmt.Sprintf("failed for req: %s", req.URL))
}

func getCollectionReq(id string, expandTransactions bool) *http.Request {
	url := fmt.Sprintf("/v1/collections/%s", id)
	if expandTransactions {
		url = fmt.Sprintf("%s?expand=transactions", url)
	}

	req, _ := http.NewRequest("GET", url, nil)
	return req
}

func TestGetCollections(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by ID", func(t *testing.T) {
		inputs := []flow.LightCollection{
			unittest.CollectionFixture(1).Light(),
			unittest.CollectionFixture(10).Light(),
			unittest.CollectionFixture(100).Light(),
		}

		for _, col := range inputs {
			backend.Mock.
				On("GetCollectionByID", mocks.Anything, col.ID()).
				Return(&col, nil)

			txs := make([]string, len(col.Transactions))
			for i, tx := range col.Transactions {
				txs[i] = fmt.Sprintf("\"/v1/transactions/%s\"", tx.String())
			}
			transactionsStr := fmt.Sprintf("[%s]", strings.Join(txs, ","))

			expected := fmt.Sprintf(`{
				"id":"%s",	
				"_links": {
					"_self": "/v1/collections/%s"
				},
                "_expandable": {
                   "transactions": %s
                }
			}`, col.ID(), col.ID(), transactionsStr)

			req := getCollectionReq(col.ID().String(), false)
			assertOKResponse(t, req, expected, backend)
		}
	})

	t.Run("get by ID expand transactions", func(t *testing.T) {
		col := unittest.CollectionFixture(3).Light()

		transactions := make([]flow.TransactionBody, len(col.Transactions))
		for i := range col.Transactions {
			transactions[i] = unittest.TransactionBodyFixture()
			col.Transactions[i] = transactions[i].ID() // overwrite tx ids

			backend.Mock.
				On("GetTransaction", mocks.Anything, transactions[i].ID()).
				Return(&transactions[i], nil)
		}

		backend.Mock.
			On("GetCollectionByID", mocks.Anything, col.ID()).
			Return(&col, nil)

		req := getCollectionReq(col.ID().String(), true)
		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		// really hacky but we can't build whole response since it's really complex
		// so we just make sure the transactions are included and have defined values
		// anyhow we already test transaction responses in transaction tests
		var res map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &res)
		assert.NoError(t, err)
		resTx := res["transactions"].([]interface{})
		for i, r := range resTx {
			c := r.(map[string]interface{})
			assert.Equal(t, transactions[i].ID().String(), c["id"])
			assert.NotNil(t, c["envelope_signatures"])
		}
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		testID := unittest.IdentifierFixture()
		tests := []struct {
			id        string
			mockValue *flow.LightCollection
			mockErr   error
			response  string
			status    int
		}{{
			testID.String(),
			nil,
			status.Error(codes.NotFound, "not found"),
			`{"code":404,"message":"not found"}`,
			http.StatusNotFound,
		}, {
			"invalidID",
			nil,
			nil,
			`{"code":400,"message":"invalid ID format"}`,
			http.StatusBadRequest,
		}}

		for _, test := range tests {
			id, err := flow.HexStringToIdentifier(test.id)
			if err == nil {
				// setup the backend mock ti return a not found error if this is a valid id
				backend.Mock.
					On("GetCollectionByID", mocks.Anything, id).
					Return(test.mockValue, test.mockErr)
			}
			req := getCollectionReq(test.id, false)
			assertResponse(t, req, test.status, test.response, backend)
		}

	})

}
