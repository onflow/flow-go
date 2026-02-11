package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/mock"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	mockcommonmodels "github.com/onflow/flow-go/engine/access/rest/common/models/mock"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func newGetTransactionRequest(id string, expandResult bool, blockIdQuery string, collectionIdQuery string) *http.Request {
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

func newGetTransactionsRequest(blockId string, height string, expandResult bool, collectionIdQuery string) *http.Request {
	u, _ := url.Parse("/v1/transactions")
	q := u.Query()

	if blockId != "" {
		q.Add("block_id", blockId)
	}

	if height != "" {
		q.Add("block_height", height)
	}

	if expandResult {
		q.Add("expand", "result")
	}

	if collectionIdQuery != "" {
		q.Add("collection_id", collectionIdQuery)
	}

	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

func newGetTransactionResultRequest(id string, blockIdQuery string, collectionIdQuery string) *http.Request {
	u, _ := url.Parse(fmt.Sprintf("/v1/transaction_results/%s", id))
	q := u.Query()
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

func newGetTransactionResultsRequest(blockIdQuery string, height string) *http.Request {
	u, _ := url.Parse("/v1/transaction_results")
	q := u.Query()

	if blockIdQuery != "" {
		q.Add("block_id", blockIdQuery)
	}

	if height != "" {
		q.Add("block_height", height)
	}

	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

func newCreateTransactionRequest(body interface{}) *http.Request {
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/transactions", bytes.NewBuffer(jsonBody))
	return req
}

func TestGetTransactions(t *testing.T) {
	t.Run("get by ID without results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		tx := unittest.TransactionFixture()
		req := newGetTransactionRequest(tx.ID().String(), false, "", "")

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(&tx, nil)

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
		backend := mock.NewAPI(t)

		tx := unittest.TransactionBodyFixture()
		txr := transactionResultFixture(tx)

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(&tx, nil)

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, tx.ID(), flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil)

		req := newGetTransactionRequest(tx.ID().String(), true, "", "")

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
				"result": {
					"block_id": "%s",
					"collection_id": "%s",
					"execution": "Success",
					"status": "Sealed",
					"status_code": 1,
					"error_message": "",
					"computation_used": "0",
					"events": [
						{
							"type": "flow.AccountCreated",
							"transaction_id": "%s",
							"transaction_index": "0",
							"event_index": "0",
							"payload": ""
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
			tx.ID(), tx.ReferenceBlockID, util.ToBase64(tx.EnvelopeSignatures[0].Signature), tx.ReferenceBlockID, txr.CollectionID, tx.ID(), tx.ID(), tx.ID())
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionRequest("invalid", false, "", "")
		expected := `{"code":400, "message":"invalid ID format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by ID non-existing", func(t *testing.T) {
		backend := mock.NewAPI(t)

		tx := unittest.TransactionFixture()
		req := newGetTransactionRequest(tx.ID().String(), false, "", "")

		backend.Mock.
			On("GetTransaction", mocks.Anything, tx.ID()).
			Return(nil, status.Error(codes.NotFound, "transaction not found"))

		expected := `{"code":404, "message":"Flow resource not found: transaction not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})
}

func TestGetTransactionsByBlock(t *testing.T) {
	t.Run("get by block ID without expanded results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		blockID := unittest.IdentifierFixture()

		tx1 := unittest.TransactionFixture()
		tx2 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1, &tx2}

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()
		req := newGetTransactionsRequest(blockID.String(), "", false, "")

		expected := fmt.Sprintf(`[
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
		},
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
		}
	]`,
			txs[0].ID(), txs[0].ReferenceBlockID, util.ToBase64(txs[0].EnvelopeSignatures[0].Signature), txs[0].ID(), txs[0].ID(),
			txs[1].ID(), txs[1].ReferenceBlockID, util.ToBase64(txs[1].EnvelopeSignatures[0].Signature), txs[1].ID(), txs[1].ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by height without expanded results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		height := uint64(123)
		block := unittest.BlockFixture()
		blockID := block.ID()

		tx1 := unittest.TransactionFixture()
		tx2 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1, &tx2}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, height).
			Return(block, flow.BlockStatusSealed, nil).Once()

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()

		req := newGetTransactionsRequest("", fmt.Sprintf("%d", height), false, "")

		expected := fmt.Sprintf(`[
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
		},
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
		}
	]`,
			txs[0].ID(), txs[0].ReferenceBlockID, util.ToBase64(txs[0].EnvelopeSignatures[0].Signature), txs[0].ID(), txs[0].ID(),
			txs[1].ID(), txs[1].ReferenceBlockID, util.ToBase64(txs[1].EnvelopeSignatures[0].Signature), txs[1].ID(), txs[1].ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by height sealed", func(t *testing.T) {
		backend := mock.NewAPI(t)

		block := unittest.BlockFixture()
		blockID := block.ID()

		tx1 := unittest.TransactionFixture()
		tx2 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1, &tx2}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, request.SealedHeight).
			Return(block, flow.BlockStatusSealed, nil).Once()

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()

		req := newGetTransactionsRequest("", router.SealedHeightQueryParam, false, "")

		expected := fmt.Sprintf(`[
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
		},
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
		}
	]`,
			txs[0].ID(), txs[0].ReferenceBlockID, util.ToBase64(txs[0].EnvelopeSignatures[0].Signature), txs[0].ID(), txs[0].ID(),
			txs[1].ID(), txs[1].ReferenceBlockID, util.ToBase64(txs[1].EnvelopeSignatures[0].Signature), txs[1].ID(), txs[1].ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by height final", func(t *testing.T) {
		backend := mock.NewAPI(t)

		block := unittest.BlockFixture()
		blockID := block.ID()

		tx1 := unittest.TransactionFixture()
		tx2 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1, &tx2}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, request.FinalHeight).
			Return(block, flow.BlockStatusFinalized, nil).Once()

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()

		req := newGetTransactionsRequest("", router.FinalHeightQueryParam, false, "")

		expected := fmt.Sprintf(`[
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
		},
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
		}
	]`,
			txs[0].ID(), txs[0].ReferenceBlockID, util.ToBase64(txs[0].EnvelopeSignatures[0].Signature), txs[0].ID(), txs[0].ID(),
			txs[1].ID(), txs[1].ReferenceBlockID, util.ToBase64(txs[1].EnvelopeSignatures[0].Signature), txs[1].ID(), txs[1].ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get with no block_id or height defaults to sealed", func(t *testing.T) {
		backend := mock.NewAPI(t)

		block := unittest.BlockFixture()
		blockID := block.ID()

		tx1 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, request.SealedHeight).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()

		req := newGetTransactionsRequest("", "", false, "")

		expected := fmt.Sprintf(`[
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
		}
	]`,
			txs[0].ID(), txs[0].ReferenceBlockID, util.ToBase64(txs[0].EnvelopeSignatures[0].Signature), txs[0].ID(), txs[0].ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by block ID with expanded results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		blockID := unittest.IdentifierFixture()

		tx1 := unittest.TransactionFixture()
		tx2 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1, &tx2}

		txr1 := transactionResultFixture(tx1)
		txr2 := transactionResultFixture(tx2)

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()

		txResults := []*accessmodel.TransactionResult{txr1, txr2}
		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txResults, nil).
			Once()

		req := newGetTransactionsRequest(blockID.String(), "", true, "")

		expected := fmt.Sprintf(`[
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
		   "result": {
			  "block_id": "%s",
			  "collection_id": "%s",
			  "execution": "Success",
			  "status": "Sealed",
			  "status_code": 1,
			  "error_message": "",
			  "computation_used": "0",
			  "events": [
				  {
					  "type": "flow.AccountCreated",
					  "transaction_id": "%s",
					  "transaction_index": "0",
					  "event_index": "0",
					  "payload": ""
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
		},
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
		   "result": {
			  "block_id": "%s",
			  "collection_id": "%s",
			  "execution": "Success",
			  "status": "Sealed",
			  "status_code": 1,
			  "error_message": "",
			  "computation_used": "0",
			  "events": [
				  {
					  "type": "flow.AccountCreated",
					  "transaction_id": "%s",
					  "transaction_index": "0",
					  "event_index": "0",
					  "payload": ""
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
		}
	]`,
			tx1.ID(), tx1.ReferenceBlockID, util.ToBase64(tx1.EnvelopeSignatures[0].Signature),
			tx1.ReferenceBlockID, txr1.CollectionID, tx1.ID(), tx1.ID(), tx1.ID(),
			tx2.ID(), tx2.ReferenceBlockID, util.ToBase64(tx2.EnvelopeSignatures[0].Signature),
			tx2.ReferenceBlockID, txr2.CollectionID, tx2.ID(), tx2.ID(), tx2.ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by height with expanded results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		height := uint64(123)
		block := unittest.BlockFixture()
		blockID := block.ID()

		tx1 := unittest.TransactionFixture()
		tx2 := unittest.TransactionFixture()
		txs := []*flow.TransactionBody{&tx1, &tx2}

		txr1 := transactionResultFixture(tx1)
		txr2 := transactionResultFixture(tx2)

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, height).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(txs, nil).
			Once()

		txResults := []*accessmodel.TransactionResult{txr1, txr2}
		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txResults, nil).
			Once()

		req := newGetTransactionsRequest("", fmt.Sprintf("%d", height), true, "")

		expected := fmt.Sprintf(`[
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
		   "result": {
			  "block_id": "%s",
			  "collection_id": "%s",
			  "execution": "Success",
			  "status": "Sealed",
			  "status_code": 1,
			  "error_message": "",
			  "computation_used": "0",
			  "events": [
				  {
					  "type": "flow.AccountCreated",
					  "transaction_id": "%s",
					  "transaction_index": "0",
					  "event_index": "0",
					  "payload": ""
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
		},
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
		   "result": {
			  "block_id": "%s",
			  "collection_id": "%s",
			  "execution": "Success",
			  "status": "Sealed",
			  "status_code": 1,
			  "error_message": "",
			  "computation_used": "0",
			  "events": [
				  {
					  "type": "flow.AccountCreated",
					  "transaction_id": "%s",
					  "transaction_index": "0",
					  "event_index": "0",
					  "payload": ""
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
		}
	]`,
			tx1.ID(), tx1.ReferenceBlockID, util.ToBase64(tx1.EnvelopeSignatures[0].Signature),
			tx1.ReferenceBlockID, txr1.CollectionID, tx1.ID(), tx1.ID(), tx1.ID(),
			tx2.ID(), tx2.ReferenceBlockID, util.ToBase64(tx2.EnvelopeSignatures[0].Signature),
			tx2.ReferenceBlockID, txr2.CollectionID, tx2.ID(), tx2.ID(), tx2.ID(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by block ID invalid block_id", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionsRequest("invalid", "", false, "")

		expected := `{"code":400, "message":"invalid ID format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by block ID non-existing block", func(t *testing.T) {
		backend := mock.NewAPI(t)
		blockID := unittest.IdentifierFixture()

		backend.Mock.
			On("GetTransactionsByBlockID", mocks.Anything, blockID).
			Return(nil, status.Error(codes.NotFound, "block not found")).
			Once()

		req := newGetTransactionsRequest(blockID.String(), "", false, "")

		expected := `{"code":404, "message":"Flow resource not found: block not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})

	t.Run("get by height invalid height", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionsRequest("", "not-a-height", false, "")

		expected := `{"code":400, "message":"invalid height format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by height non-existing block", func(t *testing.T) {
		backend := mock.NewAPI(t)

		height := uint64(123)

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, height).
			Return((*flow.Block)(nil), flow.BlockStatusUnknown, status.Error(codes.NotFound, "block not found")).
			Once()

		req := newGetTransactionsRequest("", fmt.Sprintf("%d", height), false, "")

		expected := `{"code":404, "message":"Flow resource not found: block not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})

	t.Run("get with both block_id and height is invalid", func(t *testing.T) {
		backend := mock.NewAPI(t)

		blockID := unittest.IdentifierFixture()
		req := newGetTransactionsRequest(blockID.String(), "123", false, "")

		expected := `{"code":400, "message":"can not provide both block ID and block height"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

}

func TestGetTransactionResult(t *testing.T) {
	id := unittest.IdentifierFixture()
	bid := unittest.IdentifierFixture()
	cid := unittest.IdentifierFixture()
	txr := &accessmodel.TransactionResult{
		Status:     flow.TransactionStatusSealed,
		StatusCode: 10,
		Events: []flow.Event{
			unittest.EventFixture(
				unittest.Event.WithEventType(flow.EventAccountCreated),
				unittest.Event.WithTransactionIndex(1),
				unittest.Event.WithEventIndex(0),
				unittest.Event.WithTransactionID(id),
			),
		},
		ErrorMessage: "",
		BlockID:      bid,
		CollectionID: cid,
	}
	txr.Events[0].Payload = []byte(`test payload`)
	expected := fmt.Sprintf(`{
			"block_id": "%s",
			"collection_id": "%s",
			"execution": "Success",
			"status": "Sealed",
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
		}`, bid.String(), cid.String(), id.String(), util.ToBase64(txr.Events[0].Payload), id.String())

	t.Run("get by transaction ID", func(t *testing.T) {
		backend := mock.NewAPI(t)
		req := newGetTransactionResultRequest(id.String(), "", "")

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, id, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by block ID", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionResultRequest(id.String(), bid.String(), "")

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, id, bid, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by collection ID", func(t *testing.T) {
		backend := mock.NewAPI(t)
		req := newGetTransactionResultRequest(id.String(), "", cid.String())

		backend.Mock.
			On("GetTransactionResult", mocks.Anything, id, flow.ZeroID, cid, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get execution statuses", func(t *testing.T) {
		backend := mock.NewAPI(t)

		testVectors := map[*accessmodel.TransactionResult]string{{
			Status:       flow.TransactionStatusExpired,
			ErrorMessage: "",
		}: string(commonmodels.FAILURE_RESULT), {
			Status:       flow.TransactionStatusSealed,
			ErrorMessage: "cadence runtime exception",
		}: string(commonmodels.FAILURE_RESULT), {
			Status:       flow.TransactionStatusFinalized,
			ErrorMessage: "",
		}: string(commonmodels.PENDING_RESULT), {
			Status:       flow.TransactionStatusPending,
			ErrorMessage: "",
		}: string(commonmodels.PENDING_RESULT), {
			Status:       flow.TransactionStatusExecuted,
			ErrorMessage: "",
		}: string(commonmodels.PENDING_RESULT), {
			Status:       flow.TransactionStatusSealed,
			ErrorMessage: "",
		}: string(commonmodels.SUCCESS_RESULT)}

		for txResult, err := range testVectors {
			txResult.BlockID = bid
			txResult.CollectionID = cid
			req := newGetTransactionResultRequest(id.String(), "", "")
			backend.Mock.
				On("GetTransactionResult", mocks.Anything, id, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0).
				Return(txResult, nil).
				Once()

			expectedResp := fmt.Sprintf(`{
				"block_id": "%s",
				"collection_id": "%s",
				"execution": "%s",
				"status": "%s",
				"status_code": 0,
				"error_message": "%s",
				"computation_used": "0",
				"events": [],
				"_links": {
					"_self": "/v1/transaction_results/%s"
				}
			}`, bid.String(), cid.String(), err, cases.Title(language.English).String(strings.ToLower(txResult.Status.String())), txResult.ErrorMessage, id.String())
			router.AssertOKResponse(t, req, expectedResp, backend)
		}
	})

	t.Run("get by ID Invalid", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionResultRequest("invalid", "", "")

		expected := `{"code":400, "message":"invalid ID format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})
}

func TestGetTransactionResultsByBlock(t *testing.T) {
	t.Run("get by block ID", func(t *testing.T) {
		backend := mock.NewAPI(t)
		blockID := unittest.IdentifierFixture()

		id1 := unittest.IdentifierFixture()
		bid1 := blockID
		cid1 := unittest.IdentifierFixture()
		txr1 := &accessmodel.TransactionResult{
			Status:     flow.TransactionStatusSealed,
			StatusCode: 10,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(1),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id1),
				),
			},
			ErrorMessage:  "",
			BlockID:       bid1,
			CollectionID:  cid1,
			TransactionID: id1,
		}
		txr1.Events[0].Payload = []byte(`test payload 1`)

		id2 := unittest.IdentifierFixture()
		bid2 := blockID
		cid2 := unittest.IdentifierFixture()
		txr2 := &accessmodel.TransactionResult{
			Status:     flow.TransactionStatusSealed,
			StatusCode: 10,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(0),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id2),
				),
			},
			ErrorMessage:  "",
			BlockID:       bid2,
			CollectionID:  cid2,
			TransactionID: id2,
		}
		txr2.Events[0].Payload = []byte(`test payload 2`)

		txResults := []*accessmodel.TransactionResult{txr1, txr2}

		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txResults, nil).Once()

		req := newGetTransactionResultsRequest(blockID.String(), "")

		expected := fmt.Sprintf(`[
			{
				"block_id": "%s",
				"collection_id": "%s",
				"execution": "Success",
				"status": "Sealed",
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
			},
			{
				"block_id": "%s",
				"collection_id": "%s",
				"execution": "Success",
				"status": "Sealed",
				"status_code": 10,
				"error_message": "",
				"computation_used": "0",
				"events": [
					{
						"type": "flow.AccountCreated",
						"transaction_id": "%s",
						"transaction_index": "0",
						"event_index": "0",
						"payload": "%s"
					}
				],
				"_links": {
					"_self": "/v1/transaction_results/%s"
				}
			}
		]`,
			bid1.String(), cid1.String(), id1.String(), util.ToBase64(txr1.Events[0].Payload), id1.String(),
			bid2.String(), cid2.String(), id2.String(), util.ToBase64(txr2.Events[0].Payload), id2.String(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by height", func(t *testing.T) {
		backend := mock.NewAPI(t)
		height := uint64(123)
		block := unittest.BlockFixture()
		blockID := block.ID()

		id1 := unittest.IdentifierFixture()
		cid1 := unittest.IdentifierFixture()
		txr1 := &accessmodel.TransactionResult{
			Status:     flow.TransactionStatusSealed,
			StatusCode: 10,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(1),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id1),
				),
			},
			ErrorMessage:  "",
			BlockID:       blockID,
			CollectionID:  cid1,
			TransactionID: id1,
		}
		txr1.Events[0].Payload = []byte(`test payload 1`)

		id2 := unittest.IdentifierFixture()
		cid2 := unittest.IdentifierFixture()
		txr2 := &accessmodel.TransactionResult{
			Status:     flow.TransactionStatusSealed,
			StatusCode: 10,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(0),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id2),
				),
			},
			ErrorMessage:  "",
			BlockID:       blockID,
			CollectionID:  cid2,
			TransactionID: id2,
		}
		txr2.Events[0].Payload = []byte(`test payload 2`)

		txResults := []*accessmodel.TransactionResult{txr1, txr2}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, height).
			Return(block, flow.BlockStatusSealed, nil).Once()

		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txResults, nil).Once()

		req := newGetTransactionResultsRequest("", fmt.Sprintf("%d", height))

		expected := fmt.Sprintf(`[
			{
				"block_id": "%s",
				"collection_id": "%s",
				"execution": "Success",
				"status": "Sealed",
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
			},
			{
				"block_id": "%s",
				"collection_id": "%s",
				"execution": "Success",
				"status": "Sealed",
				"status_code": 10,
				"error_message": "",
				"computation_used": "0",
				"events": [
					{
						"type": "flow.AccountCreated",
						"transaction_id": "%s",
						"transaction_index": "0",
						"event_index": "0",
						"payload": "%s"
					}
				],
				"_links": {
					"_self": "/v1/transaction_results/%s"
				}
			}
		]`,
			blockID.String(), cid1.String(), id1.String(), util.ToBase64(txr1.Events[0].Payload), id1.String(),
			blockID.String(), cid2.String(), id2.String(), util.ToBase64(txr2.Events[0].Payload), id2.String(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get results by height sealed", func(t *testing.T) {
		backend := mock.NewAPI(t)

		block := unittest.BlockFixture()
		blockID := block.ID()

		id1 := unittest.IdentifierFixture()
		cid1 := unittest.IdentifierFixture()
		txr1 := &accessmodel.TransactionResult{
			Status:        flow.TransactionStatusSealed,
			StatusCode:    10,
			ErrorMessage:  "",
			BlockID:       blockID,
			CollectionID:  cid1,
			TransactionID: id1,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(1),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id1),
				),
			},
		}
		txr1.Events[0].Payload = []byte(`test payload 1`)

		txResults := []*accessmodel.TransactionResult{txr1}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, request.SealedHeight).
			Return(block, flow.BlockStatusSealed, nil).Once()

		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txResults, nil).Once()

		req := newGetTransactionResultsRequest("", router.SealedHeightQueryParam)

		expected := fmt.Sprintf(`[
		{
			"block_id": "%s",
			"collection_id": "%s",
			"execution": "Success",
			"status": "Sealed",
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
		}
	]`,
			blockID.String(), cid1.String(), id1.String(), util.ToBase64(txr1.Events[0].Payload), id1.String(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get results by height finalized", func(t *testing.T) {
		backend := mock.NewAPI(t)

		block := unittest.BlockFixture()
		blockID := block.ID()

		id1 := unittest.IdentifierFixture()
		cid1 := unittest.IdentifierFixture()
		txr1 := &accessmodel.TransactionResult{
			Status:        flow.TransactionStatusFinalized,
			StatusCode:    10,
			ErrorMessage:  "",
			BlockID:       blockID,
			CollectionID:  cid1,
			TransactionID: id1,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(1),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id1),
				),
			},
		}
		txr1.Events[0].Payload = []byte(`test payload 1`)

		txResults := []*accessmodel.TransactionResult{txr1}

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, request.FinalHeight).
			Return(block, flow.BlockStatusFinalized, nil).Once()

		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txResults, nil).Once()

		req := newGetTransactionResultsRequest("", router.FinalHeightQueryParam)

		expected := fmt.Sprintf(`[
		{
			"block_id": "%s",
			"collection_id": "%s",
			"execution": "Pending",
			"status": "Finalized",
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
		}
	]`,
			blockID.String(), cid1.String(), id1.String(), util.ToBase64(txr1.Events[0].Payload), id1.String(),
		)

		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get results with no block_id or height defaults to sealed", func(t *testing.T) {
		backend := mock.NewAPI(t)

		block := unittest.BlockFixture()
		blockID := block.ID()

		id1 := unittest.IdentifierFixture()
		cid1 := unittest.IdentifierFixture()
		txr1 := &accessmodel.TransactionResult{
			Status:        flow.TransactionStatusSealed,
			StatusCode:    10,
			ErrorMessage:  "",
			BlockID:       blockID,
			CollectionID:  cid1,
			TransactionID: id1,
			Events: []flow.Event{
				unittest.EventFixture(
					unittest.Event.WithEventType(flow.EventAccountCreated),
					unittest.Event.WithTransactionIndex(1),
					unittest.Event.WithEventIndex(0),
					unittest.Event.WithTransactionID(id1),
				),
			},
		}
		txr1.Events[0].Payload = []byte(`test payload 1`)

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, request.SealedHeight).
			Return(block, flow.BlockStatusSealed, nil).Once()

		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return([]*accessmodel.TransactionResult{txr1}, nil).Once()

		req := newGetTransactionResultsRequest("", "")

		expected := fmt.Sprintf(`[
		{
			"block_id": "%s",
			"collection_id": "%s",
			"execution": "Success",
			"status": "Sealed",
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
		}
	]`, blockID.String(), cid1.String(), id1.String(), util.ToBase64(txr1.Events[0].Payload), id1.String())
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by block ID invalid block_id", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionResultsRequest("invalid", "")

		expected := `{"code":400, "message":"invalid ID format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by height invalid height", func(t *testing.T) {
		backend := mock.NewAPI(t)

		req := newGetTransactionResultsRequest("", "not-a-height")

		expected := `{"code":400, "message":"invalid height format"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})

	t.Run("get by block ID non-existing block", func(t *testing.T) {
		backend := mock.NewAPI(t)

		blockID := unittest.IdentifierFixture()

		backend.Mock.
			On("GetTransactionResultsByBlockID", mocks.Anything, blockID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(nil, status.Error(codes.NotFound, "block not found")).Once()

		req := newGetTransactionResultsRequest(blockID.String(), "")

		expected := `{"code":404, "message":"Flow resource not found: block not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})

	t.Run("get by height non-existing block", func(t *testing.T) {
		backend := mock.NewAPI(t)

		height := uint64(123)

		backend.Mock.
			On("GetBlockByHeight", mocks.Anything, height).
			Return((*flow.Block)(nil), flow.BlockStatusUnknown, status.Error(codes.NotFound, "block not found")).
			Once()

		req := newGetTransactionResultsRequest("", fmt.Sprintf("%d", height))

		expected := `{"code":404, "message":"Flow resource not found: block not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})

	t.Run("get with both block_id and height is invalid", func(t *testing.T) {
		backend := mock.NewAPI(t)

		blockID := unittest.IdentifierFixture()
		req := newGetTransactionResultsRequest(blockID.String(), "123")

		expected := `{"code":400, "message":"can not provide both block ID and block height"}`
		router.AssertResponse(t, req, http.StatusBadRequest, expected, backend)
	})
}

func TestGetScheduledTransactions(t *testing.T) {
	g := fixtures.NewGeneratorSuite()

	scheduledTxID := uint64(42)
	tx := scheduledTransactionFixture(t, g, scheduledTxID)
	txID := tx.ID()

	txr := transactionResultFixture(*tx)

	link := mockcommonmodels.NewLinkGenerator(t)
	link.On("TransactionLink", txID).Return(fmt.Sprintf("/v1/transactions/%s", txID), nil)
	link.On("TransactionResultLink", txID).Return(fmt.Sprintf("/v1/transaction_results/%s", txID), nil)

	expectedWithoutResult := expectedTransactionResponse(t, tx, nil, link)
	expectedWithResult := expectedTransactionResponse(t, tx, txr, link)

	t.Run("get by scheduled transaction ID without results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		backend.
			On("GetScheduledTransaction", mocks.Anything, scheduledTxID).
			Return(tx, nil).
			Once()

		req := newGetTransactionRequest(fmt.Sprint(scheduledTxID), false, "", "")

		router.AssertOKResponse(t, req, string(expectedWithoutResult), backend)
	})

	t.Run("get by scheduled transaction ID with results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		backend.
			On("GetScheduledTransaction", mocks.Anything, scheduledTxID).
			Return(tx, nil).
			Once()
		backend.
			On("GetScheduledTransactionResult", mocks.Anything, scheduledTxID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil).
			Once()

		req := newGetTransactionRequest(fmt.Sprint(scheduledTxID), true, "", "")

		router.AssertOKResponse(t, req, string(expectedWithResult), backend)
	})

	t.Run("get result by scheduled transaction ID", func(t *testing.T) {
		var expectedTxResult commonmodels.TransactionResult
		expectedTxResult.Build(txr, txID, link)
		expectedResult, err := json.Marshal(expectedTxResult)
		require.NoError(t, err)

		backend := mock.NewAPI(t)
		backend.
			On("GetScheduledTransactionResult", mocks.Anything, scheduledTxID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil).
			Once()

		req := newGetTransactionResultRequest(fmt.Sprint(scheduledTxID), "", "")

		router.AssertOKResponse(t, req, string(expectedResult), backend)
	})

	// these are identical to the regular get transaction tests, except the tx body is a scheduled
	// transaction. Scheduled tx bodies contain a subset of the information in regular submitted tx,
	// so this ensures the endpoints return the expected responses.
	t.Run("get by transaction ID without results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		backend.
			On("GetTransaction", mocks.Anything, txID).
			Return(tx, nil).
			Once()

		req := newGetTransactionRequest(txID.String(), false, "", "")

		router.AssertOKResponse(t, req, string(expectedWithoutResult), backend)
	})

	t.Run("get by transaction ID with results", func(t *testing.T) {
		backend := mock.NewAPI(t)
		backend.
			On("GetTransaction", mocks.Anything, txID).
			Return(tx, nil).
			Once()
		backend.
			On("GetTransactionResult", mocks.Anything, txID, flow.ZeroID, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0).
			Return(txr, nil).
			Once()

		req := newGetTransactionRequest(txID.String(), true, "", "")

		router.AssertOKResponse(t, req, string(expectedWithResult), backend)
	})

	t.Run("get by ID non-existing", func(t *testing.T) {
		backend := mock.NewAPI(t)
		backend.
			On("GetScheduledTransaction", mocks.Anything, scheduledTxID).
			Return(nil, status.Error(codes.NotFound, "transaction not found"))

		req := newGetTransactionRequest(fmt.Sprint(scheduledTxID), false, "", "")

		expected := `{"code":404, "message":"Flow resource not found: transaction not found"}`
		router.AssertResponse(t, req, http.StatusNotFound, expected, backend)
	})
}

func TestCreateTransaction(t *testing.T) {
	backend := mock.NewAPI(t)

	t.Run("create", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
		tx.Arguments = [][]uint8{}
		req := newCreateTransactionRequest(unittest.CreateSendTxHttpPayload(tx))

		backend.Mock.
			On("SendTransaction", mocks.Anything, &tx).
			Return(nil)

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
			{"proposal_key", "yo", `{"code":400, "message":"request body contains an invalid value for the \"proposal_key\" field (at position 461)"}`},
			{"authorizers", "", `{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 32)"}`},
			{"authorizers", "yo", `{"code":400, "message":"request body contains an invalid value for the \"authorizers\" field (at position 34)"}`},
			{"envelope_signatures", "", `{"code":400, "message":"request body contains an invalid value for the \"envelope_signatures\" field (at position 75)"}`},
			{"payload_signatures", "", `{"code":400, "message":"request body contains an invalid value for the \"payload_signatures\" field (at position 292)"}`},
		}

		for _, test := range tests {
			tx := unittest.TransactionBodyFixture()
			tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()}
			testTx := unittest.CreateSendTxHttpPayload(tx)
			testTx[test.inputField] = test.inputValue
			req := newCreateTransactionRequest(testTx)

			router.AssertResponse(t, req, http.StatusBadRequest, test.output, backend)
		}
	})
}

// transactionResultFixture constructs a successful transaction result fixture for the given
// transaction body.
func transactionResultFixture(tx flow.TransactionBody) *accessmodel.TransactionResult {
	txID := tx.ID()
	cid := unittest.IdentifierFixture()
	return &accessmodel.TransactionResult{
		Status:     flow.TransactionStatusSealed,
		StatusCode: 1,
		Events: []flow.Event{
			unittest.EventFixture(
				unittest.Event.WithEventType(flow.EventAccountCreated),
				unittest.Event.WithTransactionIndex(0),
				unittest.Event.WithEventIndex(0),
				unittest.Event.WithTransactionID(txID),
				unittest.Event.WithPayload([]byte{}),
			),
		},
		ErrorMessage:  "",
		BlockID:       tx.ReferenceBlockID,
		CollectionID:  cid,
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
	var expectedTxWithoutResult commonmodels.Transaction
	expectedTxWithoutResult.Build(tx, txr, link)

	expected, err := json.Marshal(expectedTxWithoutResult)
	require.NoError(t, err)

	return string(expected)
}
