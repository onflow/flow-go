package collector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/proto/sdk/entities"
	"github.com/dapperlabs/flow-go/proto/services/collection"
)

// TestCollector_SubmitTransaction tests the function SubmitTransaction
func TestCollector_SubmitTransaction(t *testing.T) {
	collector := NewCollector()
	assert := assert.New(t)
	_, ok := collector.Data["Hello"]
	assert.NotEqual(ok, true)

	collector.SubmitTransaction(context.Background(), generateSubmitTransactionRequest("Hello"))

	_, ok = collector.Data["Hello"]
	assert.Equal(ok, true)
}

// TestCollector_GetTransaction tests the function GetTransaction
func TestCollector_GetTransaction(t *testing.T) {
	collector := NewCollector()
	assert := assert.New(t)
	tt := []struct {
		data   map[string]bool
		text   string
		status bool
	}{
		{
			data: map[string]bool{
				"Hello": true,
			},
			text:   "Hello",
			status: true,
		},
		{
			data: map[string]bool{
				"Exists": true,
			},
			text:   "Exists",
			status: true,
		},
		{
			data:   map[string]bool{},
			text:   "DoesNotExist",
			status: false,
		},
		{
			data: map[string]bool{
				"sth": true,
			},
			text:   "sthelse",
			status: false,
		},
	}

	for _, tc := range tt {
		collector.Data = tc.data
		resp, err := collector.GetTransaction(context.Background(), &collection.GetTransactionRequest{Hash: []byte(tc.text)})

		if tc.status == true {
			assert.Nil(err)
		}

		if tc.status == false {
			assert.NotNil(err)
		}
		if tc.status == true {
			assert.Equal(tc.text, string(resp.Transaction.Script))
		}
	}
}

// generateSubmitTransactionRequest generates a SubmitTransactionRequest for testing purposes
func generateSubmitTransactionRequest(text string) *collection.SubmitTransactionRequest {
	return &collection.SubmitTransactionRequest{Transaction: &entities.Transaction{Script: []byte(text)}}
}
