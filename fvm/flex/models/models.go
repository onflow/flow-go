package models

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
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
/// The owner of the FOA resource can interact with the Flex environment on behalf of the Flex address.
//
// The Flex environment shares the same native token as Flow, there are no new tokens minted.
// Other ERC-20 fungible tokens can be bridged between FOA resources and Flow accounts.

// FlexAddress is an EVM-compatible address
type FlexAddress gethCommon.Address

func NewFlexAddress(addr gethCommon.Address) *FlexAddress {
	fa := FlexAddress(addr)
	return &fa
}

func (fa FlexAddress) ToCommon() gethCommon.Address {
	return gethCommon.Address(fa)
}

func NewFlexAddressFromString(str string) FlexAddress {
	return FlexAddress(gethCommon.BytesToAddress([]byte(str)))
}

type GasLimit uint64

type Code []byte

type Data []byte // TODO add functionality to convert this into values

type FlexAccount interface {
	Address() FlexAddress
	Exists() bool
	Balance() Balance
}

// FlowOwnedAccount is a new type of account in the Flex environment,
// that instead of being managed by public key inside the Flex,
// is managed by  a resource owned by a Flow account.
//
// In other words, the FVM account who owns the FOA resource
// can bridge native tokens to and from the Flex account associated with the FOA,
// deploy contracts to the Flex environment,
// or call methods on contracts without the need to sign a transaction.
type FlowOwnedAccount interface {
	// Address returns the flex address associated with the FOA account
	Address() *FlexAddress

	// returns balance of this account
	Balance() Balance

	// Deposit deposits the token from the given vault into the Flex main vault
	// and update the FOA balance with the new amount
	// TODO: move to FlexAccount
	Deposit(*FLOWTokenVault)

	// Withdraw deducts the balance from the FOA account and
	// withdraw and return flow token from the Flex main vault.
	Withdraw(Balance) *FLOWTokenVault

	// Deploy deploys a contract to the Flex environment
	// the new deployed contract would be at the returned address and
	// the contract data is not controlled by the FOA accounts
	Deploy(Code, GasLimit, Balance) *FlexAddress

	// Call calls a smart contract function with the given data.
	// The gas usage is limited by the given gas limit,
	// and the Flow transaction's computation limit.
	// The fees are deducted from the FOA
	// and are transferred to the target address.
	// TODO: clarify
	// Contract data is not controlled by the FOA account
	Call(*FlexAddress, Data, GasLimit, Balance) Data
}

// FlexContractHandler handles operations on the Flex environment
type FlexContractHandler interface {
	// NewFlowOwnedAccount constructs a new FOA
	NewFlowOwnedAccount() FlowOwnedAccount

	// LastExecutedBlock returns information about the last executed block
	LastExecutedBlock() *FlexBlock

	// Run runs a transaction in the Flex environment,
	// collects the gas fees, and transfers the gas fees to the given coinbase account.
	// Returns true if the transaction was successfully executed
	Run(tx []byte, coinbase *FlexAddress) bool
}
