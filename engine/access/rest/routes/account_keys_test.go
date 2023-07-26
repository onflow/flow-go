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

// TestGetAccountKeyByID tests local getAccount request.
//
// Runs the following tests:
// 1. Get key by address and ID at latest finalized block
// 2. Get missing key by address and ID at latest finalized block
// 3. Get invalid account.
func TestGetAccountKeyByID(t *testing.T) {
	backend := &mock.API{}

	t.Run("get key by address and ID at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIDRequest(t, account, "0")

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusSealed, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedAccountKeyResponse(account)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get missing key by address and ID at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		keyID := "2"
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeyByIDRequest(t, account, keyID)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusSealed, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		statusCode := 404
		expected := fmt.Sprintf(`
          {
            "code": %d,
            "message": "account key with ID: %s does not exist"
          }
		`, statusCode, keyID)

		assertResponse(t, req, statusCode, expected, backend)
	})

	t.Run("get invalid", func(t *testing.T) {
		tests := []struct {
			url string
			out string
		}{
			{
				accountKeyURL(t, "123", "3"), `{"code":400, "message":"invalid address"}`,
			},
			{
				accountKeyURL(
					t,
					unittest.AddressFixture().String(),
					"foo",
				),
				`{"code":400, "message":"invalid key index: value must be an unsigned 64 bit integer"}`,
			},
		}

		for i, test := range tests {
			req, _ := http.NewRequest("GET", test.url, nil)
			rr, err := executeRequest(req, backend)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}

func accountKeyURL(t *testing.T, address string, keyID string) string {
	u, err := url.ParseRequestURI(
		fmt.Sprintf("/v1/accounts/%s/keys/%s", address, keyID),
	)
	require.NoError(t, err)

	return u.String()
}

func getAccountKeyByIDRequest(t *testing.T, account *flow.Account, keyID string) *http.Request {
	req, err := http.NewRequest("GET", accountKeyURL(t, account.Address.String(), keyID), nil)
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
