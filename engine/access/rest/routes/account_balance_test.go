package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetAccountBalance tests local getAccountBalance request.
//
// Runs the following tests:
// 1. Get account balance by address at latest sealed block.
// 2. Get account balance by address at latest finalized block.
// 3. Get account balance by address at height.
// 4. Get invalid account balance.
func TestGetAccountBalance(t *testing.T) {
	backend := mock.NewAPI(t)

	t.Run("get balance by address at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountBalanceRequest(t, account, sealedHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, true).
			Return(block, flow.BlockStatusSealed, nil)

		backend.Mock.
			On("GetAccountBalanceAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account.Balance, nil)

		expected := expectedAccountBalanceResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get balance by address at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountBalanceRequest(t, account, finalHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil)

		backend.Mock.
			On("GetAccountBalanceAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account.Balance, nil)

		expected := expectedAccountBalanceResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get balance by address at height", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 1337
		req := getAccountBalanceRequest(t, account, fmt.Sprintf("%d", height))

		backend.Mock.
			On("GetAccountBalanceAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account.Balance, nil)

		expected := expectedAccountBalanceResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get invalid", func(t *testing.T) {
		tests := []struct {
			url string
			out string
		}{
			{accountBalanceURL(t, "123", ""), `{"code":400, "message":"invalid address"}`},
			{accountBalanceURL(t, unittest.AddressFixture().String(), "foo"), `{"code":400, "message":"invalid height format"}`},
		}

		for i, test := range tests {
			req, _ := http.NewRequest("GET", test.url, nil)
			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}

func accountBalanceURL(t *testing.T, address string, height string) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/v1/accounts/%s/balance", address))
	require.NoError(t, err)
	q := u.Query()

	if height != "" {
		q.Add("block_height", height)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func getAccountBalanceRequest(t *testing.T, account *flow.Account, height string) *http.Request {
	req, err := http.NewRequest(
		"GET",
		accountBalanceURL(t, account.Address.String(), height),
		nil,
	)

	require.NoError(t, err)
	return req
}

func expectedAccountBalanceResponse(account *flow.Account) string {
	return fmt.Sprintf(`
        {
          "balance":"%d"
        }`,
		account.Balance,
	)
}
