package experimental

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// AccountTransactionsResponse is the JSON response for the GetAccountTransactions endpoint.
type AccountTransactionsResponse struct {
	Transactions []AccountTransactionResponse `json:"transactions"`
	NextCursor   string                       `json:"next_cursor,omitempty"`
}

// AccountTransactionResponse is a single transaction entry in the response.
type AccountTransactionResponse struct {
	BlockHeight      string                          `json:"block_height"`
	BlockTimestamp   string                          `json:"timestamp"`
	TransactionID    string                          `json:"transaction_id"`
	TransactionIndex string                          `json:"transaction_index"`
	Roles            []string                        `json:"roles,omitempty"`
	Transaction      *commonmodels.Transaction       `json:"transaction,omitempty"`
	Result           *commonmodels.TransactionResult `json:"result,omitempty"`
}

type AccountTransactionFilter struct {
	Roles []string `json:"roles,omitempty"`
}

// GetAccountTransactions returns a paginated list of transactions for the given account address.
func GetAccountTransactions(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
	address, err := parser.ParseAddress(r.GetVar("address"), r.Chain)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	var limit uint32
	if raw := r.GetQueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return nil, common.NewBadRequestError(fmt.Errorf("invalid limit: %w", err))
		}
		limit = uint32(parsed)
	}

	var cursor *accessmodel.AccountTransactionCursor
	if raw := r.GetQueryParam("cursor"); raw != "" {
		c, err := parseCursor(raw)
		if err != nil {
			return nil, common.NewBadRequestError(err)
		}
		if c.BlockHeight != 0 || c.TransactionIndex != 0 {
			cursor = c
		}
	}

	var filter extended.AccountTransactionFilter
	if raw := r.GetQueryParam("roles"); raw != "" {
		roles := strings.Split(raw, ",")
		for _, role := range roles {
			parsed, err := accessmodel.ParseTransactionRole(strings.TrimSpace(role))
			if err != nil {
				return nil, common.NewBadRequestError(fmt.Errorf("invalid role: %w", err))
			}
			filter.Roles = append(filter.Roles, parsed)
		}
	}

	expandOptions := extended.AccountTransactionExpandOptions{
		Transaction: r.Expands("transaction"),
		Result:      r.Expands("result"),
	}

	page, err := backend.GetAccountTransactions(r.Context(), address, limit, cursor, filter, expandOptions, entities.EventEncodingVersion_JSON_CDC_V0)
	if err != nil {
		return nil, err
	}

	resp := AccountTransactionsResponse{
		Transactions: make([]AccountTransactionResponse, len(page.Transactions)),
	}
	for i, tx := range page.Transactions {
		roles := make([]string, len(tx.Roles))
		for j, role := range tx.Roles {
			roles[j] = role.String()
		}
		var transaction *commonmodels.Transaction
		if r.Expands("transaction") && tx.Transaction != nil {
			transaction = new(commonmodels.Transaction)
			transaction.Build(tx.Transaction, nil, link)
		}
		var result *commonmodels.TransactionResult
		if r.Expands("result") && tx.Result != nil {
			result = new(commonmodels.TransactionResult)
			result.Build(tx.Result, tx.TransactionID, link)
		}
		resp.Transactions[i] = AccountTransactionResponse{
			BlockHeight:      strconv.FormatUint(tx.BlockHeight, 10),
			BlockTimestamp:   time.UnixMilli(int64(tx.BlockTimestamp)).UTC().Format(time.RFC3339Nano),
			TransactionID:    tx.TransactionID.String(),
			TransactionIndex: strconv.FormatUint(uint64(tx.TransactionIndex), 10),
			Roles:            roles,
			Transaction:      transaction,
			Result:           result,
		}
	}
	if page.NextCursor != nil {
		resp.NextCursor = encodeCursor(page.NextCursor)
	}

	return resp, nil
}

// accountTransactionCursor is the JSON representation of a pagination cursor.
// Encoded as base64 in query params and responses to keep the format opaque to clients.
type accountTransactionCursor struct {
	BlockHeight      uint64 `json:"h"`
	TransactionIndex uint32 `json:"i"`
}

// parseCursor decodes a base64-encoded JSON cursor string.
func parseCursor(raw string) (*accessmodel.AccountTransactionCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	var c accountTransactionCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	return &accessmodel.AccountTransactionCursor{
		BlockHeight:      c.BlockHeight,
		TransactionIndex: c.TransactionIndex,
	}, nil
}

// encodeCursor encodes a cursor as base64-encoded JSON.
func encodeCursor(cursor *accessmodel.AccountTransactionCursor) string {
	c := accountTransactionCursor{
		BlockHeight:      cursor.BlockHeight,
		TransactionIndex: cursor.TransactionIndex,
	}
	data, _ := json.Marshal(c) // accountTransactionCursor marshaling cannot fail
	return base64.RawURLEncoding.EncodeToString(data)
}
