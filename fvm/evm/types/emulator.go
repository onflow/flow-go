package types

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
)

var (
	DefaultDirectCallBaseGasUsage = uint64(21_000)
	DefaultDirectCallGasPrice     = uint64(0)

	// anything block number above 0 works here
	BlockNumberForEVMRules = big.NewInt(1)
)

type Precompile interface {
	gethVM.PrecompiledContract
	Address() Address
}

// BlockContext holds the context needed for the emulator operations
type BlockContext struct {
	BlockNumber            uint64
	DirectCallBaseGasUsage uint64
	DirectCallGasPrice     uint64
	GasFeeCollector        Address
	GetHashFunc            func(n uint64) gethCommon.Hash
	Random                 gethCommon.Hash

	// a set of extra precompiles to be injected
	ExtraPrecompiles []Precompile
}

// NewDefaultBlockContext returns a new default block context
func NewDefaultBlockContext(BlockNumber uint64) BlockContext {
	return BlockContext{
		BlockNumber:            BlockNumber,
		DirectCallBaseGasUsage: DefaultDirectCallBaseGasUsage,
		DirectCallGasPrice:     DefaultDirectCallGasPrice,
		GetHashFunc: func(n uint64) gethCommon.Hash { // default returns some random hash values
			return gethCommon.BytesToHash(gethCrypto.Keccak256([]byte(new(big.Int).SetUint64(n).String())))
		},
	}
}

// ReadOnlyBlockView provides a read only view of a block
type ReadOnlyBlockView interface {
	// BalanceOf returns the balance of this address
	BalanceOf(address Address) (*big.Int, error)
	// NonceOf returns the nonce of this address
	NonceOf(address Address) (uint64, error)
	// CodeOf returns the code for this address (if smart contract is deployed at this address)
	CodeOf(address Address) (Code, error)
}

// BlockView facilitates execution of a transaction or a direct evm  call in the context of a block
// Errors returned by the methods are one of the followings:
// - Fatal error
// - Database error (non-fatal)
// - EVM validation error
// - EVM execution error
type BlockView interface {
	// executes a direct call
	DirectCall(call *DirectCall) (*Result, error)

	// RunTransaction executes an evm transaction
	RunTransaction(tx *gethTypes.Transaction) (*Result, error)
}

// Emulator emulates an evm-compatible chain
type Emulator interface {
	// constructs a new block view
	NewReadOnlyBlockView(ctx BlockContext) (ReadOnlyBlockView, error)

	// constructs a new block
	NewBlockView(ctx BlockContext) (BlockView, error)
}
