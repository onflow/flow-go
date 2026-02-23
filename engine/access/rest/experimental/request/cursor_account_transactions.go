package request

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// accountTransactionCursor is the JSON representation of an account transaction pagination cursor.
// Encoded as base64 in query params and responses to keep the format opaque to clients.
type accountTransactionCursor struct {
	BlockHeight      uint64 `json:"h"`
	TransactionIndex uint32 `json:"t"`
}

// parseAccountTransactionCursor decodes a base64-encoded JSON cursor string.
func parseAccountTransactionCursor(raw string) (*accessmodel.AccountTransactionCursor, error) {
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

// EncodeAccountTransactionCursor encodes a cursor as base64-encoded JSON.
func EncodeAccountTransactionCursor(cursor *accessmodel.AccountTransactionCursor) (string, error) {
	c := accountTransactionCursor{
		BlockHeight:      cursor.BlockHeight,
		TransactionIndex: cursor.TransactionIndex,
	}
	data, err := json.Marshal(c) // accountTransactionCursor marshaling cannot fail
	if err != nil {
		return "", fmt.Errorf("failed to marshal account transaction cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
