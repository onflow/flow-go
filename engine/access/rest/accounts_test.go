package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	restmock "github.com/onflow/flow-go/engine/access/rest/mock"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
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

func TestAccessGetAccount(t *testing.T) {
	backend := &mock.API{}
	restHandler := newAccessRestHandler(backend)

	t.Run("get by address at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountRequest(t, account, sealedHeightQueryParam, expandableFieldKeys, expandableFieldContracts)

		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, true).
			Return(block, flow.BlockStatusSealed, nil)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by address at latest finalized block", func(t *testing.T) {

		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		account := accountFixture(t)

		req := getAccountRequest(t, account, finalHeightQueryParam, expandableFieldKeys, expandableFieldContracts)
		backend.Mock.
			On("GetLatestBlockHeader", mocktestify.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil)
		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by address at height", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height), expandableFieldKeys, expandableFieldContracts)

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by address at height condensed", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height))

		backend.Mock.
			On("GetAccountAtBlockHeight", mocktestify.Anything, account.Address, height).
			Return(account, nil)

		expected := expectedCondensedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
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
			rr, err := executeRequest(req, restHandler)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}

// TestObserverGetAccount tests the get account from observer node
func TestObserverGetAccount(t *testing.T) {
	backend := &mock.API{}
	restForwarder := &restmock.RestServerApi{}

	restHandler, err := newObserverRestHandler(backend, restForwarder)
	assert.NoError(t, err)

	t.Run("get by address at latest sealed block", func(t *testing.T) {
		account := accountFixture(t)

		req := getAccountRequest(t, account, sealedHeightQueryParam, expandableFieldKeys, expandableFieldContracts)

		accountKeys := make([]models.AccountPublicKey, 1)

		sigAlgo := models.SigningAlgorithm("ECDSA_P256")
		hashAlgo := models.HashingAlgorithm("SHA3_256")

		accountKeys[0] = models.AccountPublicKey{
			Index:            "0",
			PublicKey:        account.Keys[0].PublicKey.String(),
			SigningAlgorithm: &sigAlgo,
			HashingAlgorithm: &hashAlgo,
			SequenceNumber:   "0",
			Weight:           "1000",
			Revoked:          false,
		}

		restForwarder.Mock.On("GetAccount",
			request.GetAccount{
				Address: account.Address,
				Height:  request.SealedHeight,
			},
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything).
			Return(models.Account{
				Address: account.Address.String(),
				Balance: fmt.Sprintf("%d", account.Balance),
				Keys:    accountKeys,
				Contracts: map[string]string{
					"contract1": "Y29udHJhY3Qx",
					"contract2": "Y29udHJhY3Qy",
				},
				Expandable: &models.AccountExpandable{},
				Links: &models.Links{
					Self: fmt.Sprintf("/v1/accounts/%s", account.Address),
				},
			}, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by address at latest finalized block", func(t *testing.T) {
		account := accountFixture(t)

		req := getAccountRequest(t, account, finalHeightQueryParam, expandableFieldKeys, expandableFieldContracts)

		accountKeys := make([]models.AccountPublicKey, 1)

		sigAlgo := models.SigningAlgorithm("ECDSA_P256")
		hashAlgo := models.HashingAlgorithm("SHA3_256")

		accountKeys[0] = models.AccountPublicKey{
			Index:            "0",
			PublicKey:        account.Keys[0].PublicKey.String(),
			SigningAlgorithm: &sigAlgo,
			HashingAlgorithm: &hashAlgo,
			SequenceNumber:   "0",
			Weight:           "1000",
			Revoked:          false,
		}

		restForwarder.Mock.On("GetAccount",
			request.GetAccount{
				Address: account.Address,
				Height:  request.FinalHeight,
			},
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything).
			Return(models.Account{
				Address: account.Address.String(),
				Balance: fmt.Sprintf("%d", account.Balance),
				Keys:    accountKeys,
				Contracts: map[string]string{
					"contract1": "Y29udHJhY3Qx",
					"contract2": "Y29udHJhY3Qy",
				},
				Expandable: &models.AccountExpandable{},
				Links: &models.Links{
					Self: fmt.Sprintf("/v1/accounts/%s", account.Address),
				},
			}, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by address at height", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)

		req := getAccountRequest(t, account, fmt.Sprintf("%d", height), expandableFieldKeys, expandableFieldContracts)
		accountKeys := make([]models.AccountPublicKey, 1)

		sigAlgo := models.SigningAlgorithm("ECDSA_P256")
		hashAlgo := models.HashingAlgorithm("SHA3_256")

		accountKeys[0] = models.AccountPublicKey{
			Index:            "0",
			PublicKey:        account.Keys[0].PublicKey.String(),
			SigningAlgorithm: &sigAlgo,
			HashingAlgorithm: &hashAlgo,
			SequenceNumber:   "0",
			Weight:           "1000",
			Revoked:          false,
		}

		restForwarder.Mock.On("GetAccount",
			request.GetAccount{
				Address: account.Address,
				Height:  height,
			},
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything).
			Return(models.Account{
				Address: account.Address.String(),
				Balance: fmt.Sprintf("%d", account.Balance),
				Keys:    accountKeys,
				Contracts: map[string]string{
					"contract1": "Y29udHJhY3Qx",
					"contract2": "Y29udHJhY3Qy",
				},
				Expandable: &models.AccountExpandable{},
				Links: &models.Links{
					Self: fmt.Sprintf("/v1/accounts/%s", account.Address),
				},
			}, nil)

		expected := expectedExpandedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})

	t.Run("get by address at height condensed", func(t *testing.T) {
		var height uint64 = 1337
		account := accountFixture(t)

		req := getAccountRequest(t, account, fmt.Sprintf("%d", height))

		restForwarder.Mock.On("GetAccount",
			request.GetAccount{
				Address: account.Address,
				Height:  height,
			},
			mocktestify.Anything,
			mocktestify.Anything,
			mocktestify.Anything).
			Return(models.Account{
				Address:   account.Address.String(),
				Balance:   fmt.Sprintf("%d", account.Balance),
				Contracts: map[string]string{},
				Expandable: &models.AccountExpandable{
					Keys:      expandableFieldKeys,
					Contracts: expandableFieldContracts,
				},
				Links: &models.Links{
					Self: fmt.Sprintf("/v1/accounts/%s", account.Address),
				},
			}, nil)

		expected := expectedCondensedResponse(account)

		assertOKResponse(t, req, expected, restHandler)
		mocktestify.AssertExpectationsForObjects(t, backend)
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
			rr, err := executeRequest(req, restHandler)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test #%d failed: %v", i, test))
		}
	})
}

func expectedExpandedResponse(account *flow.Account) string {
	return fmt.Sprintf(`{
			  "address":"%s",
			  "balance":"100",
			  "keys":[
				  {
					 "index":"0",
					 "public_key":"%s",
					 "signing_algorithm":"ECDSA_P256",
					 "hashing_algorithm":"SHA3_256",
					 "sequence_number":"0",
					 "weight":"1000",
					 "revoked":false
				  }
			  ],
              "_links":{"_self":"/v1/accounts/%s" },
              "_expandable": {},
              "contracts": {"contract1":"Y29udHJhY3Qx", "contract2":"Y29udHJhY3Qy"}
			}`, account.Address, account.Keys[0].PublicKey.String(), account.Address)
}

func expectedCondensedResponse(account *flow.Account) string {
	return fmt.Sprintf(`{
			  "address":"%s",
			  "balance":"100",
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
