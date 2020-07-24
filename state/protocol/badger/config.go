package badger

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Config struct {
	transactionExpiry uint64 // how many blocks after the reference block a transaction expires
	auctionWindow     uint64 // how many views at the beginning of a new epoch for seat auction
	gracePeriod       uint64 // how many views at the ond af a new epoch to wait for nodes
}

func DefaultConfig() Config {
	return Config{
		transactionExpiry: flow.DefaultTransactionExpiry,
		auctionWindow:     flow.DefaultAuctionWindow,
		gracePeriod:       flow.DefaultGracePeriod,
	}
}
