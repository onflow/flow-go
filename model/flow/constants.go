package flow

import (
	"fmt"
	"sync"
	"time"
)

// A ChainID is a unique identifier for a specific Flow network instance.
//
// Chain IDs are used used to prevent replay attacks and to support network-specific address generation.
type ChainID string

// Mainnet is the chain ID for the mainnet node chain.
const Mainnet ChainID = "flow-mainnet"

// Testnet is the chain ID for the testnet node chain.
const Testnet ChainID = "flow-testnet"

// Emulator is the chain ID for the emulated node chain.
const Emulator ChainID = "flow-emulator"

func (id ChainID) String() string {
	return string(id)
}

var chainIDOnce sync.Once
var chainID = Mainnet

// SetChainID sets the global chain ID.
//
// This function should only be called once per process. Subsequent calls
// will not update the chain ID.
func SetChainID(id ChainID) {
	chainIDOnce.Do(func() { setChainID(id) })
}

// setChainID is an unsafe version of SetChainID that does not enforce singleton behaviour.
func setChainID(id ChainID) {
	switch id {
	case Mainnet, Testnet, Emulator:
		chainID = id
	default:
		panic(fmt.Sprintf("invalid chain ID %s", id))
	}
}

// GetChainID returns the global chain ID.
func GetChainID() ChainID {
	return chainID
}

// DefaultTransactionExpiry is the default expiry for transactions, measured
// in blocks. Equivalent to 10 minutes for a 1-second block time.
const DefaultTransactionExpiry = 10 * 60

// DefaultMaxGasLimit is the default maximum value for the transaction gas limit.
const DefaultMaxGasLimit = 9999

func GenesisTime() time.Time {
	return time.Date(2018, time.December, 19, 22, 32, 30, 42, time.UTC)
}
