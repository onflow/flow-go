package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func accountURL(address string, height string) string {
	u, _ := url.ParseRequestURI(fmt.Sprintf("/v1/accounts/%s", address))
	q := u.Query()

	if height != "" {
		q.Add("height", height)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func TestGetAccount(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by address at latest block", func(t *testing.T) {
		heights := []string{"", "latest"}

		for _, h := range heights {
			account, _ := unittest.AccountFixture()
			req, _ := http.NewRequest("GET", accountURL(account.Address.String(), h), nil)

			backend.Mock.
				On("GetAccountAtLatestBlock", mocks.Anything, account.Address).
				Return(account, nil)

			rr := executeRequest(req, backend)

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

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.JSONEq(t, expected, rr.Body.String())
		}
	})

	t.Run("get by address at height", func(t *testing.T) {
		account, _ := unittest.AccountFixture()
		req, _ := http.NewRequest("GET", accountURL(account.Address.String(), "1337"), nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocks.Anything, account.Address, uint64(1337)).
			Return(account, nil)

		rr := executeRequest(req, backend)

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

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("get invalid", func(t *testing.T) {
		tests := []struct {
			url string
			out string
		}{
			{accountURL("123", ""), `{"code":400, "message":"invalid address"}`},
			{accountURL(unittest.AddressFixture().String(), "foo"), `{"code":400, "message":"invalid height format"}`},
		}

		for i, test := range tests {
			req, _ := http.NewRequest("GET", test.url, nil)
			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}
