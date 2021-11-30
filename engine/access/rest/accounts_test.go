package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func accountURL(t *testing.T, address string, height string) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/v1/accounts/%s", address))
	require.NoError(t, err)
	q := u.Query()

	if height != "" {
		q.Add("block_height", height)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func TestGetAccount(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by address at latest sealed block", func(t *testing.T) {

		account := accountFixture(t)

		req := getAccountRequest(t, account, sealedHeightQueryParam)

		backend.Mock.
			On("GetAccountAtLatestBlock", mock2.Anything, account.Address).
			Return(account, nil)

		expected := expectedResponse(account)

		assertOKResponse(t, req, expected, backend)

	})

	t.Run("get by address at latest finalized block", func(t *testing.T) {

		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		account := accountFixture(t)

		req := getAccountRequest(t, account, finalHeightQueryParam)
		backend.Mock.
			On("GetLatestBlockHeader", mock2.Anything, false).
			Return(&block, nil)
		backend.Mock.
			On("GetAccountAtBlockHeight", mock2.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedResponse(account)

		assertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address at height", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height))

		backend.Mock.
			On("GetAccountAtBlockHeight", mock2.Anything, account.Address, height).
			Return(account, nil)

		expected := fmt.Sprintf(`{
			   "address":"%s",
			   "balance":100,
			   "keys":[
				  {
					 "index":0,
					 "public_key":"%s",
					 "signing_algorithm":"ECDSA_P256",
					 "hashing_algorithm":"SHA3_256",
					 "sequence_number":0,
					 "weight":1000,
					 "revoked":false
				  }
			   ]
			}`, account.Address, account.Keys[0].PublicKey.String())

		assertOKResponse(t, req, expected, backend)
	})

	t.Run("get invalid", func(t *testing.T) {
		tests := []struct {
			url string
			out string
		}{
			{accountURL(t, "123", ""), `{"code":400, "message":"invalid address"}`},
			{accountURL(t, unittest.AddressFixture().String(), "foo"), `{"code":400, "message":"invalid height format"}`},
		}

		for i, test := range tests {
			req, _ := http.NewRequest("GET", test.url, nil)
			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}

func expectedResponse(account *flow.Account) string {
	return fmt.Sprintf(`{
			  "address":"%s",
			  "balance":100,
			  "keys":[
				  {
					 "index":0,
					 "public_key":"%s",
					 "signing_algorithm":"ECDSA_P256",
					 "hashing_algorithm":"SHA3_256",
					 "sequence_number":0,
					 "weight":1000,
					 "revoked":false
				  }
			  ]
			}`, account.Address, account.Keys[0].PublicKey.String())
}

func getAccountRequest(t *testing.T, account *flow.Account, height string) *http.Request {
	req, err := http.NewRequest("GET", accountURL(t, account.Address.String(), height), nil)
	require.NoError(t, err)
	return req
}

func accountFixture(t *testing.T) *flow.Account {
	account, err := unittest.AccountFixture()
	require.NoError(t, err)
	return account
}
