package request

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// contractDeploymentsCursorJSON is the JSON shape for a [accessmodel.ContractDeploymentsCursor].
// ContractID is omitted when empty (DeploymentsByContractID cursors; the ID comes from the URL).
type contractDeploymentsCursorJSON struct {
	ContractID string `json:"c,omitempty"`
	Height     uint64 `json:"h"`
	TxIndex    uint32 `json:"t"`
	EventIndex uint32 `json:"e"`
}

// EncodeContractDeploymentsCursor encodes a [accessmodel.ContractDeploymentsCursor] as a
// base64url JSON string. ContractID is included when non-empty (All/ByAddress cursors).
//
// No error returns are expected during normal operation.
func EncodeContractDeploymentsCursor(cursor *accessmodel.ContractDeploymentsCursor) (string, error) {
	data, err := json.Marshal(contractDeploymentsCursorJSON{
		ContractID: cursor.ContractID,
		Height:     cursor.Height,
		TxIndex:    cursor.TxIndex,
		EventIndex: cursor.EventIndex,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal deployments cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeContractDeploymentsCursor decodes a base64url JSON string into a
// [accessmodel.ContractDeploymentsCursor].
//
// Expected error returns during normal operation:
//   - wrapped base64 or JSON decode error if the cursor is malformed.
func DecodeContractDeploymentsCursor(raw string) (*accessmodel.ContractDeploymentsCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var c contractDeploymentsCursorJSON
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid deployments cursor format: %w", err)
	}
	return &accessmodel.ContractDeploymentsCursor{
		ContractID: c.ContractID,
		Height:     c.Height,
		TxIndex:    c.TxIndex,
		EventIndex: c.EventIndex,
	}, nil
}
