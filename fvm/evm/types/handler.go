package types

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/cadence/common"
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
	// DeployCOA deploys a Cadence owned account and return the address
	DeployCOA(uuid uint64) Address

	// AccountByAddress returns an account by address
	// if isAuthorized is set, it allows for functionality like `call`, `deploy`
	// should only be set for the cadence owned accounts only.
	AccountByAddress(address Address, isAuthorized bool) Account

	// LastExecutedBlock returns information about the last executed block
	LastExecutedBlock() *Block

	// Run runs a transaction in the evm environment,
	// collects the gas fees, and transfers it to the gasFeeCollector account
	Run(tx []byte, gasFeeCollector Address) *ResultSummary

	// DryRun simulates execution of the provided RLP-encoded and unsigned transaction.
	// Because the transaction is unsigned the from address is required, since
	// from address is normally derived from the transaction signature.
	// The function should not have any persisted changes made to the state.
	DryRun(tx []byte, from Address) *ResultSummary

	// DryRunWithTxData simulates execution of the provided transaction data.
	// The from address is required since the transaction is unsigned.
	// The function should not have any persisted changes made to the state.
	DryRunWithTxData(txData types.TxData, from Address) *ResultSummary

	// BatchRun runs transaction batch in the evm environment,
	// collect all the gas fees and transfers the gas fees to the gasFeeCollector account.
	BatchRun(txs [][]byte, gasFeeCollector Address) []*ResultSummary

	// FlowTokenAddress returns the address where FLOW token is deployed
	FlowTokenAddress() common.Address

	// EVMContractAddress returns the address where EVM is deployed
	EVMContractAddress() common.Address

	// GenerateResourceUUID generates a new UUID for a resource
	GenerateResourceUUID() uint64

	// Constructs and commits a new block from the block proposal
	CommitBlockProposal()
}

// AddressAllocator allocates addresses, used by the handler
type AddressAllocator interface {
	// AllocateAddress allocates an address to be used by a COA resource
	AllocateCOAAddress(uuid uint64) Address

	// COAFactoryAddress returns the address for the COA factory
	COAFactoryAddress() Address

	// NativeTokenBridgeAddress returns the address for the native token bridge
	// used for deposit and withdraw calls
	NativeTokenBridgeAddress() Address

	// AllocateAddress allocates an address by index to be used by a precompile contract
	AllocatePrecompileAddress(index uint64) Address
}

// BlockStore stores the chain of blocks
type BlockStore interface {
	// LatestBlock returns the latest appended block
	LatestBlock() (*Block, error)

	// BlockHash returns the hash of the block at the given height
	BlockHash(height uint64) (gethCommon.Hash, error)

	// BlockProposal returns the active block proposal
	BlockProposal() (*BlockProposal, error)

	// UpdateBlockProposal replaces the current block proposal with the ones passed
	UpdateBlockProposal(*BlockProposal) error

	// CommitBlockProposal commits the block proposal and update the chain of blocks
	CommitBlockProposal(*BlockProposal) error
}
