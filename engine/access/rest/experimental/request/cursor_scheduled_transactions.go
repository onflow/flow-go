package request

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// scheduledTxCursor is the JSON shape of a pagination cursor (opaque to clients).
type scheduledTxCursor struct {
	ID uint64 `json:"i"`
}

// parseScheduledTxCursor decodes a base64-encoded JSON cursor string.
func parseScheduledTxCursor(raw string) (*accessmodel.ScheduledTransactionCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c scheduledTxCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	return &accessmodel.ScheduledTransactionCursor{ID: c.ID}, nil
}

// EncodeScheduledTxCursor encodes a cursor as base64 URL-encoded JSON.
//
// All errors indicate the cursor is invalid.
func EncodeScheduledTxCursor(cursor *accessmodel.ScheduledTransactionCursor) (string, error) {
	data, err := json.Marshal(scheduledTxCursor{ID: cursor.ID})
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
