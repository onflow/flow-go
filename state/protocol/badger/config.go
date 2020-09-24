package badger

import (
	"github.com/onflow/flow-go/model/flow"
)

type Config struct {
	transactionExpiry uint64 // how many blocks after the reference block a transaction expires
}

func DefaultConfig() Config {
	return Config{
		transactionExpiry: flow.DefaultTransactionExpiry,
	}
}
