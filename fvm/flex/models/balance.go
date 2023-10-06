package models

import (
	"encoding/binary"
	"math/big"

	"github.com/onflow/cadence"
)

// Balance represents the balance of a Flex address
// in the flex environment, balances are kept in attoflow,
// the smallest denomination of FLOW token (similar to how Wei is used to store Eth)
// But on the FLOW Vaults, we use Cadence.UFix64 to store values in Flow.
// this type is defined to minimize the chance of mistake when dealing with the converision
type Balance cadence.UFix64

func (b Balance) ToAttoFlow() *big.Int {
	conv := new(big.Int).SetInt64(1e10)
	return new(big.Int).Mul(new(big.Int).SetUint64(uint64(b)), conv)
}

func (b Balance) Sub(other Balance) Balance {
	// TODO check for underflow
	return Balance(uint64(b) - uint64(other))
}

func (b Balance) Add(other Balance) Balance {
	// TODO check for overflow
	return Balance(uint64(b) + uint64(other))
}

func (b Balance) Encode() []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, b.ToAttoFlow().Uint64())
	return encoded
}

func DecodeBalance(encoded []byte) (Balance, error) {
	balance := new(big.Int)
	return NewBalanceFromAttoFlow(balance.SetUint64(binary.BigEndian.Uint64(encoded)))
}

func NewBalanceFromAttoFlow(inp *big.Int) (Balance, error) {
	conv := new(big.Int).SetInt64(1e10)
	// TODO: check for underflow
	// rem := new(big.Int).Rem(inp, conv)
	// if rem != 0 {
	// 	return error
	// }

	// we only need to divide by 10 given we already have 8 as factor
	converted := new(big.Int).Div(inp, conv)
	return Balance(cadence.UFix64(converted.Uint64())), nil
}
