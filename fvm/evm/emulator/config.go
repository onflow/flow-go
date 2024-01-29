package emulator

import (
	"math"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	gethParams "github.com/ethereum/go-ethereum/params"

	"github.com/onflow/flow-go/fvm/evm/types"
)

var (
	FlowEVMTestnetChainID = big.NewInt(666)
	FlowEVMMainnetChainID = big.NewInt(777)
	BlockLevelGasLimit    = uint64(math.MaxUint64)
	zero                  = uint64(0)
)

// Config sets the required parameters
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
	// a set of extra precompiles to be injected
	ExtraPrecompiles map[gethCommon.Address]gethVM.PrecompiledContract
}

func (c *Config) ChainRules() gethParams.Rules {
	return c.ChainConfig.Rules(
		c.BlockContext.BlockNumber,
		c.BlockContext.Random != nil,
		c.BlockContext.Time)
}

// DefaultChainConfig is the default chain config which
// considers majority of EVM upgrades (e.g. Shanghai update) already been applied
// this has done through setting the height of these changes
// to zero nad setting the time for some other changes to zero
// For the future changes of EVM, we need to update the EVM go mod version
// and set a proper height for the specific release based on the Flow EVM heights
// so it could gets activated at a desired time.
var DefaultChainConfig = &gethParams.ChainConfig{
	ChainID: FlowEVMTestnetChainID, // default is testnet

	// Fork scheduling based on block heights
	HomesteadBlock:      big.NewInt(0),
	DAOForkBlock:        big.NewInt(0),
	DAOForkSupport:      false,
	EIP150Block:         big.NewInt(0),
	EIP155Block:         big.NewInt(0),
	EIP158Block:         big.NewInt(0),
	ByzantiumBlock:      big.NewInt(0), // already on Byzantium
	ConstantinopleBlock: big.NewInt(0), // already on Constantinople
	PetersburgBlock:     big.NewInt(0), // already on Petersburg
	IstanbulBlock:       big.NewInt(0), // already on Istanbul
	BerlinBlock:         big.NewInt(0), // already on Berlin
	LondonBlock:         big.NewInt(0), // already on London
	MuirGlacierBlock:    big.NewInt(0), // already on MuirGlacier

	// Fork scheduling based on timestamps
	ShanghaiTime: &zero, // already on Shanghai
	CancunTime:   &zero, // already on Cancun
	PragueTime:   &zero, // already on Prague
}

func defaultConfig() *Config {
	return &Config{
		ChainConfig: DefaultChainConfig,
		EVMConfig: gethVM.Config{
			NoBaseFee: true,
		},
		TxContext: &gethVM.TxContext{
			GasPrice:   new(big.Int),
			BlobFeeCap: new(big.Int),
		},
		BlockContext: &gethVM.BlockContext{
			CanTransfer: gethCore.CanTransfer,
			Transfer:    gethCore.Transfer,
			GasLimit:    BlockLevelGasLimit, // block gas limit
			BaseFee:     big.NewInt(0),
			GetHash: func(n uint64) gethCommon.Hash { // default returns some random hash values
				return gethCommon.BytesToHash(gethCrypto.Keccak256([]byte(new(big.Int).SetUint64(n).String())))
			},
		},
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

// WithMainnetChainID sets the chain ID to flow evm testnet
func WithTestnetChainID() Option {
	return func(c *Config) *Config {
		c.ChainConfig.ChainID = FlowEVMTestnetChainID
		return c
	}
}

// WithMainnetChainID sets the chain ID to flow evm mainnet
func WithMainnetChainID() Option {
	return func(c *Config) *Config {
		c.ChainConfig.ChainID = FlowEVMMainnetChainID
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

// WithExtraPrecompiles appends precompile list with extra precompiles
func WithExtraPrecompiles(precompiles []types.Precompile) Option {
	return func(c *Config) *Config {
		for _, pc := range precompiles {
			if c.ExtraPrecompiles == nil {
				c.ExtraPrecompiles = make(map[gethCommon.Address]gethVM.PrecompiledContract)
			}
			c.ExtraPrecompiles[pc.Address().ToCommon()] = pc
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
