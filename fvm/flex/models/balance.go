package models

import (
	"math/big"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/fixedpoint"
)

// Balance represents the balance of a Flex address
// in the flex environment, balances are kept in attoflow,
// the smallest denomination of FLOW token (similar to how Wei is used to store Eth)
// But on the FLOW Vaults, we use Cadence.UFix64 to store values in Flow.
// this type is defined to minimize the chance of mistake when dealing with the converision
type Balance cadence.UFix64

func (b Balance) ToAttoFlow() *big.Int {
	res := new(big.Int)
	conv := new(big.Int).SetInt64(1e18)
	integer := new(big.Int).SetUint64(uint64(b) / fixedpoint.Fix64Scale)
	fraction := new(big.Int).SetUint64(uint64(b) % fixedpoint.Fix64Scale)
	scaledInteger := integer.Mul(integer, conv)
	return res.Add(scaledInteger, fraction)
}

func (b Balance) Sub(other Balance) Balance {
	// TODO check for underflow
	return Balance(uint64(b) - uint64(other))
}

func (b Balance) Add(other Balance) Balance {
	// TODO check for overflow
	return Balance(uint64(b) + uint64(other))
}

func NewBalanceFromAttoFlow(inp *big.Int) (Balance, error) {
	conv := new(big.Int).SetInt64(1e18)
	integer := inp.Div(inp, conv)
	fraction := inp.Rem(inp, conv)

	// TODO check the underlying cadence method errors on underflow and over flow
	v, err := cadence.NewUFix64FromParts(int(integer.Int64()), uint(fraction.Uint64()))
	if err != nil {
		return 0, err
	}

	return Balance(v), nil
}
