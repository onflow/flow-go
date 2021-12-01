package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"net/http"
	"net/http/httptest"
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
	rr := executeRequest(req, backend)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, expectedRespBody, rr.Body.String())
}

func collectionURL(param string) string {
	return fmt.Sprintf("/v1/collections/%s", param)
}

func TestGetCollections(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by ID", func(t *testing.T) {
		inputs := []flow.LightCollection{
			unittest.CollectionFixture(0).Light(),
			unittest.CollectionFixture(3).Light(),
			unittest.CollectionFixture(100).Light(),
		}

		for _, col := range inputs {
			req, _ := http.NewRequest("GET", collectionURL(col.ID().String()), nil)

			backend.Mock.
				On("GetCollectionByID", mocks.Anything, col.ID()).
				Return(&col, nil)

			rr := executeRequest(req, backend)

			txs := make([]generated.Transaction, len(col.Transactions))
			for i, tx := range col.Transactions {
				txs[i] = generated.Transaction{Id: tx.String()}
			}
			bx, _ := json.Marshal(txs)

			expected := fmt.Sprintf(`{
				"id":"%s",
				"transactions":%s, 	
				"_links": {
					"_self": "/v1/collections/%s"
				}
			}`, col.ID().String(), bx, col.ID())

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.JSONEq(t, expected, rr.Body.String())
		}
	})

	t.Run("get by ID expand transactions", func(t *testing.T) {
		col := unittest.CollectionFixture(3).Light()

		req, _ := http.NewRequest(
			"GET",
			fmt.Sprintf(
				"%s?expand=transactions",
				collectionURL(col.ID().String()),
			),
			nil,
		)

		backend.Mock.
			On("GetCollectionByID", mocks.Anything, col.ID()).
			Return(&col, nil)

		transactions := make([]flow.TransactionBody, len(col.Transactions))
		for i := range col.Transactions {
			transactions[i] = unittest.TransactionBodyFixture()
			col.Transactions[i] = transactions[i].ID() // overwrite tx ids

			backend.Mock.
				On("GetTransaction", mocks.Anything, transactions[i].ID()).
				Return(&transactions[i], nil)
		}

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
		tests := []struct {
			id        string
			mockValue *flow.LightCollection
			mockErr   error
			response  string
			status    int
		}{{
			"efb41e75f21e99bdf0ce62a0afc52fdaeb352c3e8f409b63aa3a076fd26e2b3a",
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
			req, _ := http.NewRequest("GET", collectionURL(test.id), nil)
			id, _ := flow.HexStringToIdentifier(test.id)

			backend.Mock.
				On("GetCollectionByID", mocks.Anything, id).
				Return(test.mockValue, test.mockErr)

			rr := executeRequest(req, backend)

			assert.Equal(t, test.status, rr.Code)
			assert.JSONEq(t, test.response, rr.Body.String())
		}

	})

}
