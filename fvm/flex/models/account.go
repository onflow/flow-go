package models

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

// FlexAddress is an EVM-compatible address
type FlexAddress gethCommon.Address

const FlexAddressLength = gethCommon.AddressLength

func NewFlexAddress(addr gethCommon.Address) FlexAddress {
	fa := FlexAddress(addr)
	return fa
}

func (fa FlexAddress) Bytes() []byte {
	return fa[:]
}

func (fa FlexAddress) ToCommon() gethCommon.Address {
	return gethCommon.Address(fa)
}

func NewFlexAddressFromString(str string) FlexAddress {
	return FlexAddress(gethCommon.BytesToAddress([]byte(str)))
}

type GasLimit uint64

type Code []byte

type Data []byte

func (d Data) AsBigInt() *big.Int {
	return new(big.Int).SetBytes(d)
}

// Flex accounts are EVM accounts on Flex, currently
// three types of accounts are supported on Flex,
// externally owned accounts (EOAs), smart contract accounts and flow owned accounts (FOAs)
// FlowOwnedAccount is a new type of account in the Flex environment,
// that instead of being managed by public key inside the Flex,
// is managed by  a resource owned by a Flow account.
//
// In other words, the FVM account who owns the FOA resource
// can bridge native tokens to and from the Flex account associated with the FOA,
// deploy contracts to the Flex environment,
// or call methods on contracts without the need to sign a transaction.
type FlexAccount interface {
	// Returns the address of this account
	Address() FlexAddress

	// Returns balance of this account
	Balance() Balance

	// Deposit deposits the token from the given vault into this account
	Deposit(*FLOWTokenVault)

	// Withdraw withdraws the balance from account and
	// return it as a FlowTokenVault
	// works only for FOA accounts
	Withdraw(Balance) *FLOWTokenVault

	// Transfer is a utility method on top of call for transfering tokens to another account
	Transfer(to FlexAddress, balance Balance)

	// Deploy deploys a contract to the Flex environment
	// the new deployed contract would be at the returned address and
	// the contract data is not controlled by the FOA accounts
	// works only for FOA accounts
	Deploy(Code, GasLimit, Balance) FlexAddress

	// Call calls a smart contract function with the given data.
	// The gas usage is limited by the given gas limit,
	// and the Flow transaction's computation limit.
	// The fees are deducted from the FOA
	// and are transferred to the target address.
	// if no data is provided it would behave as transfering tokens to the
	// target address
	// works only for FOA accounts
	Call(FlexAddress, Data, GasLimit, Balance) Data
}
