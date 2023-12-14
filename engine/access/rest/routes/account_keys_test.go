package routes

import (
	"fmt"
	"math"
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

// TestGetAccountKeyByIndex tests local getAccountKeyByIndex request.
//
// Runs the following tests:
// 1. Get key by address and index at latest sealed block.
// 2. Get key by address and index at latest finalized block.
// 3. Get missing key by address and index at latest sealed block.
// 4. Get missing key by address and index at latest finalized block.
// 5. Get key by missing address and index at latest sealed block.
// 6. Get key by missing address and index at latest finalized block.
// 7. Get key by address and index at height.
// 8. Get key by address and index at missing block.
func TestGetAccountKeyByIndex(t *testing.T) {
	backend := mock.NewAPI(t)

	t.Run("get key by address and index at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIndexRequest(t, account, "0", sealedHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, true).
			Return(block, flow.BlockStatusSealed, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedAccountKeyResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get key by address and index at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIndexRequest(t, account, "0", finalHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedAccountKeyResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get missing key by address and index at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		index := "2"
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIndexRequest(t, account, index, sealedHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, true).
			Return(block, flow.BlockStatusSealed, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		statusCode := 404
		expected := fmt.Sprintf(`
          {
            "code": %d,
            "message": "account key with index: %s does not exist"
          }
		`, statusCode, index)

		assertResponse(t, req, statusCode, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get missing key by address and index at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		index := "2"
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIndexRequest(t, account, index, finalHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		statusCode := 404
		expected := fmt.Sprintf(`
          {
            "code": %d,
            "message": "account key with index: %s does not exist"
          }
		`, statusCode, index)

		assertResponse(t, req, statusCode, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get key by missing address and index at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		index := "2"
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIndexRequest(t, account, index, sealedHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, true).
			Return(block, flow.BlockStatusSealed, nil)

		err := fmt.Errorf("account with address: %s does not exist", account.Address)
		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(nil, err)

		statusCode := 404
		expected := fmt.Sprintf(`
          {
            "code": %d,
            "message": "account with address: %s does not exist"
          }
		`, statusCode, account.Address)

		assertResponse(t, req, statusCode, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get key by missing address and index at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		index := "2"
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIndexRequest(t, account, index, finalHeightQueryParam)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil)

		err := fmt.Errorf("account with address: %s does not exist", account.Address)
		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(nil, err)

		statusCode := 404
		expected := fmt.Sprintf(`
          {
            "code": %d,
            "message": "account with address: %s does not exist"
          }
		`, statusCode, account.Address)

		assertResponse(t, req, statusCode, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get key by address and index at height", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)
		req := getAccountKeyByIndexRequest(t, account, "0", "1337")

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedAccountKeyResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get key by address and index at missing block", func(t *testing.T) {
		backend := mock.NewAPI(t)
		account := accountFixture(t)
		const finalHeight uint64 = math.MaxUint64 - 2

		req := getAccountKeyByIndexRequest(t, account, "0", finalHeightQueryParam)

		err := fmt.Errorf("block with height: %d does not exist", finalHeight)
		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(nil, flow.BlockStatusUnknown, err)

		statusCode := 404
		expected := fmt.Sprintf(`
			  {
				"code": %d,
				"message": "block with height: %d does not exist"
			  }
			`, statusCode, finalHeight)

		assertResponse(t, req, statusCode, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	tests := []struct {
		name string
		url  string
		out  string
	}{
		{
			"get key with invalid address",
			accountKeyURL(t, "123", "3", "100"),
			`{"code":400, "message":"invalid address"}`,
		},
		{
			"get key with invalid index",
			accountKeyURL(
				t,
				unittest.AddressFixture().String(),
				"foo",
				"100",
			),
			`{"code":400, "message":"invalid key index: value must be an unsigned 64 bit integer"}`,
		},
		{
			"get key with invalid height",
			accountKeyURL(
				t,
				unittest.AddressFixture().String(),
				"2",
				"-100",
			),
			`{"code":400, "message":"invalid height format"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", test.url, nil)
			rr := executeRequest(req, backend)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String())
		})
	}
}

func accountKeyURL(t *testing.T, address string, index string, height string) string {
	u, err := url.ParseRequestURI(
		fmt.Sprintf("/v1/accounts/%s/keys/%s", address, index),
	)
	require.NoError(t, err)
	q := u.Query()

	if height != "" {
		q.Add("block_height", height)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func getAccountKeyByIndexRequest(
	t *testing.T,
	account *flow.Account,
	index string,
	height string,
) *http.Request {
	req, err := http.NewRequest(
		"GET",
		accountKeyURL(t, account.Address.String(), index, height),
		nil,
	)
	require.NoError(t, err)

	return req
}

func expectedAccountKeyResponse(account *flow.Account) string {
	return fmt.Sprintf(`
        {
          "index":"0",
          "public_key":"%s",
          "signing_algorithm":"ECDSA_P256",
          "hashing_algorithm":"SHA3_256",
          "sequence_number":"0",
          "weight":"1000",
          "revoked":false
        }`,
		account.Keys[0].PublicKey.String(),
	)
}
