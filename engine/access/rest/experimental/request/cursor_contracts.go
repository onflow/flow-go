package request

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// contractDeploymentCursorUnique is the JSON shape for a unique-contracts cursor.
type contractDeploymentCursorUnique struct {
	ContractID string `json:"c"`
}

// contractDeploymentCursorDeployments is the JSON shape for a single-contract deployments cursor.
type contractDeploymentCursorDeployments struct {
	Height     uint64 `json:"h"`
	TxIndex    uint32 `json:"t"`
	EventIndex uint32 `json:"e"`
}

// EncodeContractDeploymentCursor encodes a ContractDeploymentCursor as a base64url JSON string.
// If ContractID is non-empty, encodes as {"c":"..."} (unique-contracts mode).
// Otherwise encodes as {"h":N,"t":N,"e":N} (deployments-of-one-contract mode).
//
// No error returns are expected during normal operation.
func EncodeContractDeploymentCursor(cursor *accessmodel.ContractDeploymentCursor) (string, error) {
	var (
		data []byte
		err  error
	)
	if cursor.ContractID != "" {
		data, err = json.Marshal(contractDeploymentCursorUnique{ContractID: cursor.ContractID})
	} else {
		data, err = json.Marshal(contractDeploymentCursorDeployments{
			Height:     cursor.Height,
			TxIndex:    cursor.TxIndex,
			EventIndex: cursor.EventIndex,
		})
	}
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeContractDeploymentCursor decodes a base64url JSON cursor string into a
// ContractDeploymentCursor. If the JSON contains a "c" key, the result is in
// unique-contracts mode (ContractID set). Otherwise it is in deployments mode
// (Height, TxIndex, EventIndex set).
//
// Expected error returns during normal operation:
//   - wrapped base64 or JSON decode error if the cursor is malformed.
func DecodeContractDeploymentCursor(raw string) (*accessmodel.ContractDeploymentCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	// Peek at the JSON keys to determine cursor mode.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	if _, ok := probe["c"]; ok {
		var c contractDeploymentCursorUnique
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("invalid cursor format: %w", err)
		}
		return &accessmodel.ContractDeploymentCursor{ContractID: c.ContractID}, nil
	}

	var c contractDeploymentCursorDeployments
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	return &accessmodel.ContractDeploymentCursor{
		Height:     c.Height,
		TxIndex:    c.TxIndex,
		EventIndex: c.EventIndex,
	}, nil
}
