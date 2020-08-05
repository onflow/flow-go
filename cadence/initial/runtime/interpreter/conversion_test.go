package interpreter

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByteArrayValueToByteSlice(t *testing.T) {

	t.Parallel()

	t.Run("invalid", func(t *testing.T) {

		largeBigInt, ok := new(big.Int).SetString("1000000000000000000000000000000000000000000000", 10)
		require.True(t, ok)

		invalid := []Value{
			NewArrayValueUnownedNonCopying(UInt64Value(500)),
			NewArrayValueUnownedNonCopying(NewInt256ValueFromBigInt(largeBigInt)),
			UInt64Value(500),
			BoolValue(true),
			NewStringValue("test"),
		}

		for _, value := range invalid {
			_, err := ByteArrayValueToByteSlice(value)
			require.Error(t, err)
		}
	})

	t.Run("valid", func(t *testing.T) {

		invalid := map[Value][]byte{
			NewArrayValueUnownedNonCopying():                                             {},
			NewArrayValueUnownedNonCopying(UInt64Value(2), NewUInt128ValueFromUint64(3)): {2, 3},
			NewArrayValueUnownedNonCopying(UInt8Value(4), NewIntValueFromInt64(5)):       {4, 5},
		}

		for value, expected := range invalid {
			result, err := ByteArrayValueToByteSlice(value)
			require.NoError(t, err)
			require.Equal(t, expected, result)
		}
	})
}

func TestByteValueToByte(t *testing.T) {

	t.Parallel()

	t.Run("invalid", func(t *testing.T) {

		largeBigInt, ok := new(big.Int).SetString("1000000000000000000000000000000000000000000000", 10)
		require.True(t, ok)

		invalid := []Value{
			UInt64Value(500),
			NewInt256ValueFromBigInt(largeBigInt),
		}

		for _, value := range invalid {
			_, err := ByteValueToByte(value)
			require.Error(t, err)
		}
	})

	t.Run("valid", func(t *testing.T) {

		const maxInt8Plus2 = math.MaxInt8 + 2

		invalid := map[Value]byte{
			UInt64Value(2):               2,
			NewUInt128ValueFromUint64(3): 3,
			UInt8Value(4):                4,
			NewIntValueFromInt64(5):      5,
			UInt8Value(maxInt8Plus2):     maxInt8Plus2,
		}

		for value, expected := range invalid {
			result, err := ByteValueToByte(value)
			require.NoError(t, err)
			require.Equal(t, expected, result)
		}
	})
}
