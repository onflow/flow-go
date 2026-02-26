package models

import (
	"encoding/hex"

	accessmodel "github.com/onflow/flow-go/model/access"
)

// ContractDeployment is the REST representation of a single contract deployment.
type ContractDeployment struct {
	// Identifier is the canonical contract identifier, e.g. "A.1654653399040a61.EVM".
	Identifier string `json:"identifier"`
	// Address is the hex-encoded account address that owns the contract.
	Address string `json:"address"`
	// BlockHeight is the height of the block in which this deployment was applied.
	BlockHeight uint64 `json:"block_height"`
	// TransactionId is the hex-encoded transaction ID that applied this deployment.
	TransactionId string `json:"transaction_id"`
	// TransactionIndex is the position of the deploying transaction within its block.
	TransactionIndex uint32 `json:"transaction_index"`
	// EventIndex is the position of the contract event within its transaction.
	EventIndex uint32 `json:"event_index"`
	// CodeHash is the hex-encoded SHA3-256 hash of the contract code.
	CodeHash string `json:"code_hash"`
	// Code is the Cadence source code (omitted if not available).
	Code string `json:"code,omitempty"`
}

// Build populates the REST model from the domain model.
func (m *ContractDeployment) Build(d *accessmodel.ContractDeployment) {
	m.Identifier = d.ContractID
	m.Address = d.Address.Hex()
	m.BlockHeight = d.BlockHeight
	m.TransactionId = d.TransactionID.String()
	m.TransactionIndex = d.TxIndex
	m.EventIndex = d.EventIndex
	m.CodeHash = hex.EncodeToString(d.CodeHash)
	if len(d.Code) > 0 {
		m.Code = string(d.Code)
	}
}

// ContractDeploymentsResponse is the paginated list response for contract deployment endpoints.
type ContractDeploymentsResponse struct {
	// Contracts is the list of contract deployments for this page.
	Contracts []ContractDeployment `json:"contracts"`
	// NextCursor is the opaque pagination token for the next page; empty if no more results.
	NextCursor string `json:"next_cursor,omitempty"`
}
