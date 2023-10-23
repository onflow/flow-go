package types

import (
	"encoding/binary"
	"math/big"

	"github.com/onflow/cadence"
)

var (
	SmalletUnitOfAttoFlowConvertibleToCadence = new(big.Int).SetInt64(1e10)
	OneAttoFlow                               = new(big.Int).SetInt64(1)
	OneFlowInAttoFlow                         = new(big.Int).SetInt64(1e18)
)

// Balance represents the balance of an address
// in the evm environment, balances are kept in attoflow,
// the smallest denomination of FLOW token (similar to how Wei is used to store Eth)
// But on the FLOW Vaults, we use Cadence.UFix64 to store values in Flow.
// this type is defined to minimize the chance of mistake when dealing with the converision
type Balance cadence.UFix64

// ToAttoFlow converts the balance into AttoFlow
func (b Balance) ToAttoFlow() *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(uint64(b)), SmalletUnitOfAttoFlowConvertibleToCadence)
}

// Sub subtract the other balance from this balance
func (b Balance) Sub(other Balance) Balance {
	// no need to check for underflow, as go does it
	return Balance(uint64(b) - uint64(other))
}

// Add adds the other balance from this balance
func (b Balance) Add(other Balance) Balance {
	// no need to check for overflow, as go does it
	return Balance(uint64(b) + uint64(other))
}

// Encode encodes the balance into byte slice
func (b Balance) Encode() []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, b.ToAttoFlow().Uint64())
	return encoded
}

// DecodeBalance decodes a balance from an encoded byte slice
func DecodeBalance(encoded []byte) (Balance, error) {
	balance := new(big.Int)
	return NewBalanceFromAttoFlow(balance.SetUint64(binary.BigEndian.Uint64(encoded)))
}

// NewBalanceFromAttoFlow constructs a new balance from atto flow value
func NewBalanceFromAttoFlow(inp *big.Int) (Balance, error) {
	if new(big.Int).Mod(inp, SmalletUnitOfAttoFlowConvertibleToCadence).Cmp(big.NewInt(0)) != 0 {
		return 0, ErrBalanceConversion
	}

	// we only need to divide by 10 given we already have 8 as factor
	converted := new(big.Int).Div(inp, SmalletUnitOfAttoFlowConvertibleToCadence)
	return Balance(cadence.UFix64(converted.Uint64())), nil
}
