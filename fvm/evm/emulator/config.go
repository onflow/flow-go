package emulator

import (
	"math/big"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/onflow/go-ethereum/eth/tracers"
	gethParams "github.com/onflow/go-ethereum/params"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	zero    = uint64(0)
	bigZero = big.NewInt(0)
)

// Config aggregates all the configuration (chain, evm, block, tx, ...)
// needed during executing a transaction.
type Config struct {
	// Chain Config
	ChainConfig *gethParams.ChainConfig
	// EVM config
	EVMConfig gethVM.Config
	// block context
	BlockContext *gethVM.BlockContext
	// transaction context
	TxContext *gethVM.TxContext
	// base unit of gas for direct calls
	DirectCallBaseGasUsage uint64
	// captures extra precompiled calls
	PCTracker *CallTracker
	// BlockTxCount captures the total number of
	// transactions included in this block so far
	BlockTxCountSoFar uint
	// BlockTotalGasSoFar captures the total
	// amount of gas used so far
	BlockTotalGasUsedSoFar uint64
}

// ChainRules returns the chain rules
func (c *Config) ChainRules() gethParams.Rules {
	return c.ChainConfig.Rules(
		c.BlockContext.BlockNumber,
		c.BlockContext.Random != nil,
		c.BlockContext.Time)
}

// PreviewNetChainConfig is the chain config used by the previewnet
var PreviewNetChainConfig = MakeChainConfig(types.FlowEVMPreviewNetChainID)

// PreviewNetChainConfig is the chain config used by the testnet
var TestNetChainConfig = MakeChainConfig(types.FlowEVMTestNetChainID)

// MainNetChainConfig is the chain config used by the mainnet
var MainNetChainConfig = MakeChainConfig(types.FlowEVMMainNetChainID)

// DefaultChainConfig is the chain config used by the emulator
var DefaultChainConfig = PreviewNetChainConfig

// MakeChainConfig constructs a chain config
// it considers majority of EVM upgrades (e.g. Cancun update) are already applied
// This has been done through setting the height of these changes
// to zero nad setting the time for some other changes to zero
// For the future changes of EVM, we need to update the EVM go mod version
// and set a proper height for the specific release based on the Flow EVM heights
// so it could gets activated at a desired time.
func MakeChainConfig(chainID *big.Int) *gethParams.ChainConfig {
	return &gethParams.ChainConfig{
		ChainID: chainID,

		// Fork scheduling based on block heights
		HomesteadBlock:      bigZero,
		DAOForkBlock:        bigZero,
		DAOForkSupport:      false,
		EIP150Block:         bigZero,
		EIP155Block:         bigZero,
		EIP158Block:         bigZero,
		ByzantiumBlock:      bigZero, // already on Byzantium
		ConstantinopleBlock: bigZero, // already on Constantinople
		PetersburgBlock:     bigZero, // already on Petersburg
		IstanbulBlock:       bigZero, // already on Istanbul
		BerlinBlock:         bigZero, // already on Berlin
		LondonBlock:         bigZero, // already on London
		MuirGlacierBlock:    bigZero, // already on MuirGlacier

		// Fork scheduling based on timestamps
		ShanghaiTime: &zero, // already on Shanghai
		CancunTime:   &zero, // already on Cancun
		PragueTime:   nil,   // not on Prague
		VerkleTime:   nil,   // not on Verkle
	}
}

// Default config supports the dynamic fee structure (EIP-1559)
// so it accepts both legacy transactions with a fixed gas price
// and dynamic transactions with tip and cap.
// Yet default config keeps the base fee to zero (no automatic adjustment)
func defaultConfig() *Config {
	return &Config{
		ChainConfig: DefaultChainConfig,
		EVMConfig: gethVM.Config{
			// Forces the EIP-1559 baseFee to 0 (needed for 0 price calls)
			NoBaseFee: true,
		},
		TxContext: &gethVM.TxContext{
			GasPrice:   new(big.Int),
			BlobFeeCap: new(big.Int),
		},
		BlockContext: &gethVM.BlockContext{
			CanTransfer: gethCore.CanTransfer,
			Transfer:    gethCore.Transfer,
			GasLimit:    types.DefaultBlockLevelGasLimit,
			BaseFee:     types.DefaultBaseFee,
			GetHash: func(n uint64) gethCommon.Hash {
				return gethCommon.Hash{}
			},
			GetPrecompile: gethCore.GetPrecompile,
		},
		PCTracker: NewCallTracker(),
	}
}

// NewConfig initializes a new config
func NewConfig(opts ...Option) *Config {
	ctx := defaultConfig()
	for _, applyOption := range opts {
		ctx = applyOption(ctx)
	}
	return ctx
}

type Option func(*Config) *Config

// WithChainID sets the evm chain ID
func WithChainID(chainID *big.Int) Option {
	return func(c *Config) *Config {
		switch chainID.Uint64() {
		case types.FlowEVMPreviewNetChainIDInUInt64:
			c.ChainConfig = PreviewNetChainConfig
		case types.FlowEVMTestNetChainIDInUInt64:
			c.ChainConfig = TestNetChainConfig
		case types.FlowEVMMainNetChainIDInUInt64:
			c.ChainConfig = MainNetChainConfig
		}
		return c
	}
}

// WithOrigin sets the origin of the transaction (signer)
func WithOrigin(origin gethCommon.Address) Option {
	return func(c *Config) *Config {
		c.TxContext.Origin = origin
		return c
	}
}

// WithGasPrice sets the gas price for the transaction (usually the one sets by the sender)
func WithGasPrice(gasPrice *big.Int) Option {
	return func(c *Config) *Config {
		c.TxContext.GasPrice = gasPrice
		return c
	}
}

// WithGasLimit sets the gas limit of the transaction
func WithGasLimit(gasLimit uint64) Option {
	return func(c *Config) *Config {
		c.BlockContext.GasLimit = gasLimit
		return c
	}
}

// WithCoinbase sets the coinbase of the block where the fees are collected in
func WithCoinbase(coinbase gethCommon.Address) Option {
	return func(c *Config) *Config {
		c.BlockContext.Coinbase = coinbase
		return c
	}
}

// WithBlockNumber sets the block height in the block context
func WithBlockNumber(blockNumber *big.Int) Option {
	return func(c *Config) *Config {
		c.BlockContext.BlockNumber = blockNumber
		return c
	}
}

// WithBlockTime sets the block time in the block context
func WithBlockTime(time uint64) Option {
	return func(c *Config) *Config {
		c.BlockContext.Time = time
		return c
	}
}

// WithGetBlockHashFunction sets the functionality to look up block hash by height
func WithGetBlockHashFunction(getHash gethVM.GetHashFunc) Option {
	return func(c *Config) *Config {
		c.BlockContext.GetHash = getHash
		return c
	}
}

// WithDirectCallBaseGasUsage sets the base direct call gas usage
func WithDirectCallBaseGasUsage(gas uint64) Option {
	return func(c *Config) *Config {
		c.DirectCallBaseGasUsage = gas
		return c
	}
}

// WithExtraPrecompiledContracts appends the precompiled contract list with extra precompiled contracts
func WithExtraPrecompiledContracts(precompiledContracts []types.PrecompiledContract) Option {
	return func(c *Config) *Config {
		extraPreCompMap := make(map[gethCommon.Address]gethVM.PrecompiledContract)
		for _, pc := range precompiledContracts {
			// wrap pcs for tracking
			wpc := c.PCTracker.RegisterPrecompiledContract(pc)
			extraPreCompMap[pc.Address().ToCommon()] = wpc
		}
		c.BlockContext.GetPrecompile = func(rules gethParams.Rules, addr gethCommon.Address) (gethVM.PrecompiledContract, bool) {
			prec, found := extraPreCompMap[addr]
			if found {
				return prec, true
			}
			return gethCore.GetPrecompile(rules, addr)
		}
		return c
	}
}

// WithRandom sets the block context random field
func WithRandom(rand *gethCommon.Hash) Option {
	return func(c *Config) *Config {
		c.BlockContext.Random = rand
		return c
	}
}

// WithTransactionTracer sets a transaction tracer
func WithTransactionTracer(tracer *tracers.Tracer) Option {
	return func(c *Config) *Config {
		if tracer != nil {
			c.EVMConfig.Tracer = tracer.Hooks
		}
		return c
	}
}

// WithBlockTxCountSoFar sets the total number of transactions
// included in the current block so far
func WithBlockTxCountSoFar(txCount uint) Option {
	return func(c *Config) *Config {
		c.BlockTxCountSoFar = txCount
		return c
	}
}

// WithBlockTotalGasSoFar sets the total amount of gas used
// for this block so far
func WithBlockTotalGasUsedSoFar(gasUsed uint64) Option {
	return func(c *Config) *Config {
		c.BlockTotalGasUsedSoFar = gasUsed
		return c
	}
}
