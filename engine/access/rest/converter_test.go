package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func linkFixture() LinkGenerator {
	backend := &mock.API{}
	var log []byte
	r := initRouter(backend, zerolog.New(bytes.NewBuffer(log)))
	return NewLinkGeneratorImpl(r)
}

func responseToMap(response interface{}) map[string]interface{} {
	j, _ := json.Marshal(response)
	var res map[string]interface{}
	_ = json.Unmarshal(j, &res)
	return res
}

func TestTransactions(t *testing.T) {

	t.Run("response", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		response := transactionResponse(&tx.TransactionBody, nil, linkFixture(), nil)

		res, _ := json.Marshal(response)
		expected := fmt.Sprintf(`{
		   "id":"%s",
		   "script":"cHViIGZ1biBtYWluKCkge30=",
		   "arguments":null,
		   "reference_block_id":"%s",
		   "gas_limit":10,
		   "payer":"8c5303eaa26202d6",
		   "_expandable":{
			  "proposal_key":"proposal_key",
			  "authorizers":"authorizers",
			  "payload_signatures":"payload_signatures",
			  "envelope_signatures":"envelope_signatures",
			  "result":"/v1/transaction_results/%s"
		   },
		   "_links":{
			  "_self":"/v1/transactions/%s"
		   }
		}`, tx.ID().String(), tx.ReferenceBlockID.String(), tx.ID().String(), tx.ID().String())

		assert.JSONEq(t, expected, string(res))
	})

	t.Run("response expands", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()} // add payload to fixture

		tests := []map[string]bool{
			{"proposal_key": true},
			{"authorizers": true},
			{"envelope_signatures": true},
			{"payload_signatures": true},
		}

		for _, test := range tests {
			response := responseToMap(
				transactionResponse(&tx.TransactionBody, nil, linkFixture(), test),
			)

			for expand := range test {
				assert.NotNilf(t, response[expand], fmt.Sprintf("shouldn't be nil: %s", expand))
			}
		}

	})

	t.Run("response with result", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		txr := transactionResultFixture(tx)
		tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()} // add payload to fixture

		response := responseToMap(
			transactionResponse(&tx.TransactionBody, txr, linkFixture(), map[string]bool{"result": true}),
		)

		assert.NotNilf(t, response["result"], "result shouldn't be nil")
	})

}
