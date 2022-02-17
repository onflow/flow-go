package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/onflow/flow-go/engine/access/rest/util"

	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func getTransactionReq(id string, expandResult bool) *http.Request {
	u, _ := url.Parse(fmt.Sprintf("/v1/transactions/%s", id))
	if expandResult {
		q := u.Query()
		// by default expand all since we test expanding with converters
		q.Add("expand", "result")
		u.RawQuery = q.Encode()
	}

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

func getTransactionResultReq(id string) *http.Request {
	req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/transaction_results/%s", id), nil)
	return req
}

func createTransactionReq(body interface{}) *http.Request {
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/transactions", bytes.NewBuffer(jsonBody))
	return req
}

func validCreateBody(tx flow.TransactionBody) map[string]interface{} {
	tx.Arguments = [][]uint8{} // fix how fixture creates nil values
	auth := make([]string, len(tx.Authorizers))
	for i, a := range tx.Authorizers {
		auth[i] = a.String()
	}

	return map[string]interface{}{
		"script":             util.ToBase64(tx.Script),
		"arguments":          tx.Arguments,
		"reference_block_id": tx.ReferenceBlockID.String(),
		"gas_limit":          fmt.Sprintf("%d", tx.GasLimit),
		"payer":              tx.Payer.String(),
		"proposal_key": map[string]interface{}{
			"address":         tx.ProposalKey.Address.String(),
			"key_index":       fmt.Sprintf("%d", tx.ProposalKey.KeyIndex),
			"sequence_number": fmt.Sprintf("%d", tx.ProposalKey.SequenceNumber),
		},
		"authorizers": auth,
		"payload_signatures": []map[string]interface{}{{
			"address":   tx.PayloadSignatures[0].Address.String(),
			"key_index": fmt.Sprintf("%d", tx.PayloadSignatures[0].KeyIndex),
			"signature": util.ToBase64(tx.PayloadSignatures[0].Signature),
		}},
		"envelope_signatures": []map[string]interface{}{{
			"address":   tx.EnvelopeSignatures[0].Address.String(),
			"key_index": fmt.Sprintf("%d", tx.EnvelopeSignatures[0].KeyIndex),
			"signature": util.ToBase64(tx.EnvelopeSignatures[0].Signature),
		}},
	}
}

func TestGetTransactions(t *testing.T) {

	t.Run("get by ID without results", func(t *testing.T) {
		backend := &mock.API{}
		tx := unittest.TransactionFixture()
		req := getTransactionReq(tx.ID().String(), false)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(&tx.TransactionBody, nil)

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"cHViIGZ1biBtYWluKCkge30=",
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

		assertOKResponse(t, req, expected, backend)
	})

	t.Run("Get by ID with results", func(t *testing.T) {
		backend := &mock.API{}
		tx := unittest.TransactionFixture()
		txr := transactionResultFixture(tx)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(&tx.TransactionBody, nil)

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, tx.ID()).
			Return(txr, nil)

		req := getTransactionReq(tx.ID().String(), true)

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"cHViIGZ1biBtYWluKCkge30=",
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
				"result": {
					"block_id": "%s",
					"status": "Sealed",
					"status_code": 1,
					"error_message": "",
					"computation_used": "0",
					"events": [
						{
							"type": "flow.AccountCreated",
							"transaction_id": "%s",
							"transaction_index": "0",
							"event_index": "0"
						}
					],
					"_links": {
						"_self": "/v1/transaction_results/%s"
					}
				},
               "_expandable": {},
			   "_links":{
				  "_self":"/v1/transactions/%s"
			   }
			}`,
			tx.ID(), tx.ReferenceBlockID, util.ToBase64(tx.EnvelopeSignatures[0].Signature), tx.ReferenceBlockID, tx.ID(), tx.ID(), tx.ID())
		assertOKResponse(t, req, expected, backend)
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		backend := &mock.API{}

		req := getTransactionReq("invalid", false)
		expected := `{"code":400, "message":"invalid ID format"}`
		assertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by ID non-existing", func(t *testing.T) {
		backend := &mock.API{}
		tx := unittest.TransactionFixture()
		req := getTransactionReq(tx.ID().String(), false)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(nil, status.Error(codes.NotFound, "transaction not found"))

		expected := `{"code":404, "message":"Flow resource not found: transaction not found"}`
		assertResponse(t, req, http.StatusNotFound, expected, backend)
	})
}

func TestGetTransactionResult(t *testing.T) {

	t.Run("get by ID", func(t *testing.T) {
		backend := &mock.API{}
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
		txr.Events[0].Payload = []byte(`test payload`)

		req := getTransactionResultReq(id.String())

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, id).
			Return(txr, nil)

		expected := fmt.Sprintf(`{
			"block_id": "%s",
			"status": "Executed",
			"status_code": 10,
			"error_message": "",
			"computation_used": "0",
			"events": [
				{
					"type": "flow.AccountCreated",
					"transaction_id": "%s",
					"transaction_index": "1",
					"event_index": "0",
					"payload": "%s"
				}
			],
			"_links": {
				"_self": "/v1/transaction_results/%s"
			}
		}`, bid.String(), id.String(), util.ToBase64(txr.Events[0].Payload), id.String())
		assertOKResponse(t, req, expected, backend)
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		backend := &mock.API{}
		req := getTransactionResultReq("invalid")

		expected := `{"code":400, "message":"invalid ID format"}`
		assertResponse(t, req, http.StatusBadRequest, expected, backend)
	})
}

func TestCreateTransaction(t *testing.T) {

	t.Run("create", func(t *testing.T) {
		backend := &mock.API{}
		tx := unittest.TransactionBodyFixture()
		tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
		tx.Arguments = [][]uint8{}
		req := createTransactionReq(validCreateBody(tx))

		backend.Mock.
			On("SendTransaction", mocks.Anything, &tx).
			Return(nil)

		expected := fmt.Sprintf(`
			{
			   "id":"%s",
			   "script":"cHViIGZ1biBtYWluKCkge30=",
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
		assertOKResponse(t, req, expected, backend)
	})

	t.Run("post invalid transaction", func(t *testing.T) {
		backend := &mock.API{}
		tests := []struct {
			inputField string
			inputValue string
			output     string
		}{
			{"reference_block_id", "-1", `{"code":400, "message":"invalid reference block ID: invalid ID format"}`},
			{"reference_block_id", "", `{"code":400, "message":"reference block not provided"}`},
			{"gas_limit", "-1", `{"code":400, "message":"invalid gas limit: value must be an unsigned 64 bit integer"}`},
			{"payer", "yo", `{"code":400, "message":"invalid payer: invalid address"}`},
			{"proposal_key", "yo", `{"code":400, "message":"request body contains an invalid value for the \"proposal_key\" field (at position 461)"}`},
			{"authorizers", "", `{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 32)"}`},
			{"authorizers", "yo", `{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 34)"}`},
			{"envelope_signatures", "", `{"code":400, "message":"request body contains an invalid value for the \"envelope_signatures\" field (at position 75)"}`},
			{"payload_signatures", "", `{"code":400, "message":"request body contains an invalid value for the \"payload_signatures\" field (at position 292)"}`},
		}

		for _, test := range tests {
			tx := unittest.TransactionBodyFixture()
			tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
			testTx := validCreateBody(tx)
			testTx[test.inputField] = test.inputValue
			req := createTransactionReq(testTx)

			assertResponse(t, req, http.StatusBadRequest, test.output, backend)
		}
	})
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
