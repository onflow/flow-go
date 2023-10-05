package models

import (
	fvmenv "github.com/onflow/flow-go/fvm/environment"
)

// Flex is an account inside FVM with special access to the underlying infrastructure
// which allows to run a virtual EVM-based blockchain inside FVM.
//
// There are two ways to interact with this environment:
//
// First, passing a signed transaction (EOA account) to the `Flex.run` Cadence function
// creates a new block, updates the internal merkle tree, and emits a new root hash.
//
// The Second way is through a new form of account called Flow-owned account (FOA),
// which is represented and controlled through a resource, owned by a Flow account.
// The FOA has a collision-proof allocated address.
// The owner of the FOA resource can interact with the Flex environment on behalf of the Flex address.
//
// The Flex environment shares the same native token as Flow, there are no new tokens minted.
// Other ERC-20 fungible tokens can be bridged between FOA resources and Flow accounts.

// FlexContractHandler handles operations on the Flex environment
type FlexContractHandler interface {
	// AllocateAddress allocates an address to be used by a foa resource
	AllocateAddress() FlexAddress

	// AccountByAddress returns the FlexAccount by address
	// if isFOA is set, it allows for functionality like `call`, `deploy`
	AccountByAddress(addr FlexAddress, isFOA bool) FlexAccount

	// LastExecutedBlock returns information about the last executed block
	LastExecutedBlock() *FlexBlock

	// Run runs a transaction in the Flex environment,
	// collects the gas fees, and transfers the gas fees to the given coinbase account.
	// Returns true if the transaction was successfully executed
	Run(tx []byte, coinbase FlexAddress) bool
}

// Backend passes the FVM functionality needed inside the handler
type Backend interface {
	fvmenv.ValueStore
	fvmenv.Meter
	fvmenv.EventEmitter
}
