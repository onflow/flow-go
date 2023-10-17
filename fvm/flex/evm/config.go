package evm

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// TODO we might need to register our chain ID here:
//  https://github.com/DefiLlama/chainlist/blob/main/constants/chainIds.json

var (
	FlexTestnetChainID = big.NewInt(666)
	FlexMainnetChainID = big.NewInt(777)
	TransferGasUsage   = uint64(21_000)
	DefaultGasPrice    = big.NewInt(0)
	// anything block number above 0 works here
	BlockNumberForEVMRules = big.NewInt(1)
)

// Config sets the required parameters
type Config struct {
	// Chain Config
	ChainConfig *params.ChainConfig
	// EVM config
	EVMConfig vm.Config
	// block context
	BlockContext *vm.BlockContext
	// transaction context
	TxContext *vm.TxContext
}

var zero = uint64(0)

func defaultConfig() *Config {
	return &Config{
		ChainConfig: &params.ChainConfig{
			ChainID: FlexTestnetChainID, // default is testnet

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
		},
		EVMConfig: vm.Config{
			NoBaseFee: true,
		},
		TxContext: &vm.TxContext{
			GasPrice: DefaultGasPrice,
		},
		BlockContext: &vm.BlockContext{
			BlockNumber: BlockNumberForEVMRules, // default block number to make setup right
			CanTransfer: core.CanTransfer,
			Transfer:    core.Transfer,
			GasLimit:    math.MaxUint64, // block gas limit
			BaseFee:     big.NewInt(1),  // small base fee for block
			GetHash: func(n uint64) common.Hash { // default returns some random hash values
				return common.BytesToHash(crypto.Keccak256([]byte(new(big.Int).SetUint64(n).String())))
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

// WithMainnetChainID sets the chain ID to flex testnet
func WithTestnetChainID() Option {
	return func(c *Config) *Config {
		c.ChainConfig.ChainID = FlexTestnetChainID
		return c
	}
}

// WithMainnetChainID sets the chain ID to flex mainnet
func WithMainnetChainID() Option {
	return func(c *Config) *Config {
		c.ChainConfig.ChainID = FlexMainnetChainID
		return c
	}

}

// WithOrigin sets the origin of the transaction (signer)
func WithOrigin(origin common.Address) Option {
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

// WithBaseFee sets the the base fee for each transaction
// ramtin: it think this is similar to inclusion fee but I need to validate
func WithBaseFee(baseFee *big.Int) Option {
	return func(c *Config) *Config {
		c.BlockContext.BaseFee = baseFee
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
func WithCoinbase(coinbase common.Address) Option {
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
func WithGetBlockHashFunction(getHash vm.GetHashFunc) Option {
	return func(c *Config) *Config {
		c.BlockContext.GetHash = getHash
		return c
	}
}
