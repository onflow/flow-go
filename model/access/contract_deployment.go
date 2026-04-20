package access

import (
	"crypto/sha3"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

const (
	// MinContractIDLength is the minimum length of a valid contract ID.
	// The format is "A.{address_hex}.{name}".
	MinContractIDLength = 4 + 2*flow.AddressLength
)

// ContractDeployment is a point-in-time snapshot of a contract on-chain.
// Each deployment (add or update) produces a distinct ContractDeployment record.
type ContractDeployment struct {
	// Address is the account that owns the contract.
	Address flow.Address
	// ContractName is the unqualified contract name, derived from ContractID.
	ContractName string
	// BlockHeight is the block height at which this deployment was applied.
	BlockHeight uint64
	// TransactionID is the ID of the transaction that applied this deployment.
	TransactionID flow.Identifier
	// TransactionIndex is the position of the deploying transaction within its block.
	TransactionIndex uint32
	// EventIndex is the position of the contract event within its transaction.
	// Needed to uniquely identify a deployment when a single transaction deploys multiple contracts.
	EventIndex uint32
	// Code is the Cadence source code of the contract at the time of deployment.
	// May be nil if code could not be extracted from the transaction body.
	Code []byte
	// CodeHash is the SHA3-256 hash of the contract code, as reported in the protocol event.
	CodeHash []byte

	// IsDeleted is true if the contract was deleted.
	// Deleting contracts is not currently supported by the protocol, however there are contracts on
	// mainnet that are deleted.
	IsDeleted bool

	// IsPlaceholder is true if the deployment was created during bootstrapping based on the current
	// chain state, and not based on a protocol event.
	// When true, the BlockHeight, TransactionID, TxIndex, and EventIndex fields are undefined.
	IsPlaceholder bool

	// Expansion fields populated when expandResults is true. Never persisted.
	Transaction *flow.TransactionBody `msgpack:"-"` // Transaction body (nil unless expanded)
	Result      *TransactionResult    `msgpack:"-"` // Transaction result (nil unless expanded)
}

// ContractID constructs the canonical contract ID string from an address and contract name.
// Format: "A.{address_hex}.{name}"
func ContractID(address flow.Address, name string) string {
	return "A." + address.Hex() + "." + name
}

// ParseContractID extracts the address and contract name from a contract identifier of the form
// "A.{address_hex}.{name}".
//
// Any error indicates the contractID is not in the expected format.
func ParseContractID(id string) (flow.Address, string, error) {
	if len(id) < MinContractIDLength || id[:2] != "A." {
		return flow.Address{}, "", fmt.Errorf("unexpected contract ID format: %q", id)
	}
	addr, name, ok := strings.Cut(id[2:], ".") // strip "A." then split on "."
	if !ok {
		return flow.Address{}, "", fmt.Errorf("unexpected contract ID format (no second dot): %q", id)
	}
	address, err := flow.StringToAddress(addr)
	if err != nil {
		return flow.Address{}, "", fmt.Errorf("invalid address in contract ID %q: %w", id, err)
	}
	if strings.Contains(name, ".") {
		return flow.Address{}, "", fmt.Errorf("contract name is invalid: %q", name)
	}
	return address, name, nil
}

// CadenceCodeHash calculates the SHA3-256 hash of the provided code using the same algorithm as Cadence.
func CadenceCodeHash(code []byte) []byte {
	hash := sha3.Sum256(code)
	return hash[:]
}

// ContractDeploymentsCursor is the single pagination cursor type for all contract index iterators.
type ContractDeploymentsCursor struct {
	// Address is the account that owns the contract.
	Address flow.Address
	// ContractName is the unqualified contract name.
	ContractName string
	// BlockHeight is the block height of the last returned deployment.
	BlockHeight uint64
	// TransactionIndex is the position of the deploying transaction within its block.
	TransactionIndex uint32
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
