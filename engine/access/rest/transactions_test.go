package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func transactionURL(id string, expandable []string) string {
	u, _ := url.Parse(fmt.Sprintf("/v1/transactions/%s", id))
	q := u.Query()

	// by default expand all since we test expanding with converters
	expandable = append(expandable, []string{"proposal_key", "authorizers", "payload_signatures", "envelope_signatures"}...)
	if len(expandable) > 0 {
		q.Add("expand", strings.Join(expandable, ","))
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func transactionResultURL(id string) string {
	return fmt.Sprintf("/v1/transaction_results/%s", id)
}

func TestGetTransactions(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by ID", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		req, _ := http.NewRequest(
			"GET",
			transactionURL(tx.ID().String(), nil),
			nil,
		)

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
				  "_self":"/v1/transactions/%s"
			   },
				"_expandable": {
					"proposal_key": "proposal_key",
					"authorizers": "authorizers",
					"payload_signatures": "payload_signatures",
					"envelope_signatures": "envelope_signatures",
					"result": "/v1/transaction_results/%s"
				}
			}`,
			tx.ID().String(),
			tx.ReferenceBlockID.String(),
			toBase64(tx.EnvelopeSignatures[0].Signature),
			tx.ID().String(),
			tx.ID().String(),
		)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("Get by ID with results", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		txr := transactionResultFixture(tx)

		req, _ := http.NewRequest(
			"GET",
			transactionURL(tx.ID().String(), []string{"result"}),
			nil,
		)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(&tx.TransactionBody, nil)

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, tx.ID()).
			Return(txr, nil)

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
				"result": {
					"block_id": "%s",
					"status": "Sealed",
					"error_message": "",
					"computation_used": 0,
					"events": [
						{
							"type": "flow.AccountCreated",
							"transaction_id": "%s",
							"transaction_index": 0,
							"event_index": 0,
							"payload": ""
						}
					],
					"_expandable": {
						"events": "events"
					},
					"_links": {
						"_self": "/v1/transaction_results/%s"
					}
				},
			   "_links":{
				  "_self":"/v1/transactions/%s"
			   },
				"_expandable": {
					"proposal_key": "proposal_key",
					"authorizers": "authorizers",
					"payload_signatures": "payload_signatures",
					"envelope_signatures": "envelope_signatures",
					"result": "/v1/transaction_results/%s"
				}
			}`,
			tx.ID().String(),
			tx.ReferenceBlockID.String(),
			toBase64(tx.EnvelopeSignatures[0].Signature),
			tx.ReferenceBlockID.String(),
			tx.ID().String(),
			tx.ID().String(),
			tx.ID().String(),
			tx.ID().String(),
		)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		req, _ := http.NewRequest("GET", transactionURL("invalid", nil), nil)
		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.JSONEq(t, `{"code":400, "message":"invalid ID format"}`, rr.Body.String())
	})

	t.Run("get by ID Non-existing", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		req, _ := http.NewRequest("GET", transactionURL(tx.ID().String(), nil), nil)

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
			"_expandable": {
				"events": "events"
			},
			"_links": {
				"_self": "/v1/transaction_results/%s"
			}
		}`, bid.String(), id.String(), id.String())

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		req, _ := http.NewRequest("GET", transactionURL("invalid", nil), nil)
		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.JSONEq(t, `{"code":400, "message":"invalid ID format"}`, rr.Body.String())
	})
}

func TestCreateTransaction(t *testing.T) {
	backend := &mock.API{}
	tx := unittest.TransactionBodyFixture()
	tx.Arguments = [][]uint8{} // fix how fixture creates nil values
	tx.PayloadSignatures = []flow.TransactionSignature{}
	auth := make([]string, len(tx.Authorizers))
	for i, a := range tx.Authorizers {
		auth[i] = a.String()
	}

	jsonTx := map[string]interface{}{
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
		"envelope_signatures": []map[string]interface{}{{
			"address":      tx.EnvelopeSignatures[0].Address.String(),
			"signer_index": int32(tx.EnvelopeSignatures[0].SignerIndex),
			"key_index":    int32(tx.EnvelopeSignatures[0].KeyIndex),
			"signature":    toBase64(tx.EnvelopeSignatures[0].Signature),
		}},
	}
	validBody, _ := json.Marshal(jsonTx)

	t.Run("create", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/v1/transactions", bytes.NewBuffer(validBody))

		backend.Mock.
			On("SendTransaction", mocks.Anything, &tx).
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
				"_expandable": {
					"proposal_key": "proposal_key",
					"authorizers": "authorizers",
					"payload_signatures": "payload_signatures",
					"envelope_signatures": "envelope_signatures",
					"result": "/v1/transaction_results/%s"
				},
			   "_links":{
				  "_self":"/v1/transactions/%s"
			   }
			}`,
			tx.ID().String(),
			tx.ReferenceBlockID.String(),
			toBase64(tx.EnvelopeSignatures[0].Signature),
			tx.ID().String(),
			tx.ID().String(),
		)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("create invalid", func(t *testing.T) {
		tests := []struct {
			inputField string
			inputValue string
			output     string
		}{
			{"reference_block_id", "-1", `{"code":400, "message":"invalid ID format"}`},
			{"reference_block_id", "", `{"code":400, "message":"reference block not provided"}`},
			{"gas_limit", "-1", `{"code":400, "message":"request body contains an invalid value for the \"gas_limit\" field (at position 256)"}`},
			{"payer", "yo", `{"code":400, "message":"invalid address"}`},
			{"proposal_key", "yo", `{"code":400, "message":"request body contains an invalid value for the \"proposal_key\" field (at position 301)"}`},
			{"authorizers", "", `{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 32)"}`},
			{"authorizers", "yo", `{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 34)"}`},
			{"envelope_signatures", "", `{"code":400, "message":"request body contains an invalid value for the \"envelope_signatures\" field (at position 75)"}`},
			{"payload_signatures", "", `{"code":400, "message":"request body contains an invalid value for the \"payload_signatures\" field (at position 305)"}`},
		}

		for i, test := range tests {
			testTx := copyMap(jsonTx)
			testTx[test.inputField] = test.inputValue
			validBody, _ = json.Marshal(testTx)

			req, _ := http.NewRequest("POST", "/v1/transactions", bytes.NewBuffer(validBody))
			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.output, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}

func copyMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func transactionResultFixture(tx flow.Transaction) *access.TransactionResult {
	return &access.TransactionResult{
		Status:     flow.TransactionStatusSealed,
		StatusCode: 1,
		Events: []flow.Event{
			unittest.EventFixture(flow.EventAccountCreated, 0, 0, tx.ID(), 255),
		},
		ErrorMessage: "",
		BlockID:      tx.ReferenceBlockID,
	}
}
