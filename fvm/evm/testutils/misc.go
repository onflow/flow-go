package testutils

import (
	cryptoRand "crypto/rand"
	"math"
	"math/big"
	"math/rand"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func RandomCommonHash(t testing.TB) gethCommon.Hash {
	ret := gethCommon.Hash{}
	_, err := cryptoRand.Read(ret[:gethCommon.HashLength])
	require.NoError(t, err)
	return ret
}

func RandomBigInt(limit int64) *big.Int {
	return big.NewInt(rand.Int63n(limit) + 1)
}

func RandomAddress(t testing.TB) types.Address {
	return types.NewAddress(RandomCommonAddress(t))
}

func RandomCommonAddress(t testing.TB) gethCommon.Address {
	ret := gethCommon.Address{}
	_, err := cryptoRand.Read(ret[:gethCommon.AddressLength])
	require.NoError(t, err)
	return ret
}

func RandomGas(limit int64) uint64 {
	return uint64(rand.Int63n(limit) + 1)
}

func RandomData(t testing.TB) []byte {
	// byte size [1, 100]
	size := rand.Intn(100) + 1
	ret := make([]byte, size)
	_, err := cryptoRand.Read(ret[:])
	require.NoError(t, err)
	return ret
}

func GetRandomLogFixture(t testing.TB) *gethTypes.Log {
	return &gethTypes.Log{
		Address: RandomCommonAddress(t),
		Topics: []gethCommon.Hash{
			RandomCommonHash(t),
			RandomCommonHash(t),
		},
		Data: RandomData(t),
	}
}

// MakeABalanceInFlow makes a balance object that has `amount` Flow Token in it
func MakeABalanceInFlow(amount uint64) *types.Balance {
	return types.NewBalance(cadence.UFix64(uint64(math.Pow(10, float64(types.UFixedScale))) * amount))
}
