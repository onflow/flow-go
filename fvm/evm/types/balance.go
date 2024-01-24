package types

import (
	"fmt"
	"math"
	"math/big"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/fixedpoint"
)

var (
	AttoScale                      = 18
	UFixedScale                    = fixedpoint.Fix64Scale
	UFixedToAttoConversionScale    = AttoScale - UFixedScale
	UFixToAttoConversionMultiplier = new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(UFixedToAttoConversionScale)), nil)

	OneFlowInUFix64 = cadence.UFix64(uint64(math.Pow(10, float64(UFixedScale))))
	OneFlowBalance  = Balance(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(AttoScale)), nil))
)

// Balance represents the balance of an address
// in the evm environment, balances are kept in attoflow (1e10^-18 flow),
// the smallest denomination of FLOW token (similar to how Wei is used to store Eth)
// But on the FLOW Vaults, we use Cadence.UFix64 to store values in Flow.
// this could result in accidental conversion mistakes, the balance object here would
// do the conversions and does appropriate checks.
type Balance *big.Int

// NewBalanceconstructs a new balance from an atto flow value
func NewBalance(inp *big.Int) Balance {
	return Balance(inp)
}

// NewBalanceFromUFix64 constructs a new balance from flow value (how its stored in Cadence Flow)
func NewBalanceFromUFix64(inp cadence.UFix64) Balance {
	return new(big.Int).Mul(
		new(big.Int).SetUint64(uint64(inp)),
		UFixToAttoConversionMultiplier)
}

// CopyBalance creates a copy of the balance
func CopyBalance(inp Balance) Balance {
	return Balance(new(big.Int).Set(inp))
}

// BalanceToBigInt convert balance into big int
func BalanceToBigInt(bal Balance) *big.Int {
	return (*big.Int)(bal)
}

// ConvertBalanceToUFix64 casts the balance into a UFix64,
//
// Warning! The smallest unit of Flow token that a FlowVault (Cadence) could store is 1e10^-8,
// so transfering smaller values (or values with smalls fractions) could result in loss in
// conversion. The rounded flag should be used to prevent loss of assets.
func ConvertBalanceToUFix64(bal Balance) (value cadence.UFix64, roundedOff bool, err error) {
	converted := new(big.Int).Div(bal, UFixToAttoConversionMultiplier)
	if !converted.IsUint64() {
		// this should never happen
		err = fmt.Errorf("balance can't be casted to a uint64")
	}
	return cadence.UFix64(converted.Uint64()), BalanceConvertionToUFix64ProneToRoundingError(bal), err

}

// BalanceConvertionToUFix64ProneToRoundingError returns true
// if casting to UFix64 could result in rounding error
func BalanceConvertionToUFix64ProneToRoundingError(bal Balance) bool {
	return new(big.Int).Mod(bal, UFixToAttoConversionMultiplier).BitLen() != 0
}

// Subtract balance 2 from balance 1 and returns the result as a new balance
func SubBalance(bal1 Balance, bal2 Balance) (Balance, error) {
	if (*big.Int)(bal1).Cmp(bal2) == -1 {
		return nil, ErrInsufficientBalance
	}
	return new(big.Int).Sub(bal1, bal2), nil
}

// AddBalance balance 2 to balance 1 and returns the result as a new balance
func AddBalance(bal1 Balance, bal2 Balance) (Balance, error) {
	return new(big.Int).Add(bal1, bal2), nil
}

// MakeABalanceInFlow makes a balance object that has `amount` Flow Token in it
func MakeABalanceInFlow(amount uint64) Balance {
	return NewBalance(new(big.Int).Mul(OneFlowBalance, new(big.Int).SetUint64(amount)))
}
