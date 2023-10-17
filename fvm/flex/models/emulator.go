package models

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
	DefaultDirectCallBaseGasUsage = uint64(21_000)
	DefaultDirectCallGasPrice     = uint64(0)

	// anything block number above 0 works here
	BlockNumberForEVMRules = big.NewInt(1)
)

type BlockContext struct {
	BlockNumber            uint64
	DirectCallBaseGasUsage uint64
	DirectCallGasPrice     uint64
	GasFeeCollector        FlexAddress
}

func NewDefaultBlockContext(BlockNumber uint64) BlockContext {
	return BlockContext{
		BlockNumber:            BlockNumber,
		DirectCallBaseGasUsage: DefaultDirectCallBaseGasUsage,
		DirectCallGasPrice:     DefaultDirectCallGasPrice,
	}
}

// BlockView provides a read only view of a block
type BlockView interface {
	// BalanceOf returns the balance of this address
	BalanceOf(address FlexAddress) (*big.Int, error)
	// CodeOf returns the code for this address (if smart contract is deployed at this address)
	CodeOf(address FlexAddress) (Code, error)
}

// Block allows evm calls in the context of a block
type Block interface {
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
	// RunTransaction runs a transaction
	RunTransaction(tx *types.Transaction) (*Result, error)
}

type Emulator interface {
	// constructs a new block view
	NewBlockView(ctx BlockContext) (BlockView, error)

	// constructs a new block
	NewBlock(ctx BlockContext) (Block, error)
}
