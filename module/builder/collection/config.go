package collection

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
	// per collection. The rate is computed based on the previous 10 blocks.
	//
	// A value of 0 indicates no rate limiting.
	MaxPayerTransactionRate float64
}

func DefaultConfig() Config {
	return Config{
		MaxCollectionSize:       100, // max 100 transactions per collection
		ExpiryBuffer:            15,  // leave 15 blocks of time for collections to be included
		MaxPayerTransactionRate: 0,   // no rate limiting
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
		c.MaxPayerTransactionRate = rate
	}
}
