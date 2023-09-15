package flex

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// TODO we might need to register our chain ID here https://github.com/DefiLlama/chainlist/blob/main/constants/chainIds.json
var (
	FlexTestnetChainID     = big.NewInt(666)
	FlexMainnetChainID     = big.NewInt(777)
	DefaultMaxGasLimit     = uint64(30_000_000)
	TransferGasUsage       = uint64(21_000)
	DefaultGasPrice        = big.NewInt(1)
	BlockNumberForEVMRules = big.NewInt(18138954) // a recent block to be used as a refrence for the EVM setup

	// ErrFlexEnvReuse is returned when a flex environment is used more than once
	ErrFlexEnvReuse        = errors.New("flex env has been used")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrGasLimit            = errors.New("gas limit hit")
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

func defaultConfig() *Config {
	return &Config{
		ChainConfig: &params.ChainConfig{
			ChainID:             FlexTestnetChainID, // default is testnet
			HomesteadBlock:      new(big.Int),
			DAOForkBlock:        new(big.Int),
			DAOForkSupport:      false,
			EIP150Block:         new(big.Int),
			EIP155Block:         new(big.Int),
			EIP158Block:         new(big.Int),
			ByzantiumBlock:      new(big.Int),
			ConstantinopleBlock: new(big.Int),
			PetersburgBlock:     new(big.Int),
			IstanbulBlock:       new(big.Int),
			MuirGlacierBlock:    new(big.Int),
			BerlinBlock:         new(big.Int),
			LondonBlock:         new(big.Int),
		},
		TxContext: &vm.TxContext{
			GasPrice: DefaultGasPrice,
		},
		BlockContext: &vm.BlockContext{
			CanTransfer: core.CanTransfer,
			Transfer:    core.Transfer,
			GasLimit:    DefaultMaxGasLimit,
			BaseFee:     big.NewInt(1), // small base fee for block
			GetHash: func(n uint64) common.Hash {
				return common.BytesToHash(crypto.Keccak256([]byte(new(big.Int).SetUint64(n).String())))
			},
		},
	}
}

// NewFlexConfig initializes a new flex configuration
func NewFlexConfig(opts ...Option) *Config {
	ctx := defaultConfig()
	for _, applyOption := range opts {
		ctx = applyOption(ctx)
	}
	return ctx
}

// returns the chain rules based on the block number
func (fc *Config) Rules() params.Rules {
	return fc.ChainConfig.Rules(BlockNumberForEVMRules, false, fc.BlockContext.Time)
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

// WithBlobHashes sets the blob hash part of the transaction context
// not used at the moment but would be useful in near future
func WithBlobHashes(hashes []common.Hash) Option {
	return func(c *Config) *Config {
		c.TxContext.BlobHashes = hashes
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
