package emulator

import (
	"fmt"
	"github.com/onflow/cadence"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/meter"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

// config is a set of configuration options for an emulated emulator.
type config struct {
	ServiceKey                   *ServiceKey
	Store                        EmulatorStorage
	SimpleAddresses              bool
	GenesisTokenSupply           cadence.UFix64
	TransactionMaxGasLimit       uint64
	ScriptGasLimit               uint64
	TransactionExpiry            uint
	StorageLimitEnabled          bool
	TransactionFeesEnabled       bool
	ExecutionEffortWeights       meter.ExecutionEffortWeights
	ContractRemovalEnabled       bool
	MinimumStorageReservation    cadence.UFix64
	StorageMBPerFLOW             cadence.UFix64
	Logger                       zerolog.Logger
	ServerLogger                 zerolog.Logger
	TransactionValidationEnabled bool
	ChainID                      flowgo.ChainID
	AutoMine                     bool
}

const defaultGenesisTokenSupply = "1000000000.0"
const defaultScriptGasLimit = 100000
const defaultTransactionMaxGasLimit = flowgo.DefaultMaxTransactionGasLimit

// defaultConfig is the default configuration for an emulated emulator.
var defaultConfig = func() config {
	genesisTokenSupply, err := cadence.NewUFix64(defaultGenesisTokenSupply)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse default genesis token supply: %s", err.Error()))
	}

	return config{
		ServiceKey:                   DefaultServiceKey(),
		Store:                        nil,
		SimpleAddresses:              false,
		GenesisTokenSupply:           genesisTokenSupply,
		ScriptGasLimit:               defaultScriptGasLimit,
		TransactionMaxGasLimit:       defaultTransactionMaxGasLimit,
		MinimumStorageReservation:    fvm.DefaultMinimumStorageReservation,
		StorageMBPerFLOW:             fvm.DefaultStorageMBPerFLOW,
		TransactionExpiry:            0, // TODO: replace with sensible default
		StorageLimitEnabled:          true,
		Logger:                       zerolog.Nop(),
		ServerLogger:                 zerolog.Nop(),
		TransactionValidationEnabled: true,
		ChainID:                      flowgo.Emulator,
		AutoMine:                     false,
	}
}()

func (conf config) GetStore() EmulatorStorage {
	if conf.Store == nil {
		conf.Store = NewMemoryStore()
	}
	return conf.Store
}

func (conf config) GetChainID() flowgo.ChainID {
	if conf.SimpleAddresses {
		return flowgo.MonotonicEmulator
	}
	return conf.ChainID
}

func (conf config) GetServiceKey() ServiceKey {
	// set up service key
	serviceKey := conf.ServiceKey
	if serviceKey == nil {
		serviceKey = DefaultServiceKey()
	}
	serviceKey.Address = conf.GetChainID().Chain().ServiceAddress()
	serviceKey.Weight = fvm.AccountKeyWeightThreshold
	return *serviceKey
}

// Option is a function applying a change to the emulator config.
type Option func(*config)

// WithLogger sets the fvm logger
func WithLogger(
	logger zerolog.Logger,
) Option {
	return func(c *config) {
		c.Logger = logger
	}
}

// WithServerLogger sets the logger
func WithServerLogger(
	logger zerolog.Logger,
) Option {
	return func(c *config) {
		c.ServerLogger = logger
	}
}

// WithServicePublicKey sets the service key from a public key.
func WithServicePublicKey(
	servicePublicKey crypto.PublicKey,
	sigAlgo crypto.SigningAlgorithm,
	hashAlgo hash.HashingAlgorithm,
) Option {
	return func(c *config) {
		c.ServiceKey = &ServiceKey{
			PublicKey: servicePublicKey,
			SigAlgo:   sigAlgo,
			HashAlgo:  hashAlgo,
		}
	}
}

// WithServicePrivateKey sets the service key from private key.
func WithServicePrivateKey(
	privateKey crypto.PrivateKey,
	sigAlgo crypto.SigningAlgorithm,
	hashAlgo hash.HashingAlgorithm,
) Option {
	return func(c *config) {
		c.ServiceKey = &ServiceKey{
			PrivateKey: privateKey,
			PublicKey:  privateKey.PublicKey(),
			HashAlgo:   hashAlgo,
			SigAlgo:    sigAlgo,
		}
	}
}

// WithStore sets the persistent storage provider.
func WithStore(store EmulatorStorage) Option {
	return func(c *config) {
		c.Store = store
	}
}

// WithSimpleAddresses enables simple addresses, which are sequential starting with 0x01.
func WithSimpleAddresses() Option {
	return func(c *config) {
		c.SimpleAddresses = true
	}
}

// WithGenesisTokenSupply sets the genesis token supply.
func WithGenesisTokenSupply(supply cadence.UFix64) Option {
	return func(c *config) {
		c.GenesisTokenSupply = supply
	}
}

// WithTransactionMaxGasLimit sets the maximum gas limit for transactions.
//
// Individual transactions will still be bounded by the limit they declare.
// This function sets the maximum limit that any transaction can declare.
//
// This limit does not affect script executions. Use WithScriptGasLimit
// to set the gas limit for script executions.
func WithTransactionMaxGasLimit(maxLimit uint64) Option {
	return func(c *config) {
		c.TransactionMaxGasLimit = maxLimit
	}
}

// WithScriptGasLimit sets the gas limit for scripts.
//
// This limit does not affect transactions, which declare their own limit.
// Use WithTransactionMaxGasLimit to set the maximum gas limit for transactions.
func WithScriptGasLimit(limit uint64) Option {
	return func(c *config) {
		c.ScriptGasLimit = limit
	}
}

// WithTransactionExpiry sets the transaction expiry measured in blocks.
//
// If set to zero, transaction expiry is disabled and the reference block ID field
// is not required.
func WithTransactionExpiry(expiry uint) Option {
	return func(c *config) {
		c.TransactionExpiry = expiry
	}
}

// WithStorageLimitEnabled enables/disables limiting account storage used to their storage capacity.
//
// If set to false, accounts can store any amount of data,
// otherwise they can only store as much as their storage capacity.
// The default is true.
func WithStorageLimitEnabled(enabled bool) Option {
	return func(c *config) {
		c.StorageLimitEnabled = enabled
	}
}

// WithMinimumStorageReservation sets the minimum account balance.
//
// The cost of creating new accounts is also set to this value.
// The default is taken from fvm.DefaultMinimumStorageReservation
func WithMinimumStorageReservation(minimumStorageReservation cadence.UFix64) Option {
	return func(c *config) {
		c.MinimumStorageReservation = minimumStorageReservation
	}
}

// WithStorageMBPerFLOW sets the cost of a megabyte of storage in FLOW
//
// the default is taken from fvm.DefaultStorageMBPerFLOW
func WithStorageMBPerFLOW(storageMBPerFLOW cadence.UFix64) Option {
	return func(c *config) {
		c.StorageMBPerFLOW = storageMBPerFLOW
	}
}

// WithTransactionFeesEnabled enables/disables transaction fees.
//
// If set to false transactions don't cost any flow.
// The default is false.
func WithTransactionFeesEnabled(enabled bool) Option {
	return func(c *config) {
		c.TransactionFeesEnabled = enabled
	}
}

// WithExecutionEffortWeights sets the execution effort weights.
// default is the Mainnet values.
func WithExecutionEffortWeights(weights meter.ExecutionEffortWeights) Option {
	return func(c *config) {
		c.ExecutionEffortWeights = weights
	}
}

// WithContractRemovalEnabled restricts/allows removal of already deployed contracts.
//
// The default is provided by on-chain value.
func WithContractRemovalEnabled(enabled bool) Option {
	return func(c *config) {
		c.ContractRemovalEnabled = enabled
	}
}

// WithTransactionValidationEnabled enables/disables transaction validation.
//
// If set to false, the emulator will not verify transaction signatures or validate sequence numbers.
//
// The default is true.
func WithTransactionValidationEnabled(enabled bool) Option {
	return func(c *config) {
		c.TransactionValidationEnabled = enabled
	}
}

// WithChainID sets chain type for address generation
// The default is emulator.
func WithChainID(chainID flowgo.ChainID) Option {
	return func(c *config) {
		c.ChainID = chainID
	}
}
