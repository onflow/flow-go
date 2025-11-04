package routes_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetAccountBalance tests local getAccountBalance request.
//
// Test cases:
// 1. Get account balance by address at latest sealed block.
// 2. Get account balance by address at latest finalized block.
// 3. Get account balance by address at height.
// 4. Get account balance by address at the latest sealed block with executor metadata.
func TestGetAccountBalance(t *testing.T) {
	backend := accessmock.NewAPI(t)

	t.Run("get balance by address at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountBalanceRequest(t, account, router.SealedHeightQueryParam, "2", []string{}, "false")

		backend.On("GetLatestBlockHeader", mock.Anything, true).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.On("GetAccountBalanceAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Balance, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountBalanceResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get balance by address at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountBalanceRequest(t, account, router.FinalHeightQueryParam, "2", []string{}, "false")

		backend.On("GetLatestBlockHeader", mock.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil).
			Once()

		backend.On("GetAccountBalanceAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Balance, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountBalanceResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get balance by address at height", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 1337
		req := getAccountBalanceRequest(t, account, fmt.Sprintf("%d", height), "1", []string{}, "false")

		backend.On("GetAccountBalanceAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Balance, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountBalanceResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get balance by address at latest sealed block with metadata", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		metadata := &access.ExecutorMetadata{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutorIDs:       unittest.IdentifierListFixture(2),
		}

		req := getAccountBalanceRequest(t, account, router.SealedHeightQueryParam, "2", metadata.ExecutorIDs.Strings(), "true")

		backend.On("GetLatestBlockHeader", mock.Anything, true).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.On("GetAccountBalanceAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Balance, metadata, nil).
			Once()

		expected := expectedAccountBalanceResponse(account, metadata)
		router.AssertOKResponse(t, req, expected, backend)
	})
}

// TestGetAccountBalanceErrors verifies that the getAccountBalance endpoint
// correctly returns appropriate HTTP error codes and messages in various failure scenarios.
//
// Test cases:
//  1. A request with an invalid account address return http.StatusBadRequest.
//  2. A request where GetLatestBlockHeader fails for the "sealed" height return http.StatusNotFound.
//  3. A request where GetAccountBalanceAtBlockHeight fails for a valid block height return http.StatusNotFound.
func TestGetAccountBalanceErrors(t *testing.T) {
	backend := accessmock.NewAPI(t)

	tests := []struct {
		name   string
		url    string
		setup  func()
		status int
		out    string
	}{
		{
			name:   "invalid account address",
			url:    accountBalanceURL(t, "123", "", "2", []string{}, "true"),
			setup:  func() {},
			status: http.StatusBadRequest,
			out:    `{"code":400, "message":"invalid address"}`,
		},
		{
			name: "GetLatestBlockHeader fails for sealed height",
			url:  accountBalanceURL(t, unittest.AddressFixture().String(), router.SealedHeightQueryParam, "2", []string{}, "false"),
			setup: func() {
				backend.On("GetLatestBlockHeader", mock.Anything, true).
					Return(nil, flow.BlockStatusUnknown, fmt.Errorf("latest block header error")).
					Once()
			},
			status: http.StatusNotFound,
			out:    fmt.Sprintf(`{"code":404, "message":"block with height: %d does not exist"}`, request.SealedHeight),
		},
		{
			name: "GetAccountBalanceAtBlockHeight fails for valid height",
			url:  accountBalanceURL(t, unittest.AddressFixture().String(), "100", "2", []string{}, "false"),
			setup: func() {
				backend.On("GetAccountBalanceAtBlockHeight", mock.Anything, mock.Anything, uint64(100), mock.Anything).
					Return(uint64(0), nil, fmt.Errorf("database error")).
					Once()
			},
			status: http.StatusNotFound,
			out:    `{"code":404, "message":"failed to get account balance, reason: database error"}`,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup()
			req, _ := http.NewRequest("GET", test.url, nil)
			rr := router.ExecuteRequest(req, backend)

			require.Equal(t, test.status, rr.Code, fmt.Sprintf("test #%d failed: %v", i, test))
			require.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		})
	}
}

func accountBalanceURL(t *testing.T,
	address string,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/v1/accounts/%s/balance", address))
	require.NoError(t, err)
	q := u.Query()

	if height != "" {
		q.Add("block_height", height)
	}

	q.Add(router.AgreeingExecutorsCountQueryParam, agreeingExecutorsCount)

	if len(requiredExecutors) > 0 {
		q.Add(router.RequiredExecutorIdsQueryParam, strings.Join(requiredExecutors, ","))
	}

	if len(includeExecutorMetadata) > 0 {
		q.Add(router.IncludeExecutorMetadataQueryParam, includeExecutorMetadata)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func getAccountBalanceRequest(
	t *testing.T,
	account *flow.Account,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) *http.Request {
	req, err := http.NewRequest(
		"GET",
		accountBalanceURL(t, account.Address.String(), height, agreeingExecutorsCount, requiredExecutors, includeExecutorMetadata),
		nil,
	)

	require.NoError(t, err)
	return req
}

// expectedAccountBalanceResponse returns the expected JSON response string.
// If metadata is provided, it includes the executor metadata fields nested
// under "metadata.executor_metadata", matching the actual API structure.
func expectedAccountBalanceResponse(account *flow.Account, metadata *access.ExecutorMetadata) string {
	if metadata != nil {
		executorIDs := make([]string, len(metadata.ExecutorIDs))
		for i, id := range metadata.ExecutorIDs {
			executorIDs[i] = fmt.Sprintf(`"%s"`, id)
		}

		return fmt.Sprintf(`
      {
        "balance":"%d",
        "metadata": {
          "executor_metadata": {
            "execution_result_id": "%s",
            "executor_ids": [%s]
          }
        }
      }`,
			account.Balance,
			metadata.ExecutionResultID,
			strings.Join(executorIDs, ", "),
		)
	}

	return fmt.Sprintf(`
      {
        "balance":"%d"
      }`,
		account.Balance,
	)
}
