package access

import "github.com/onflow/flow-go/model/flow"

// Contract represents a Cadence smart contract as returned by the extended API.
type Contract struct {
	Identifier string
	Body       string
}

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
}

// ContractDeploymentCursor is an opaque pagination token for contract deployment queries.
// If ContractID is non-empty, the cursor is used to paginate over unique contracts (All, ByAddress).
// If ContractID is empty, Height, TxIndex, and EventIndex identify the last returned deployment
// and are used to paginate over deployments of a single contract (GetContractDeployments).
type ContractDeploymentCursor struct {
	// ContractID is the unique contract identifier of the last returned contract.
	// Used when paginating over unique contracts.
	ContractID string
	// Height is the block height of the last returned deployment.
	// Used when paginating over deployments of a single contract.
	Height uint64
	// TxIndex is the transaction index of the last returned deployment within its block.
	TxIndex uint32
	// EventIndex is the event index of the last returned deployment within its transaction.
	EventIndex uint32
}

// ContractDeploymentPage is a page of ContractDeployment results with an optional continuation cursor.
type ContractDeploymentPage struct {
	// Deployments is the list of contract deployments for this page.
	Deployments []ContractDeployment
	// NextCursor is nil when no more results exist.
	NextCursor *ContractDeploymentCursor
}
