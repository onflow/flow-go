package types

import (
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/fixedpoint"
)

var (
	AttoScale                      = 18
	UFixedScale                    = fixedpoint.Fix64Scale
	UFixedToAttoConversionScale    = AttoScale - UFixedScale
	UFixToAttoConversionMultiplier = new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(UFixedToAttoConversionScale)), nil)

	OneFlow           = cadence.UFix64(uint64(math.Pow(10, float64(UFixedScale))))
	OneFlowInAttoFlow = new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(AttoScale)), nil)
)

// Balance represents the balance of an address
// in the evm environment, balances are kept in attoflow (1e10^-18 flow),
// the smallest denomination of FLOW token (similar to how Wei is used to store Eth)
// But on the FLOW Vaults, we use Cadence.UFix64 to store values in Flow.
// this could result in accidental conversion mistakes, the balance object here would
// do the conversions and does appropriate checks.
type Balance struct {
	Value *big.Int
}

// NewBalance constructs a new balance from flow value (how its stored in Cadence Flow)
func NewBalance(inp cadence.UFix64) *Balance {
	return &Balance{
		Value: new(big.Int).Mul(
			new(big.Int).SetUint64(uint64(inp)),
			UFixToAttoConversionMultiplier),
	}
}

// NewBalanceFromAttoFlow constructs a new balance from atto flow value
func NewBalanceFromAttoFlow(inp *big.Int) *Balance {
	return &Balance{
		Value: inp,
	}
}

// ToAttoFlow returns the balance in AttoFlow
func (b *Balance) ToAttoFlow() *big.Int {
	return b.Value
}

// Copy creates a copy of this balance
func (b *Balance) Copy() *Balance {
	return NewBalanceFromAttoFlow(b.ToAttoFlow())
}

// ToUFix64 casts the balance into a UFix64,
//
// Warning! The smallest unit of Flow token that a FlowVault (Cadence) could store is 1e10^-8,
// so transfering smaller values (or values with smalls fractions) could result in loss in
// conversion. The rounded flag should be used to prevent loss of assets.
func (b *Balance) ToUFix64() (cadence.UFix64, error) {
	var err error
	converted := new(big.Int).Div(b.Value, UFixToAttoConversionMultiplier)
	if !converted.IsUint64() {
		// this should never happen
		err = fmt.Errorf("balance can't be casted to a uint64")
	}
	return cadence.UFix64(converted.Uint64()), err
}

// HasUFix64RoundingError returns true if casting to UFix64 has rounding error
func (b *Balance) HasUFix64RoundingError() bool {
	return new(big.Int).Mod(b.Value, UFixToAttoConversionMultiplier).Cmp(big.NewInt(0)) != 0
}

// Sub subtract the other balance from this balance
func (b *Balance) Sub(other *Balance) error {
	otherInAtto := other.ToAttoFlow()
	// check underflow b < other
	if b.Value.Cmp(otherInAtto) == -1 {
		return ErrWithdrawBalanceRounding
	}
	b.Value = new(big.Int).Sub(b.Value, otherInAtto)
	return nil
}

// Add adds the other balance to this balance
func (b *Balance) Add(other *Balance) error {
	// no need to check for overflow, as go does it
	b.Value = new(big.Int).Add(b.Value, other.ToAttoFlow())
	return nil
}

// Encode encodes the balance into byte slice
func (b *Balance) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(b)
}

// DecodeBalance decodes a balance from an encoded byte slice
func DecodeBalance(encoded []byte) (*Balance, error) {
	b := &Balance{}
	return b, rlp.DecodeBytes(encoded, b)

}
