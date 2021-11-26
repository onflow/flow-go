package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
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

func transactionResultURL(id string) string {
	return fmt.Sprintf("/v1/transaction_results/%s", id)
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
	backend := &mock.API{}

	t.Run("get by ID", func(t *testing.T) {
		id := unittest.IdentifierFixture()
		bid := unittest.IdentifierFixture()
		txr := &access.TransactionResult{
			Status:     flow.TransactionStatusExecuted,
			StatusCode: 10,
			Events: []flow.Event{
				unittest.EventFixture(flow.EventAccountCreated, 1, 0, id, 200),
			},
			ErrorMessage: "",
			BlockID:      bid,
		}
		req, _ := http.NewRequest("GET", transactionResultURL(id.String()), nil)

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, id).
			Return(txr, nil)

		rr := executeRequest(req, backend)

		expected := fmt.Sprintf(`{
			"block_id": "%s",
			"status": "Executed",
			"error_message": "",
			"computation_used": 0,
			"events": [
				{
					"type": "flow.AccountCreated",
					"transaction_id": "%s",
					"transaction_index": 1,
					"event_index": 0,
					"payload": ""
				}
			],
			"_links": {
				"_self": "/v1/transaction_results/%s"
			}
		}`, bid.String(), id.String(), id.String())

		fmt.Println(rr.Body.String())

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		req, _ := http.NewRequest("GET", transactionURL("invalid"), nil)
		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.JSONEq(t, `{"code":400, "message":"invalid ID"}`, rr.Body.String())
	})
}

func TestCreateTransaction(t *testing.T) {
	backend := &mock.API{}

	t.Run("create", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		auth := make([]string, len(tx.Authorizers))
		for i, a := range tx.Authorizers {
			auth[i] = a.String()
		}
		validBody, _ := json.Marshal(map[string]interface{}{
			"script":             toBase64(tx.Script),
			"arguments":          tx.Arguments,
			"reference_block_id": tx.ReferenceBlockID.String(),
			"gas_limit":          tx.GasLimit,
			"payer":              tx.Payer.String(),
			"proposal_key": map[string]interface{}{
				"address":         tx.ProposalKey.Address.String(),
				"key_index":       tx.ProposalKey.KeyIndex,
				"sequence_number": tx.ProposalKey.SequenceNumber,
			},
			"authorizers": auth,
			"envelope_signatures": map[string]interface{}{
				"address":      tx.EnvelopeSignatures[0].Address.String(),
				"signer_index": int32(tx.EnvelopeSignatures[0].SignerIndex),
				"key_index":    int32(tx.EnvelopeSignatures[0].KeyIndex),
				"signature":    toBase64(tx.EnvelopeSignatures[0].Signature),
			},
		})

		req, _ := http.NewRequest("POST", "/v1/transactions", bytes.NewBuffer(validBody))

		backend.Mock.
			On("SendTransaction", mocks.Anything, tx).
			Return(nil)

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
		assert.Equal(t, expected, rr.Body.String())
	})
}
