package routes_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	extendedmock "github.com/onflow/flow-go/access/backends/extended/mock"
	"github.com/onflow/flow-go/engine/access/rest/router"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/utils/unittest"
)

type nftTransfersURLParams struct {
	limit         string
	cursor        string
	tokenType     string
	sourceAddr    string
	recipientAddr string
	role          string
	expand        string
}

func accountNFTTransfersURL(t *testing.T, address string, params nftTransfersURLParams) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/accounts/%s/nft/transfers", address))
	require.NoError(t, err)
	q := u.Query()
	if params.limit != "" {
		q.Add("limit", params.limit)
	}
	if params.cursor != "" {
		q.Add("cursor", params.cursor)
	}
	if params.tokenType != "" {
		q.Add("token_type", params.tokenType)
	}
	if params.sourceAddr != "" {
		q.Add("source_address", params.sourceAddr)
	}
	if params.recipientAddr != "" {
		q.Add("recipient_address", params.recipientAddr)
	}
	if params.role != "" {
		q.Add("role", params.role)
	}
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func TestGetAccountNonFungibleTokenTransfers(t *testing.T) {
	address := unittest.AddressFixture()
	txID := unittest.IdentifierFixture()
	sourceAddr := unittest.RandomAddressFixture()
	recipientAddr := unittest.RandomAddressFixture()

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      1000,
					BlockTimestamp:   1700000000000,
					TransactionIndex: 3,
					EventIndices:     []uint32{5, 6},
					TokenType:        "A.1654653399040a61.MyNFT",
					ID:               42,
					SourceAddress:    sourceAddr,
					RecipientAddress: recipientAddr,
				},
			},
			NextCursor: &accessmodel.TransferCursor{
				BlockHeight:      999,
				TransactionIndex: 0,
				EventIndex:       1,
			},
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		ts := time.UnixMilli(1700000000000).UTC().Format(time.RFC3339Nano)
		expectedCursorStr := testEncodeTransferCursor(t, 999, 0, 1)
		expected := fmt.Sprintf(`{
			"transfers": [
				{
					"transaction_id": "%s",
					"block_height": "1000",
					"timestamp": "%s",
					"transaction_index": "3",
					"event_indices": ["5", "6"],
					"token_type": "A.1654653399040a61.MyNFT",
					"nft_id": "42",
					"source_address": "%s",
					"recipient_address": "%s",
					"_expandable": {"transaction": "transaction", "result": "result"}
				}
			],
			"next_cursor": "%s"
		}`, txID.String(), ts, sourceAddr.Hex(), recipientAddr.Hex(), expectedCursorStr)

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      500,
					BlockTimestamp:   1698000000000,
					TransactionIndex: 1,
					EventIndices:     []uint32{0},
					TokenType:        "A.1654653399040a61.MyNFT",
					ID:               7,
					SourceAddress:    sourceAddr,
					RecipientAddress: recipientAddr,
				},
			},
			NextCursor: nil,
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(10),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{limit: "10"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		ts := time.UnixMilli(1698000000000).UTC().Format(time.RFC3339Nano)
		expected := fmt.Sprintf(`{
			"transfers": [
				{
					"transaction_id": "%s",
					"block_height": "500",
					"timestamp": "%s",
					"transaction_index": "1",
					"event_indices": ["0"],
					"token_type": "A.1654653399040a61.MyNFT",
					"nft_id": "7",
					"source_address": "%s",
					"recipient_address": "%s",
					"_expandable": {"transaction": "transaction", "result": "result"}
				}
			]
		}`, txID.String(), ts, sourceAddr.Hex(), recipientAddr.Hex())

		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      900,
					TransactionIndex: 2,
					EventIndices:     []uint32{1},
					TokenType:        "A.1654653399040a61.MyNFT",
					ID:               3,
					SourceAddress:    sourceAddr,
					RecipientAddress: recipientAddr,
				},
			},
			NextCursor: nil,
		}

		expectedCursor := &accessmodel.TransferCursor{
			BlockHeight:      1000,
			TransactionIndex: 3,
			EventIndex:       5,
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			expectedCursor,
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		encodedCursor := testEncodeTransferCursor(t, 1000, 3, 5)
		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{cursor: encodedCursor})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with token_type filter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{},
		}

		expectedFilter := extended.AccountNFTTransferFilter{
			AccountAddress: address,
			TokenType:      "A.1654653399040a61.MyNFT",
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			expectedFilter,
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{tokenType: "A.1654653399040a61.MyNFT"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with role=sender filter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{},
		}

		expectedFilter := extended.AccountNFTTransferFilter{
			AccountAddress: address,
			TransferRole:   accessmodel.TransferRoleSender,
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			expectedFilter,
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{role: "sender"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with expand=transaction", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{},
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{Transaction: true},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{expand: "transaction"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with expand=result", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{},
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{Result: true},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{expand: "result"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid address", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountNFTTransfersURL(t, "invalid", nftTransfersURLParams{})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid address")
	})

	t.Run("invalid cursor format", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{cursor: "badcursor"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("invalid limit", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{limit: "abc"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid limit")
	})

	t.Run("invalid role", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{role: "invalidrole"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid role")
	})

	t.Run("invalid recipient_address", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{recipientAddr: "not-an-address"})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid recipient_address")
	})

	t.Run("backend returns not found", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.NotFound, "no transfers found for account %s", address))

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		reqURL := accountNFTTransfersURL(t, address.String(), nftTransfersURLParams{})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})

	t.Run("address with 0x prefix", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      100,
					TransactionIndex: 0,
					EventIndices:     []uint32{0},
					TokenType:        "A.1654653399040a61.MyNFT",
					ID:               1,
					SourceAddress:    sourceAddr,
					RecipientAddress: recipientAddr,
				},
			},
		}

		backend.On("GetAccountNonFungibleTokenTransfers",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.TransferCursor)(nil),
			extended.AccountNFTTransferFilter{AccountAddress: address},
			extended.AccountTransferExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		reqURL := accountNFTTransfersURL(t, "0x"+address.String(), nftTransfersURLParams{})
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
