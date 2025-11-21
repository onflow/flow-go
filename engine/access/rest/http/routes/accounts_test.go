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
	"github.com/onflow/flow-go/engine/access/rest/common/middleware"
	"github.com/onflow/flow-go/engine/access/rest/router"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const expandableFieldKeys = "keys"
const expandableFieldContracts = "contracts"

// TestAccessGetAccount tests local getAccount request.
//
// Test cases:
// 1. Get account by address at latest sealed block.
// 2. Get account by address at latest finalized block.
// 3. Get account by address at height.
// 4. Get account by address at height condensed.
// 5. Get account by address with executor metadata included.
func TestAccessGetAccount(t *testing.T) {
	backend := accessmock.NewAPI(t)

	t.Run("get by address at latest sealed block", func(t *testing.T) {
		account, err := unittest.AccountFixture()
		require.NoError(t, err)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountRequest(t, account, router.SealedHeightQueryParam, "2", []string{}, "false", expandableFieldKeys, expandableFieldContracts)

		backend.On("GetLatestBlockHeader", mock.Anything, true).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.On("GetAccountAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedExpandedResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address at latest finalized block", func(t *testing.T) {
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		account, err := unittest.AccountFixture()
		require.NoError(t, err)

		req := getAccountRequest(t, account, router.FinalHeightQueryParam, "2", []string{}, "false", expandableFieldKeys, expandableFieldContracts)
		backend.On("GetLatestBlockHeader", mock.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil).
			Once()
		backend.On("GetAccountAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedExpandedResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address at height", func(t *testing.T) {
		var height uint64 = 1337
		account, err := unittest.AccountFixture()
		require.NoError(t, err)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height), "2", []string{}, "false", expandableFieldKeys, expandableFieldContracts)

		backend.On("GetAccountAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedExpandedResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address at height condensed", func(t *testing.T) {
		var height uint64 = 100
		account, err := unittest.AccountFixture()
		require.NoError(t, err)
		req := getAccountRequest(t, account, fmt.Sprintf("%d", height), "2", []string{}, "false")

		backend.On("GetAccountAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account, &access.ExecutorMetadata{}, nil).
			Once()

		expected := expectedCondensedResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get by address with executor metadata included", func(t *testing.T) {
		var height uint64 = 100
		account, err := unittest.AccountFixture()
		require.NoError(t, err)

		metadata := &access.ExecutorMetadata{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutorIDs:       unittest.IdentifierListFixture(2),
		}

		req := getAccountRequest(t, account, fmt.Sprintf("%d", height), "2", []string{}, "true")
		backend.On("GetAccountAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account, metadata, nil).
			Once()

		expected := expectedCondensedResponse(account, metadata)
		router.AssertOKResponse(t, req, expected, backend)
	})
}

// TestGetAccountErrors verifies that the GetAccount endpoint
// correctly returns appropriate HTTP error codes and messages
// in various failure scenarios.
//
// Test cases:
//  1. A request with an invalid account address returns http.StatusBadRequest.
//  2. A request where GetLatestBlockHeader fails for the "sealed" height returns http.StatusInternalServerError.
//  3. A request where GetAccountAtBlockHeight fails for a valid block height returns http.StatusInternalServerError.
//
// TODO(#7650): These tests will be updated in https://github.com/onflow/flow-go/pull/8141
func TestGetAccountErrors(t *testing.T) {
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
			url:    accountURL(t, "123", "100", "2", []string{}, "false"),
			setup:  func() {},
			status: http.StatusBadRequest,
			out:    `{"code":400, "message":"invalid address"}`,
		},
		{
			name: "GetLatestBlockHeader fails for sealed height",
			url:  accountURL(t, unittest.AddressFixture().String(), router.SealedHeightQueryParam, "2", []string{}, "false"),
			setup: func() {
				backend.On("GetLatestBlockHeader", mock.Anything, true).
					Return(nil, flow.BlockStatusUnknown, fmt.Errorf("latest block header error")).
					Once()
			},
			status: http.StatusInternalServerError,
			out:    `{"code":500, "message":"internal server error"}`,
		},
		{
			name: "GetAccountAtBlockHeight fails for valid height",
			url:  accountURL(t, unittest.AddressFixture().String(), "100", "2", []string{}, "false"),
			setup: func() {
				backend.On("GetAccountAtBlockHeight", mock.Anything, mock.Anything, uint64(100), mock.Anything).
					Return(nil, nil, fmt.Errorf("database error")).
					Once()
			},
			status: http.StatusInternalServerError,
			out:    `{"code":500, "message":"internal server error"}`,
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

func expectedExpandedResponse(account *flow.Account, metadata *access.ExecutorMetadata) string {
	metadataSection := expectedMetadata(metadata)

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
            "contracts": {"contract1":"Y29udHJhY3Qx", "contract2":"Y29udHJhY3Qy"}%s
			}`, account.Address, account.Keys[0].PublicKey.String(), account.Address, metadataSection)
}

func expectedCondensedResponse(account *flow.Account, metadata *access.ExecutorMetadata) string {
	metadataSection := expectedMetadata(metadata)

	return fmt.Sprintf(`{
			  "address":"%s",
			  "balance":"100",
            "_links":{"_self":"/v1/accounts/%s" },
            "_expandable":{"contracts":"contracts", "keys":"keys"}%s
			}`, account.Address, account.Address, metadataSection)
}

func getAccountRequest(
	t *testing.T,
	account *flow.Account,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
	expandFields ...string) *http.Request {
	req, err := http.NewRequest("GET", accountURL(t, account.Address.String(), height, agreeingExecutorsCount, requiredExecutors, includeExecutorMetadata), nil)
	if len(expandFields) > 0 {
		fieldParam := strings.Join(expandFields, ",")
		q := req.URL.Query()
		q.Add(middleware.ExpandQueryParam, fieldParam)
		req.URL.RawQuery = q.Encode()
	}

	require.NoError(t, err)
	return req
}

func accountURL(
	t *testing.T,
	address string,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/v1/accounts/%s", address))
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
