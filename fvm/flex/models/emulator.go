package models

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

type Emulator interface {
	// returns the amount of gas needed for a token transfer
	TransferGasUsage() uint64
	// BalanceOf returns the balance of this address
	BalanceOf(address FlexAddress) (*big.Int, error)
	// CodeOf returns the code for this address (if smart contract is deployed at this address)
	CodeOf(address FlexAddress) (Code, error)
	// MintTo mints new tokens to this address
	MintTo(address FlexAddress, amount *big.Int) (*Result, error)
	// WithdrawFrom withdraws tokens from this address
	WithdrawFrom(address FlexAddress, amount *big.Int) (*Result, error)
	// Transfer transfers token between addresses
	Transfer(from FlexAddress, to FlexAddress, value *big.Int) (*Result, error)
	// Deploy deploys an smart contract
	Deploy(caller FlexAddress, code Code, gasLimit uint64, value *big.Int) (*Result, error)
	// Call makes a call to a smart contract
	Call(caller FlexAddress, to FlexAddress, data Data, gasLimit uint64, value *big.Int) (*Result, error)
	// RunTransaction runs a transaction and collect gas fees to the coinbase account
	RunTransaction(tx *types.Transaction, coinbase FlexAddress) (*Result, error)
}
