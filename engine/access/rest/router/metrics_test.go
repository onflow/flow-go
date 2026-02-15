package router

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	method   string
	name     string
	url      string
	expected string
}

func testCases() []testCase {
	return []testCase{
		{
			method:   http.MethodPost,
			name:     "/v1/transactions",
			url:      "/v1/transactions",
			expected: "createTransaction",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/transactions",
			url:      "/v1/transactions",
			expected: "getTransactionsByBlock",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/transactions/{id}",
			url:      "/v1/transactions/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getTransactionByID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/transactions/{index}",
			url:      "/v1/transactions/12345678",
			expected: "getTransactionByID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/transaction_results",
			url:      "/v1/transaction_results",
			expected: "getTransactionResultsByBlock",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/transaction_results/{id}",
			url:      "/v1/transaction_results/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getTransactionResultByID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/transaction_results/{index}",
			url:      "/v1/transaction_results/12345678",
			expected: "getTransactionResultByID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/blocks",
			url:      "/v1/blocks",
			expected: "getBlocksByHeight",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/blocks/{id}",
			url:      "/v1/blocks/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getBlocksByIDs",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/blocks/{id}/payload",
			url:      "/v1/blocks/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76/payload",
			expected: "getBlockPayloadByID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/execution_results/{id}",
			url:      "/v1/execution_results/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getExecutionResultByID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/execution_results",
			url:      "/v1/execution_results",
			expected: "getExecutionResultByBlockID",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/collections/{id}",
			url:      "/v1/collections/53730d3f3d2d2f46cb910b16db817d3a62adaaa72fdb3a92ee373c37c5b55a76",
			expected: "getCollectionByID",
		},
		{
			method:   http.MethodPost,
			name:     "/v1/scripts",
			url:      "/v1/scripts",
			expected: "executeScript",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/accounts/{address}",
			url:      "/v1/accounts/6a587be304c1224c",
			expected: "getAccount",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/accounts/{address}/balance",
			url:      "/v1/accounts/6a587be304c1224c/balance",
			expected: "getAccountBalance",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/accounts/{address}/keys/{index}",
			url:      "/v1/accounts/6a587be304c1224c/keys/0",
			expected: "getAccountKeyByIndex",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/accounts/{address}/keys",
			url:      "/v1/accounts/6a587be304c1224c/keys",
			expected: "getAccountKeys",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/events",
			url:      "/v1/events",
			expected: "getEvents",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/network/parameters",
			url:      "/v1/network/parameters",
			expected: "getNetworkParameters",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/node_version_info",
			url:      "/v1/node_version_info",
			expected: "getNodeVersionInfo",
		},
		{
			method:   http.MethodGet,
			name:     "/v1/subscribe_events",
			url:      "/v1/subscribe_events",
			expected: "subscribeEvents",
		},
		{
			method:   http.MethodGet,
			name:     "/experimental/v1/accounts/{address}/transactions",
			url:      "/experimental/v1/accounts/6a587be304c1224c/transactions",
			expected: "getAccountTransactions",
		},
	}
}

func TestURLToRoute(t *testing.T) {
	for _, tt := range testCases() {
		t.Run(tt.method+" "+tt.name, func(t *testing.T) {
			got, err := MethodURLToRoute(tt.method, tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBenchmarkURLToRoute(t *testing.T) {
	for _, tt := range testCases() {
		t.Run(tt.method+" "+tt.name, func(t *testing.T) {
			start := time.Now()
			for range 100_000 {
				_, _ = MethodURLToRoute(tt.method, tt.url)
			}
			t.Logf("%s %s: %v", tt.method, tt.name, time.Since(start)/100_000)
		})
	}
}
