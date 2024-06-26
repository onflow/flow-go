package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "/v1/transactions",
			url:      "/v1/transactions",
			expected: "createTransaction",
		},
		{
			name:     "/v1/transactions/{id}",
			url:      "/v1/transactions/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getTransactionByID",
		},
		{
			name:     "/v1/transaction_results/{id}",
			url:      "/v1/transaction_results/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getTransactionResultByID",
		},
		{
			name:     "/v1/blocks",
			url:      "/v1/blocks",
			expected: "getBlocksByHeight",
		},
		{
			name:     "/v1/blocks/{id}",
			url:      "/v1/blocks/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getBlocksByIDs",
		},
		{
			name:     "/v1/blocks/{id}/payload",
			url:      "/v1/blocks/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76/payload",
			expected: "getBlockPayloadByID",
		},
		{
			name:     "/v1/execution_results/{id}",
			url:      "/v1/execution_results/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getExecutionResultByID",
		},
		{
			name:     "/v1/execution_results",
			url:      "/v1/execution_results",
			expected: "getExecutionResultByBlockID",
		},
		{
			name:     "/v1/collections/{id}",
			url:      "/v1/collections/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getCollectionByID",
		},
		{
			name:     "/v1/scripts",
			url:      "/v1/scripts",
			expected: "executeScript",
		},
		{
			name:     "/v1/accounts/{address}",
			url:      "/v1/accounts/6a587be304c1224c",
			expected: "getAccount",
		},
		{
			name:     "/v1/accounts/{address}/keys/{index}",
			url:      "/v1/accounts/6a587be304c1224c/keys/0",
			expected: "getAccountKeyByIndex",
		},
		{
			name:     "/v1/events",
			url:      "/v1/events",
			expected: "getEvents",
		},
		{
			name:     "/v1/network/parameters",
			url:      "/v1/network/parameters",
			expected: "getNetworkParameters",
		},
		{
			name:     "/v1/node_version_info",
			url:      "/v1/node_version_info",
			expected: "getNodeVersionInfo",
		},
		{
			name:     "/v1/subscribe_events",
			url:      "/v1/subscribe_events",
			expected: "subscribeEvents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := URLToRoute(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBenchmarkParseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "/v1/transactions",
			url:      "/v1/transactions",
			expected: "createTransaction",
		},
		{
			name:     "/v1/transactions/{id}",
			url:      "/v1/transactions/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getTransactionByID",
		},
		{
			name:     "/v1/transaction_results/{id}",
			url:      "/v1/transaction_results/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getTransactionResultByID",
		},
		{
			name:     "/v1/blocks",
			url:      "/v1/blocks",
			expected: "getBlocksByHeight",
		},
		{
			name:     "/v1/blocks/{id}",
			url:      "/v1/blocks/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getBlocksByIDs",
		},
		{
			name:     "/v1/blocks/{id}/payload",
			url:      "/v1/blocks/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76/payload",
			expected: "getBlockPayloadByID",
		},
		{
			name:     "/v1/execution_results/{id}",
			url:      "/v1/execution_results/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getExecutionResultByID",
		},
		{
			name:     "/v1/execution_results",
			url:      "/v1/execution_results",
			expected: "getExecutionResultByBlockID",
		},
		{
			name:     "/v1/collections/{id}",
			url:      "/v1/collections/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getCollectionByID",
		},
		{
			name:     "/v1/scripts",
			url:      "/v1/scripts",
			expected: "executeScript",
		},
		{
			name:     "/v1/accounts/{address}",
			url:      "/v1/accounts/6a587be304c1224c",
			expected: "getAccount",
		},
		{
			name:     "/v1/accounts/{address}/keys/{index}",
			url:      "/v1/accounts/6a587be304c1224c/keys/0",
			expected: "getAccountKeyByIndex",
		},
		{
			name:     "/v1/events",
			url:      "/v1/events",
			expected: "getEvents",
		},
		{
			name:     "/v1/network/parameters",
			url:      "/v1/network/parameters",
			expected: "getNetworkParameters",
		},
		{
			name:     "/v1/node_version_info",
			url:      "/v1/node_version_info",
			expected: "getNodeVersionInfo",
		},
		{
			name:     "/v1/subscribe_events",
			url:      "/v1/subscribe_events",
			expected: "subscribeEvents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			for i := 0; i < 100_000; i++ {
				_, _ = URLToRoute(tt.url)
			}
			t.Logf("%s: %v", tt.name, time.Since(start)/100_000)
		})
	}
}
