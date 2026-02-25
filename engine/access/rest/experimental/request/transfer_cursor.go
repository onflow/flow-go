package request

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// transferCursor is the JSON representation of a transfer pagination cursor.
// Encoded as base64 in query params and responses to keep the format opaque to clients.
type transferCursor struct {
	BlockHeight      uint64 `json:"h"`
	TransactionIndex uint32 `json:"i"`
	EventIndex       uint32 `json:"e"`
}

// ParseTransferCursor decodes a base64-encoded JSON transfer cursor string.
//
// All errors indicate the cursor is invalid.
func ParseTransferCursor(raw string) (*accessmodel.TransferCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	var c transferCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	return &accessmodel.TransferCursor{
		BlockHeight:      c.BlockHeight,
		TransactionIndex: c.TransactionIndex,
		EventIndex:       c.EventIndex,
	}, nil
}

// EncodeTransferCursor encodes a transfer cursor as base64-encoded JSON.
//
// All errors indicate the cursor is invalid.
func EncodeTransferCursor(cursor *accessmodel.TransferCursor) (string, error) {
	c := transferCursor{
		BlockHeight:      cursor.BlockHeight,
		TransactionIndex: cursor.TransactionIndex,
		EventIndex:       cursor.EventIndex,
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transfer cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// ParseTransferRole parses a role query parameter value into a TransferRole.
func ParseTransferRole(raw string) (accessmodel.TransferRole, error) {
	role := accessmodel.TransferRole(raw)
	switch role {
	case accessmodel.TransferRoleSender, accessmodel.TransferRoleRecipient:
		return role, nil
	default:
		return "", fmt.Errorf("invalid role %q: must be %q or %q", raw, accessmodel.TransferRoleSender, accessmodel.TransferRoleRecipient)
	}
}
