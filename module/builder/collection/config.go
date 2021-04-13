package collection

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultExpiryBuffer            uint    = 15 // 15 blocks for collections to be included
	DefaultMaxPayerTransactionRate float64 = 0  // no rate limiting
)

// Config is the configurable options for the collection builder.
type Config struct {

	// MaxCollectionSize is the maximum size of collections.
	MaxCollectionSize uint

	// ExpiryBuffer is how much buffer we add when considering transaction
	// expiry. If the buffer is set to 10, and a transaction actually expires
	// in 15 blocks, we consider it expired in 5 (15-10) blocks. This accounts
	// for the time between the collection being built and being included in
	// block.
	ExpiryBuffer uint

	// MaxPayerTransactionRate is the maximum number of transactions per payer
	// per collection. Fractional values greater than 1 are rounded down.
	// Fractional values 0<k<1 mean that only 1 transaction every ceil(1/k)
	// collections is allowed.
	//
	// A negative value or 0 indicates no rate limiting.
	MaxPayerTransactionRate float64

	// UnlimitedPayer is a set of addresses which are not affected by per-payer
	// rate limiting.
	UnlimitedPayers map[flow.Address]struct{}

	// MaxCollectionByteSize is the maximum byte size of a collection.
	MaxCollectionByteSize uint64

	// MaxCollectionTotalGas is the maximum of total of gas per collection (sum of maxGasLimit over transactions)
	MaxCollectionTotalGas uint64
}

func DefaultConfig() Config {
	return Config{
		MaxCollectionSize:       flow.DefaultMaxCollectionSize,
		ExpiryBuffer:            DefaultExpiryBuffer,
		MaxPayerTransactionRate: DefaultMaxPayerTransactionRate,
		UnlimitedPayers:         make(map[flow.Address]struct{}), // no unlimited payers
		MaxCollectionByteSize:   flow.DefaultMaxCollectionByteSize,
		MaxCollectionTotalGas:   flow.DefaultMaxCollectionTotalGas,
	}
}

type Opt func(config *Config)

func WithMaxCollectionSize(size uint) Opt {
	return func(c *Config) {
		c.MaxCollectionSize = size
	}
}

func WithExpiryBuffer(buf uint) Opt {
	return func(c *Config) {
		c.ExpiryBuffer = buf
	}
}

func WithMaxPayerTransactionRate(rate float64) Opt {
	return func(c *Config) {
		if rate < 0 {
			rate = 0
		}
		c.MaxPayerTransactionRate = rate
	}
}

func WithUnlimitedPayers(payers ...flow.Address) Opt {
	lookup := make(map[flow.Address]struct{})
	for _, payer := range payers {
		lookup[payer] = struct{}{}
	}
	return func(c *Config) {
		c.UnlimitedPayers = lookup
	}
}

func WithMaxCollectionByteSize(limit uint64) Opt {
	return func(c *Config) {
		c.MaxCollectionByteSize = limit
	}
}

func WithMaxCollectionTotalGas(limit uint64) Opt {
	return func(c *Config) {
		c.MaxCollectionTotalGas = limit
	}
}
