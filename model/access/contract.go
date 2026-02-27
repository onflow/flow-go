package access

import (
	"crypto/sha3"

	"github.com/onflow/flow-go/model/flow"
)

// ContractDeployment is a point-in-time snapshot of a contract on-chain.
// Each deployment (add or update) produces a distinct ContractDeployment record.
type ContractDeployment struct {
	// ContractID is the address-qualified canonical identifier, e.g. "A.1654653399040a61.EVM".
	ContractID string
	// Address is the account that owns the contract.
	Address flow.Address
	// BlockHeight is the block height at which this deployment was applied.
	BlockHeight uint64
	// TransactionID is the ID of the transaction that applied this deployment.
	TransactionID flow.Identifier
	// TxIndex is the position of the deploying transaction within its block.
	TxIndex uint32
	// EventIndex is the position of the contract event within its transaction.
	// Needed to uniquely identify a deployment when a single transaction deploys multiple contracts.
	EventIndex uint32
	// Code is the Cadence source code of the contract at the time of deployment.
	// May be nil if code could not be extracted from the transaction body.
	Code []byte
	// CodeHash is the SHA3-256 hash of the contract code, as reported in the protocol event.
	CodeHash []byte

	// IsPlaceholder is true if the deployment was created during bootstrapping based on the current
	// chain state, and not based on a protocol event.
	// When true, the BlockHeight, TransactionID, TxIndex, and EventIndex fields are undefined.
	IsPlaceholder bool

	// Expansion fields populated when expandResults is true. Never persisted.
	Transaction *flow.TransactionBody `msgpack:"-"` // Transaction body (nil unless expanded)
	Result      *TransactionResult    `msgpack:"-"` // Transaction result (nil unless expanded)
}

// CadenceCodeHash calculates the SHA3-256 hash of the provided code using the same algorithm as Cadence.
func CadenceCodeHash(code []byte) []byte {
	hash := sha3.Sum256(code)
	return hash[:]
}

// ContractDeploymentsCursor is the single pagination cursor type for all contract index iterators.
// For DeploymentsByContractID, all fields are meaningful. For All and ByAddress, only ContractID
// is used for resumption; Height/TxIndex/EventIndex are populated but ignored by callers.
type ContractDeploymentsCursor struct {
	// ContractID identifies the contract. For All/ByAddress it is the resume key; for
	// DeploymentsByContractID it is set by the storage layer and not encoded in the REST cursor.
	ContractID string
	// Height is the block height of the last returned deployment.
	Height uint64
	// TxIndex is the position of the deploying transaction within its block.
	TxIndex uint32
	// EventIndex is the position of the contract event within its transaction.
	EventIndex uint32
}

// ContractDeploymentPage is a page of deployment history for a single contract.
// Returned by GetContractDeployments.
type ContractDeploymentPage struct {
	// Deployments is the ordered list of deployments for this page.
	Deployments []ContractDeployment
	// NextCursor is nil when no more results exist.
	NextCursor *ContractDeploymentsCursor
}

// ContractsPage is a page of contracts at their latest deployment.
// Returned by GetContracts and GetContractsByAddress.
type ContractsPage struct {
	// Deployments is the ordered list of latest-deployment entries for this page.
	Deployments []ContractDeployment
	// NextCursor is nil when no more results exist.
	// Only the ContractID field is used for resumption; Height/TxIndex/EventIndex are ignored.
	NextCursor *ContractDeploymentsCursor
}
