package types

import (
	"math"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/tracers"
)

var (
	// DefaultBlockLevelGasLimit is the default value for the block gas limit
	// currently set to maximum and we don't consider any limit
	// given number of included EVM transactions are naturally
	// limited by the Flow block production limits.
	DefaultBlockLevelGasLimit = uint64(math.MaxUint64)
	// DefaultBaseFee is the default base fee value for the block
	// is set to zero but can be updated by the config
	DefaultBaseFee = big.NewInt(0)

	// DefaultDirectCallBaseGasUsage holds the minimum gas
	// charge for direct calls
	DefaultDirectCallBaseGasUsage = uint64(21_000)
	// DefaultDirectCallGasPrice captures the default
	// gas price for the direct call.
	// its set to zero currently given that we charge
	// computation but we don't need to refund to any
	// coinbase account.
	DefaultDirectCallGasPrice = uint64(0)

	// anything block number above 0 works here
	BlockNumberForEVMRules = big.NewInt(1)
)

// BlockContext holds the context needed for the emulator operations
type BlockContext struct {
	ChainID                *big.Int
	BlockNumber            uint64
	BlockTimestamp         uint64
	DirectCallBaseGasUsage uint64
	DirectCallGasPrice     uint64
	TxCountSoFar           uint
	TotalGasUsedSoFar      uint64
	GasFeeCollector        Address
	GetHashFunc            func(n uint64) gethCommon.Hash
	Random                 gethCommon.Hash
	Tracer                 *tracers.Tracer

	// a set of extra precompiled contracts to be injected
	ExtraPrecompiledContracts []PrecompiledContract
}

// NewDefaultBlockContext returns a new default block context
func NewDefaultBlockContext(BlockNumber uint64) BlockContext {
	return BlockContext{
		ChainID:                FlowEVMPreviewNetChainID,
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
	// CodeOf returns the code for this address
	CodeOf(address Address) (Code, error)
	// CodeHashOf returns the code hash for this address
	CodeHashOf(address Address) ([]byte, error)
}

// BlockView facilitates execution of a transaction or a direct evm call in the context of a block
// Any error returned by any of the methods (e.g. stateDB errors) if non-fatal stops the outer flow transaction
// if fatal stops the node.
// EVM validation errors and EVM execution errors are part of the returned result
// and should be handled separately.
type BlockView interface {
	// DirectCall executes a direct call
	DirectCall(call *DirectCall) (*Result, error)

	// RunTransaction executes an evm transaction
	RunTransaction(tx *gethTypes.Transaction) (*Result, error)

	// DryRunTransaction executes unsigned transaction but does not persist the state changes,
	// since transaction is not signed, from address is used as the signer.
	DryRunTransaction(tx *gethTypes.Transaction, from gethCommon.Address) (*Result, error)

	// BatchRunTransactions executes a batch of evm transactions producing
	// a slice of execution Result where each result corresponds to each
	// item in the txs slice.
	BatchRunTransactions(txs []*gethTypes.Transaction) ([]*Result, error)
}

// Emulator emulates an evm-compatible chain
type Emulator interface {
	// constructs a new block view
	NewReadOnlyBlockView() (ReadOnlyBlockView, error)

	// constructs a new block
	NewBlockView(ctx BlockContext) (BlockView, error)
}
