package rest

import (
	"fmt"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"net/http"
	"testing"
)

func transactionURL(id string) string {
	return fmt.Sprintf("/v1/transactions/%s", id)
}

func TestGetTransactions(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by ID", func(t *testing.T) {
		tx := unittest.TransactionFixture()

		req, _ := http.NewRequest("GET", transactionURL(tx.ID().String()), nil)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(&tx.TransactionBody, nil)

		rr := executeRequest(req, backend)

		//bx, _ := json.Marshal(col.Transactions)
		//expected := fmt.Sprintf(`{"id":"%s","transactions":%s}`, col.ID().String(), bx)
		expected := ""

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, expected, rr.Body.String())
	})
}
