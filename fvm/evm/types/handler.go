package types

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
)

// EVM is an account inside FVM with special access to the underlying infrastructure
// which allows to run a virtual EVM-based blockchain inside FVM.
//
// There are two ways to interact with this environment:
//
// First, passing a signed transaction (EOA account) to the `EVM.run` Cadence function
// creates a new block, updates the internal merkle tree, and emits a new root hash.
//
// The Second way is through a new form of account called cadence-owned-accounts (COAs),
// which is represented and controlled through a resource, owned by a Flow account.
// The owner of the COA resource can interact with the evm environment on behalf of the address stored on the resource.
//
// The evm environment shares the same native token as Flow, there are no new tokens minted.
// Other ERC-20 fungible tokens can be bridged between COA resources and Flow accounts.

// ContractHandler handles operations on the evm environment
type ContractHandler interface {
	// Deploys a cadence-owned-account
	DeployACOAAccount() Address

	// AccountByAddress returns an account by address
	// if isAuthorized is set, it allows for functionality like `call`, `deploy`
	// should only be set for bridged accounts only.
	AccountByAddress(address Address, isAuthorized bool) Account

	// LastExecutedBlock returns information about the last executed block
	LastExecutedBlock() *Block

	// Run runs a transaction in the evm environment,
	// collects the gas fees, and transfers the gas fees to the given coinbase account.
	Run(tx []byte, coinbase Address)

	// FlowTokenAddress returns the address where FLOW token is deployed
	FlowTokenAddress() common.Address
}

// Backend passes the FVM functionality needed inside the handler
type Backend interface {
	environment.ValueStore
	environment.Meter
	environment.EventEmitter
}

// AddressAllocator allocates addresses, used by the handler
type AddressAllocator interface {
	// AllocateAddress allocates an address to be used by a COA resource
	AllocateAddress() (Address, error)
}

// BlockStore stores the chain of blocks
type BlockStore interface {
	// LatestBlock returns the latest appended block
	LatestBlock() (*Block, error)

	// BlockHash returns the hash of the block at the given height
	BlockHash(height int) (gethCommon.Hash, error)

	// BlockProposal returns the block proposal
	BlockProposal() (*Block, error)

	// CommitBlockProposal commits the block proposal and update the chain of blocks
	CommitBlockProposal() error

	// ResetBlockProposal resets the block proposal
	ResetBlockProposal() error
}
