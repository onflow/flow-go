package main

import (
	"context"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/grpc/services/collect"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// TestCollector_SubmitTransaction tests the function SubmitTransaction
func TestCollector_SubmitTransaction(t *testing.T) {
	collector := NewCollector()

	if _, ok := collector.Data["Hello"]; ok == true {
		t.Error("collector node should not contain key before insertion")
	}

	collector.SubmitTransaction(context.Background(), generateSubmitTransactionRequest("Hello"))

	if _, ok := collector.Data["Hello"]; ok != true {
		t.Error("collector node should contain key: Key not found")
	}
}

// TestCollector_GetTransaction tests the function GetTransaction
func TestCollector_GetTransaction(t *testing.T) {
	collector := NewCollector()

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
		resp, err := collector.GetTransaction(context.Background(), &collect.GetTransactionRequest{TransactionHash: []byte(tc.text)})

		if err != nil && tc.status == true {
			t.Errorf("GetTransaction: Expected: nil, Got: %v", err)
		}

		if err == nil && tc.status == false {
			t.Errorf("GetTransaction: Expected %v, Got: %v", errNotFound, err)
		}
		if tc.status == true && string(resp.Transaction.Script) != tc.text {
			t.Errorf("expected Script to be: %v returned: %v", tc.text, string(resp.Transaction.Script))
		}
	}
}

// generateSubmitTransactionRequest generates a SubmitTransactionRequest for testing purposes
func generateSubmitTransactionRequest(text string) *collect.SubmitTransactionRequest {
	return &collect.SubmitTransactionRequest{Transaction: &shared.Transaction{Script: []byte(text)}}
}
