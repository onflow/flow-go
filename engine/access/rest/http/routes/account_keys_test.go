package routes_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/router"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGetAccountKeyByIndex tests local getAccountKeyByIndex request.
//
// Test cases:
// 1. Get key by address and index at latest sealed block.
// 2. Get key by address and index at latest finalized block.
// 3. Get key by address and index at height.
// 4. Get key by address and index with executor metadata included.
func TestGetAccountKeyByIndex(t *testing.T) {
	backend := accessmock.NewAPI(t)

	t.Run("get key by address and index at latest sealed block", func(t *testing.T) {
		account, err := unittest.AccountFixture()
		require.NoError(t, err)

		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		var keyIndex uint32 = 0
		keyByIndex := findAccountKeyByIndex(account.Keys, keyIndex)

		req := getAccountKeyByIndexRequest(t, account, "0", router.SealedHeightQueryParam, "2", []string{}, "false")

		backend.On("GetLatestBlockHeader", mock.Anything, true).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.On("GetAccountKeyAtBlockHeight", mock.Anything, account.Address, keyIndex, height, mock.Anything).
			Return(keyByIndex, &accessmodel.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountKeyResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get key by address and index at latest finalized block", func(t *testing.T) {
		account, err := unittest.AccountFixture()
		require.NoError(t, err)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		var keyIndex uint32 = 0
		keyByIndex := findAccountKeyByIndex(account.Keys, keyIndex)

		req := getAccountKeyByIndexRequest(t, account, "0", router.FinalHeightQueryParam, "2", []string{}, "false")

		backend.On("GetLatestBlockHeader", mock.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil).
			Once()

		backend.On("GetAccountKeyAtBlockHeight", mock.Anything, account.Address, keyIndex, height, mock.Anything).
			Return(keyByIndex, &accessmodel.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountKeyResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get key by address and index at height", func(t *testing.T) {
		var height uint64 = 100
		account, err := unittest.AccountFixture()
		require.NoError(t, err)
		req := getAccountKeyByIndexRequest(t, account, "0", fmt.Sprintf("%d", height), "2", []string{}, "false")

		var keyIndex uint32 = 0
		keyByIndex := findAccountKeyByIndex(account.Keys, keyIndex)

		backend.On("GetAccountKeyAtBlockHeight", mock.Anything, account.Address, keyIndex, height, mock.Anything).
			Return(keyByIndex, &accessmodel.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountKeyResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get key by address and index with executor metadata included", func(t *testing.T) {
		account, err := unittest.AccountFixture()
		require.NoError(t, err)
		var height uint64 = 100
		var keyIndex uint32 = 0
		keyByIndex := findAccountKeyByIndex(account.Keys, keyIndex)

		metadata := &accessmodel.ExecutorMetadata{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutorIDs:       unittest.IdentifierListFixture(2),
		}

		req := getAccountKeyByIndexRequest(t, account, "0", fmt.Sprintf("%d", height), "2", []string{}, "true")

		backend.On("GetAccountKeyAtBlockHeight", mock.Anything, account.Address, keyIndex, height, mock.Anything).
			Return(keyByIndex, metadata, nil).
			Once()

		expected := expectedAccountKeyResponse(account, metadata)
		router.AssertOKResponse(t, req, expected, backend)
	})
}

// TestGetAccountByIndexErrors verifies that the GetAccountKeyByIndex endpoint
// correctly returns appropriate HTTP error codes and messages in various failure scenarios.
//
// Test cases:
//  1. A request with an invalid account address returns http.StatusBadRequest.
//  2. Simulated failure when fetching the latest block header for a "sealed" height
//     to ensure that any propagated errors are correctly returned as http.StatusInternalServerError.
//
// 3. A request where GetAccountKeyAtBlockHeight fails for a valid block height returns http.StatusNotFound.
func TestGetAccountByIndexErrors(t *testing.T) {
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
			url:    accountKeyURL(t, "123", "3", "100", "2", []string{}, "false"),
			setup:  func() {},
			status: http.StatusBadRequest,
			out:    `{"code":400, "message":"invalid address"}`,
		},
		{
			name: "GetLatestBlockHeader fails for sealed height",
			url:  accountKeyURL(t, unittest.AddressFixture().String(), "0", router.SealedHeightQueryParam, "2", []string{}, "false"),
			setup: func() {
				backend.On("GetLatestBlockHeader", mock.Anything, true).
					Return(nil, flow.BlockStatusUnknown, access.NewInternalError(fmt.Errorf("internal server error"))).
					Once()
			},
			status: http.StatusInternalServerError,
			out:    `{"code":500, "message":"internal error: internal server error"}`,
		},
		{
			name: "GetAccountKeyAtBlockHeight fails for valid height",
			url:  accountKeyURL(t, unittest.AddressFixture().String(), "2", "100", "2", []string{}, "false"),
			setup: func() {
				backend.On("GetAccountKeyAtBlockHeight", mock.Anything, mock.Anything, uint32(2), uint64(100), mock.Anything).
					Return(nil, nil, access.NewDataNotFoundError("block", fmt.Errorf("not found"))).
					Once()
			},
			status: http.StatusNotFound,
			out:    `{"code":404,  "message":"Flow resource not found: data not found for block: not found"}`,
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

// TestGetAccountKeys tests local GetAccountKeys request.
//
// Test cases:
// 1. Get keys by address at latest sealed block.
// 2. Get keys by address at latest finalized block.
// 3. Get keys by address at height.
// 4. Get keys by address with executor metadata included in the response.
func TestGetAccountKeys(t *testing.T) {
	backend := accessmock.NewAPI(t)

	t.Run("get keys by address at latest sealed block", func(t *testing.T) {
		account := accountWithKeysFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeysRequest(t, account, router.SealedHeightQueryParam, "2", []string{}, "false")

		backend.On("GetLatestBlockHeader", mock.Anything, true).
			Return(block, flow.BlockStatusSealed, nil).
			Once()

		backend.On("GetAccountKeysAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Keys, &accessmodel.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountKeysResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get keys by address at latest finalized block", func(t *testing.T) {
		account := accountWithKeysFixture(t)
		var height uint64 = 100
		block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		req := getAccountKeysRequest(t, account, router.FinalHeightQueryParam, "2", []string{}, "false")

		backend.On("GetLatestBlockHeader", mock.Anything, false).
			Return(block, flow.BlockStatusFinalized, nil).
			Once()

		backend.On("GetAccountKeysAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Keys, &accessmodel.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountKeysResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get keys by address at height", func(t *testing.T) {
		var height uint64 = 1337
		account := accountWithKeysFixture(t)
		req := getAccountKeysRequest(t, account, fmt.Sprintf("%d", height), "2", []string{}, "false")

		backend.On("GetAccountKeysAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Keys, &accessmodel.ExecutorMetadata{}, nil).
			Once()

		expected := expectedAccountKeysResponse(account, nil)
		router.AssertOKResponse(t, req, expected, backend)
	})

	t.Run("get keys by address with metadata", func(t *testing.T) {
		account := accountWithKeysFixture(t)
		var height uint64 = 100
		req := getAccountKeysRequest(t, account, fmt.Sprintf("%d", height), "2", []string{}, "true")

		metadata := &accessmodel.ExecutorMetadata{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutorIDs:       unittest.IdentifierListFixture(2),
		}

		backend.On("GetAccountKeysAtBlockHeight", mock.Anything, account.Address, height, mock.Anything).
			Return(account.Keys, metadata, nil).
			Once()

		expected := expectedAccountKeysResponse(account, metadata)
		router.AssertOKResponse(t, req, expected, backend)
	})
}

// TestGetAccountKeysErrors verifies that the GetAccountKeys endpoint
// correctly returns appropriate HTTP error codes and messages in various failure scenarios.
//
// Test cases:
//  1. A request with an invalid account address returns http.StatusBadRequest.
//  2. Simulated failure when fetching the latest block header for a "sealed" height
//     to ensure that any propagated errors are correctly returned as http.StatusInternalServerError.
//
// 3. A request where GetAccountKeysAtBlockHeight fails for a valid block height returns http.StatusNotFound.
func TestGetAccountKeysErrors(t *testing.T) {
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
			url:    accountKeysURL(t, "123", "100", "2", []string{}, "false"),
			setup:  func() {},
			status: http.StatusBadRequest,
			out:    `{"code":400, "message":"invalid address"}`,
		},
		{
			name: "GetLatestBlockHeader fails for sealed height",
			url:  accountKeysURL(t, unittest.AddressFixture().String(), router.SealedHeightQueryParam, "2", []string{}, "false"),
			setup: func() {
				backend.On("GetLatestBlockHeader", mock.Anything, true).
					Return(nil, flow.BlockStatusUnknown, access.NewInternalError(fmt.Errorf("internal server error"))).
					Once()
			},
			status: http.StatusInternalServerError,
			out:    `{"code":500, "message":"internal error: internal server error"}`,
		},
		{
			name: "GetAccountKeysAtBlockHeight fails for valid height",
			url:  accountKeysURL(t, unittest.AddressFixture().String(), "100", "2", []string{}, "false"),
			setup: func() {
				backend.On("GetAccountKeysAtBlockHeight", mock.Anything, mock.Anything, uint64(100), mock.Anything).
					Return(nil, nil, access.NewDataNotFoundError("block", fmt.Errorf("not found"))).
					Once()
			},
			status: http.StatusNotFound,
			out:    `{"code":404, "message":"Flow resource not found: data not found for block: not found"}`,
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

func accountKeyURL(t *testing.T,
	address string,
	index string,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) string {
	u, err := url.ParseRequestURI(
		fmt.Sprintf("/v1/accounts/%s/keys/%s", address, index),
	)
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

func accountKeysURL(
	t *testing.T,
	address string,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) string {
	u, err := url.ParseRequestURI(
		fmt.Sprintf("/v1/accounts/%s/keys", address),
	)
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

func getAccountKeyByIndexRequest(
	t *testing.T,
	account *flow.Account,
	index string,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) *http.Request {
	req, err := http.NewRequest(
		"GET",
		accountKeyURL(t, account.Address.String(), index, height, agreeingExecutorsCount, requiredExecutors, includeExecutorMetadata),
		nil,
	)
	require.NoError(t, err)

	return req
}

func getAccountKeysRequest(
	t *testing.T,
	account *flow.Account,
	height string,
	agreeingExecutorsCount string,
	requiredExecutors []string,
	includeExecutorMetadata string,
) *http.Request {
	req, err := http.NewRequest(
		"GET",
		accountKeysURL(t, account.Address.String(), height, agreeingExecutorsCount, requiredExecutors, includeExecutorMetadata),
		nil,
	)
	require.NoError(t, err)

	return req
}

func expectedAccountKeyResponse(account *flow.Account, metadata *accessmodel.ExecutorMetadata) string {
	metadataSection := expectedMetadata(metadata)

	return fmt.Sprintf(`
       {
         "index": "0",
         "public_key": "%s",
         "signing_algorithm": "ECDSA_P256",
         "hashing_algorithm": "SHA3_256",
         "sequence_number": "0",
         "weight": "1000",
         "revoked": false%s
       }`,
		account.Keys[0].PublicKey.String(),
		metadataSection,
	)
}

// expectedAccountKeysResponse returns the expected JSON response string.
// If metadata is provided, it includes metadata fields under each key.
func expectedAccountKeysResponse(account *flow.Account, metadata *accessmodel.ExecutorMetadata) string {
	metadataSection := expectedMetadata(metadata)

	return fmt.Sprintf(`
	{
		"keys": [
			{
				"index": "0",
				"public_key": "%s",
				"signing_algorithm": "ECDSA_P256",
				"hashing_algorithm": "SHA3_256",
				"sequence_number": "0",
				"weight": "1000",
				"revoked": false
			},
			{
				"index": "1",
				"public_key": "%s",
				"signing_algorithm": "ECDSA_P256",
				"hashing_algorithm": "SHA3_256",
				"sequence_number": "0",
				"weight": "500",
				"revoked": false
			}
		]%s
	}`,
		account.Keys[0].PublicKey.String(),
		account.Keys[1].PublicKey.String(),
		metadataSection,
	)
}

func findAccountKeyByIndex(keys []flow.AccountPublicKey, keyIndex uint32) *flow.AccountPublicKey {
	for _, key := range keys {
		if key.Index == keyIndex {
			return &key
		}
	}
	return &flow.AccountPublicKey{}
}

func accountWithKeysFixture(t *testing.T) *flow.Account {
	account, err := unittest.AccountFixture()
	require.NoError(t, err)

	key2, err := unittest.AccountKeyFixture(128, crypto.ECDSAP256, hash.SHA3_256)
	require.NoError(t, err)

	account.Keys = append(account.Keys, key2.PublicKey(500))
	account.Keys[1].Index = 1

	return account
}
