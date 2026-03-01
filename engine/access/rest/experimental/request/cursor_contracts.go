package request

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// contractDeploymentsCursorJSON is the JSON shape for a [accessmodel.ContractDeploymentsCursor].
// Address and Name are omitted when empty (DeploymentsByContract cursors; the contract is
// identified by the URL parameter).
type contractDeploymentsCursorJSON struct {
	Address    string `json:"a,omitempty"`
	Name       string `json:"n,omitempty"`
	Height     uint64 `json:"h"`
	TxIndex    uint32 `json:"t"`
	EventIndex uint32 `json:"e"`
}

// EncodeContractDeploymentsCursor encodes a [accessmodel.ContractDeploymentsCursor] as a
// base64url JSON string. Address and Name are included when non-empty (All/ByAddress cursors).
//
// No error returns are expected during normal operation.
func EncodeContractDeploymentsCursor(cursor *accessmodel.ContractDeploymentsCursor) (string, error) {
	data, err := json.Marshal(contractDeploymentsCursorJSON{
		Address:    cursor.Address.Hex(),
		Name:       cursor.ContractName,
		Height:     cursor.BlockHeight,
		TxIndex:    cursor.TransactionIndex,
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
	cursor := &accessmodel.ContractDeploymentsCursor{
		ContractName:     c.Name,
		BlockHeight:      c.Height,
		TransactionIndex: c.TxIndex,
		EventIndex:       c.EventIndex,
	}
	if c.Address != "" {
		addr, err := flow.StringToAddress(c.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid address in cursor: %w", err)
		}
		cursor.Address = addr
	}
	return cursor, nil
}
