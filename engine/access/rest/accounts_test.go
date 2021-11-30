package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const expandableFieldKeys = "keys"
const expandableFieldContracts = "contracts"

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

		req := getAccountRequest(t, account, sealedHeightQueryParam, expandableFieldKeys, expandableFieldContracts)

		backend.Mock.
			On("GetAccountAtLatestBlock", mock2.Anything, account.Address).
			Return(account, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, backend)

	})

	t.Run("get by address at latest finalized block", func(t *testing.T) {

		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		account := accountFixture(t)

		req := getAccountRequest(t, account, finalHeightQueryParam, expandableFieldKeys, expandableFieldContracts)
		backend.Mock.
			On("GetLatestBlockHeader", mock2.Anything, false).
			Return(&block, nil)
		backend.Mock.
			On("GetAccountAtBlockHeight", mock2.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address at height", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height), expandableFieldKeys, expandableFieldContracts)

		backend.Mock.
			On("GetAccountAtBlockHeight", mock2.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address at height condensed", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height))

		backend.Mock.
			On("GetAccountAtBlockHeight", mock2.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedCondensedResponse(account)

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

func expectedExpandedResponse(account *flow.Account) string {
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
			  ],
              "_links":{"_self":"/v1/accounts/%s" },
              "contracts": {"contract1":"Y29udHJhY3Qx", "contract2":"Y29udHJhY3Qy"}
			}`, account.Address, account.Keys[0].PublicKey.String(), account.Address)
}

func expectedCondensedResponse(account *flow.Account) string {
	return fmt.Sprintf(`{
			  "address":"%s",
			  "balance":100,
              "_links":{"_self":"/v1/accounts/%s" },
              "_expandable":{"contracts":"contracts", "keys":"keys"}
			}`, account.Address, account.Address)
}

func getAccountRequest(t *testing.T, account *flow.Account, height string, expandFields ...string) *http.Request {
	req, err := http.NewRequest("GET", accountURL(t, account.Address.String(), height), nil)
	if len(expandFields) > 0 {
		fieldParam := strings.Join(expandFields, ",")
		q := req.URL.Query()
		q.Add(middleware.ExpandQueryParam, fieldParam)
		req.URL.RawQuery = q.Encode()
	}
	require.NoError(t, err)
	return req
}

func accountFixture(t *testing.T) *flow.Account {
	account, err := unittest.AccountFixture()
	require.NoError(t, err)
	return account
}
