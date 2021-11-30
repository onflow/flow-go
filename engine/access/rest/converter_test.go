package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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

	t.Run("transaction response", func(t *testing.T) {
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

	t.Run("transaction response with result", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		txr := transactionResultFixture(tx)
		tx.PayloadSignatures = []flow.TransactionSignature{unittest.TransactionSignatureFixture()} // add payload to fixture

		response := responseToMap(
			transactionResponse(&tx.TransactionBody, txr, linkFixture(), map[string]bool{"result": true}),
		)

		assert.NotNilf(t, response["result"], "result shouldn't be nil")
	})

}
