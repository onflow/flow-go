package rest

import (
	"fmt"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"cHViIGZ1biBtYWluKCkge30=",
			   "arguments":null,
			   "reference_block_id":"%s",
			   "gas_limit":10,
			   "payer":"8c5303eaa26202d6",
			   "proposal_key":{
				  "address":"8c5303eaa26202d6",
				  "key_index":1,
				  "sequence_number":0
			   },
			   "authorizers":[
				  "8c5303eaa26202d6"
			   ],
			   "envelope_signatures":[
				  {
					 "address":"8c5303eaa26202d6",
					 "signer_index":0,
					 "key_index":1,
					 "signature":"%s"
				  }
			   ],
			   "_links":{
				  "_self":"%s"
			   }
			}`,
			tx.ID().String(),
			tx.ReferenceBlockID.String(),
			toBase64(tx.EnvelopeSignatures[0].Signature),
			transactionURL(tx.ID().String()),
		)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		req, _ := http.NewRequest("GET", transactionURL("invalid"), nil)
		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.JSONEq(t, `{"code":400, "message":"invalid ID"}`, rr.Body.String())
	})

	t.Run("get by ID Non-existing", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		req, _ := http.NewRequest("GET", transactionURL(tx.ID().String()), nil)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(nil, status.Error(codes.NotFound, "not found"))

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.JSONEq(t, `{"code":404, "message":"not found"}`, rr.Body.String())
	})
}

func TestGetTransactionResult(t *testing.T) {

}
