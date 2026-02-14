package filter

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	cadencecommon "github.com/onflow/cadence/common"
	fixedPoint "github.com/onflow/fixed-point"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allCadenceTypes = map[cadence.Type]struct{}{
	cadence.IntType:       {},
	cadence.Int8Type:      {},
	cadence.Int16Type:     {},
	cadence.Int32Type:     {},
	cadence.Int64Type:     {},
	cadence.Int128Type:    {},
	cadence.Int256Type:    {},
	cadence.UIntType:      {},
	cadence.UInt8Type:     {},
	cadence.UInt16Type:    {},
	cadence.UInt32Type:    {},
	cadence.UInt64Type:    {},
	cadence.UInt128Type:   {},
	cadence.UInt256Type:   {},
	cadence.Word8Type:     {},
	cadence.Word16Type:    {},
	cadence.Word32Type:    {},
	cadence.Word64Type:    {},
	cadence.Word128Type:   {},
	cadence.Word256Type:   {},
	cadence.Fix64Type:     {},
	cadence.Fix128Type:    {},
	cadence.UFix64Type:    {},
	cadence.UFix128Type:   {},
	cadence.BoolType:      {},
	cadence.StringType:    {},
	cadence.TheBytesType:  {},
	cadence.CharacterType: {},
	cadence.AddressType:   {},

	// TODO: add test for generic path type
	cadence.StoragePathType: {},
	cadence.PublicPathType:  {},

	// &cadence.OptionalType{}:           {},
	// &cadence.VariableSizedArrayType{}: {},
	// &cadence.ConstantSizedArrayType{}: {},
	// &cadence.DictionaryType{}:         {},
}

func TestTypesEqual(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	for cadenceType := range allCadenceTypes {
		value := valueFactory(t, g, cadenceType)

		// base types
		t.Run(fmt.Sprintf("%T equal", value), func(t *testing.T) {
			got, err := typesEqual(value, cadenceType)
			require.NoError(t, err)
			require.True(t, got)
		})
		t.Run(fmt.Sprintf("%T not equal", value), func(t *testing.T) {
			for otherType := range allCadenceTypes {
				if otherType == cadenceType {
					continue // skip same type
				}
				got, err := typesEqual(value, otherType)
				require.NoError(t, err)
				require.Falsef(t, got, fmt.Sprintf("%T should not be equal to %T", cadenceType, otherType))
			}
		})

		// optional
		t.Run(fmt.Sprintf("%T optional equal", value), func(t *testing.T) {
			got, err := typesEqual(cadence.NewOptional(value), cadenceType)
			require.NoError(t, err)
			require.True(t, got)
		})
		t.Run(fmt.Sprintf("%T optional not equal", value), func(t *testing.T) {
			for otherType := range allCadenceTypes {
				if otherType == cadenceType {
					continue // skip same type
				}
				got, err := typesEqual(cadence.NewOptional(value), otherType)
				require.NoError(t, err)
				require.Falsef(t, got, fmt.Sprintf("%T should not be equal to %T", cadenceType, otherType))
			}
		})

		// array
		t.Run(fmt.Sprintf("%T array equal with ArrayType", value), func(t *testing.T) {
			array := cadence.NewArray([]cadence.Value{value})
			array.ArrayType = cadence.NewVariableSizedArrayType(value.Type())

			got, err := typesEqual(array, cadenceType)
			require.NoError(t, err)
			require.True(t, got)
		})
		t.Run(fmt.Sprintf("%T array equal without ArrayType", value), func(t *testing.T) {
			array := cadence.NewArray([]cadence.Value{value})
			got, err := typesEqual(array, cadenceType)
			require.NoError(t, err)
			require.True(t, got)
		})
		t.Run(fmt.Sprintf("%T array not equal with ArrayType", value), func(t *testing.T) {
			array := cadence.NewArray([]cadence.Value{value})
			array.ArrayType = cadence.NewVariableSizedArrayType(value.Type())
			for otherType := range allCadenceTypes {
				if otherType == cadenceType {
					continue // skip same type
				}
				got, err := typesEqual(array, otherType)
				require.NoError(t, err)
				require.Falsef(t, got, fmt.Sprintf("%T should not be equal to %T", cadenceType, otherType))
			}
		})
		t.Run(fmt.Sprintf("%T array not equal without ArrayType", value), func(t *testing.T) {
			array := cadence.NewArray([]cadence.Value{value})
			for otherType := range allCadenceTypes {
				if otherType == cadenceType {
					continue // skip same type
				}
				got, err := typesEqual(array, otherType)
				require.NoError(t, err)
				require.Falsef(t, got, fmt.Sprintf("%T should not be equal to %T", cadenceType, otherType))
			}
		})
	}
}

func TestEqual(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	for cadenceType := range allCadenceTypes {
		value1 := valueFactory(t, g, cadenceType)
		value2 := valueFactory(t, g, cadenceType)

		t.Run(fmt.Sprintf("%T equal", value1), func(t *testing.T) {
			got, err := equal(value1, value1)
			require.NoError(t, err)
			require.True(t, got)
		})

		t.Run(fmt.Sprintf("%T not equal", value1), func(t *testing.T) {
			got, err := equal(value1, value2)
			require.NoError(t, err)
			require.False(t, got)
		})

		t.Run(fmt.Sprintf("%T optional equal", value1), func(t *testing.T) {
			got, err := equal(cadence.NewOptional(value1), value1)
			require.NoError(t, err)
			require.True(t, got)
		})

		t.Run(fmt.Sprintf("%T optional not equal", value1), func(t *testing.T) {
			got, err := equal(cadence.NewOptional(value1), value2)
			require.NoError(t, err)
			require.False(t, got)
		})

		t.Run(fmt.Sprintf("%T array equal", value1), func(t *testing.T) {
			got, err := equal(cadence.NewArray([]cadence.Value{value1}), cadence.NewArray([]cadence.Value{value1}))
			require.ErrorIs(t, err, ErrComparisonNotSupported)
			require.False(t, got)
		})
	}
}

func TestCmp(t *testing.T) {
	// g := fixtures.NewGeneratorSuite()

	t.Run("Int", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt(1),
			cadence.NewInt(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt(1),
			cadence.NewInt(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt(1),
			cadence.NewInt(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Int8", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt8(1),
			cadence.NewInt8(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt8(1),
			cadence.NewInt8(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt8(1),
			cadence.NewInt8(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Int16", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt16(1),
			cadence.NewInt16(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt16(1),
			cadence.NewInt16(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt16(1),
			cadence.NewInt16(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Int32", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt32(1),
			cadence.NewInt32(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt32(1),
			cadence.NewInt32(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt32(1),
			cadence.NewInt32(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Int64", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt64(1),
			cadence.NewInt64(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt64(1),
			cadence.NewInt64(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt64(1),
			cadence.NewInt64(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Int128", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt128(1),
			cadence.NewInt128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt128(1),
			cadence.NewInt128(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt128(1),
			cadence.NewInt128(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Int256", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewInt256(1),
			cadence.NewInt256(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewInt256(1),
			cadence.NewInt256(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewInt256(1),
			cadence.NewInt256(-1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt(1),
			cadence.NewUInt(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt(1),
			cadence.NewUInt(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt(2),
			cadence.NewUInt(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt8", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt8(1),
			cadence.NewUInt8(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt8(1),
			cadence.NewUInt8(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt8(2),
			cadence.NewUInt8(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt16", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt16(1),
			cadence.NewUInt16(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt16(1),
			cadence.NewUInt16(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt16(2),
			cadence.NewUInt16(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt32", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt32(1),
			cadence.NewUInt32(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt32(1),
			cadence.NewUInt32(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt32(2),
			cadence.NewUInt32(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt64", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt64(1),
			cadence.NewUInt64(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt64(1),
			cadence.NewUInt64(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt64(2),
			cadence.NewUInt64(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt128", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt128(1),
			cadence.NewUInt128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt128(1),
			cadence.NewUInt128(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt128(2),
			cadence.NewUInt128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UInt256", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewUInt256(1),
			cadence.NewUInt256(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewUInt256(1),
			cadence.NewUInt256(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewUInt256(2),
			cadence.NewUInt256(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Word8", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewWord8(1),
			cadence.NewWord8(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewWord8(1),
			cadence.NewWord8(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewWord8(2),
			cadence.NewWord8(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Word16", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewWord16(1),
			cadence.NewWord16(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewWord16(1),
			cadence.NewWord16(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewWord16(2),
			cadence.NewWord16(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Word32", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewWord32(1),
			cadence.NewWord32(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewWord32(1),
			cadence.NewWord32(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewWord32(2),
			cadence.NewWord32(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Word64", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewWord64(1),
			cadence.NewWord64(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewWord64(1),
			cadence.NewWord64(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewWord64(2),
			cadence.NewWord64(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Word128", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewWord128(1),
			cadence.NewWord128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewWord128(1),
			cadence.NewWord128(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewWord128(2),
			cadence.NewWord128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Word256", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewWord256(1),
			cadence.NewWord256(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewWord256(1),
			cadence.NewWord256(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewWord256(2),
			cadence.NewWord256(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Fix64", func(t *testing.T) {
		mustFix64 := func(v string) cadence.Fix64 {
			out, err := cadence.NewFix64(v)
			require.NoError(t, err)
			return out
		}

		actual, err := cmp(
			mustFix64("1.0"),
			mustFix64("1.0"),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustFix64("1.0"),
			mustFix64("2.0"),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustFix64("2.0"),
			mustFix64("1.0"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Fix128", func(t *testing.T) {
		mustFix128 := func(v uint64) cadence.Fix128 {
			out, err := cadence.NewFix128(fixedPoint.NewFix128(0, v))
			require.NoError(t, err)
			return out
		}

		actual, err := cmp(
			mustFix128(1),
			mustFix128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustFix128(1),
			mustFix128(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustFix128(2),
			mustFix128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UFix64", func(t *testing.T) {
		mustUFix64 := func(v string) cadence.UFix64 {
			out, err := cadence.NewUFix64(v)
			require.NoError(t, err)
			return out
		}

		actual, err := cmp(
			mustUFix64("1.0"),
			mustUFix64("1.0"),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustUFix64("1.0"),
			mustUFix64("2.0"),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustUFix64("2.0"),
			mustUFix64("1.0"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("UFix128", func(t *testing.T) {
		mustUFix128 := func(v uint64) cadence.UFix128 {
			out, err := cadence.NewUFix128(fixedPoint.NewUFix128(0, v))
			require.NoError(t, err)
			return out
		}

		actual, err := cmp(
			mustUFix128(1),
			mustUFix128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustUFix128(1),
			mustUFix128(2),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustUFix128(2),
			mustUFix128(1),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("String", func(t *testing.T) {
		mustString := func(v string) cadence.String {
			out, err := cadence.NewString(v)
			require.NoError(t, err)
			return out
		}

		actual, err := cmp(
			mustString("abc"),
			mustString("abc"),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustString("abc"),
			mustString("xyz"),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustString("xyz"),
			mustString("abc"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)

		// note: lowercase are "larger" due to UTF8 encoding
		actual, err = cmp(
			mustString("abc"),
			mustString("ABC"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Character", func(t *testing.T) {
		mustCharacter := func(v string) cadence.Character {
			out, err := cadence.NewCharacter(v)
			require.NoError(t, err)
			return out
		}

		actual, err := cmp(
			mustCharacter("a"),
			mustCharacter("a"),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustCharacter("a"),
			mustCharacter("b"),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustCharacter("z"),
			mustCharacter("a"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)

		// note: lowercase are "larger" due to UTF8 encoding
		actual, err = cmp(
			mustCharacter("a"),
			mustCharacter("A"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Address", func(t *testing.T) {
		mustAddress := func(v string) cadence.Address {
			addr, err := cadencecommon.HexToAddress(v)
			require.NoError(t, err)
			return cadence.NewAddress(addr)
		}

		actual, err := cmp(
			mustAddress("0x1"),
			mustAddress("0x1"),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			mustAddress("0x1"),
			mustAddress("0x2"),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			mustAddress("0x2"),
			mustAddress("0x1"),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})

	t.Run("Bytes", func(t *testing.T) {
		actual, err := cmp(
			cadence.NewBytes([]byte{0x1}),
			cadence.NewBytes([]byte{0x1}),
		)
		require.NoError(t, err)
		assert.Equal(t, 0, actual)

		actual, err = cmp(
			cadence.NewBytes([]byte{0x1}),
			cadence.NewBytes([]byte{0x2}),
		)
		require.NoError(t, err)
		assert.Equal(t, -1, actual)

		actual, err = cmp(
			cadence.NewBytes([]byte{0x2}),
			cadence.NewBytes([]byte{0x1}),
		)
		require.NoError(t, err)
		assert.Equal(t, 1, actual)
	})
}

func valueFactory(t *testing.T, g *fixtures.GeneratorSuite, cadenceType cadence.Type) cadence.Value {
	switch cadenceType {
	case cadence.IntType:
		return cadence.NewInt(g.Random().Int())
	case cadence.Int8Type:
		return cadence.NewInt8(int8(g.Random().Int()))
	case cadence.Int16Type:
		return cadence.NewInt16(int16(g.Random().Int()))
	case cadence.Int32Type:
		return cadence.NewInt32(int32(g.Random().Int()))
	case cadence.Int64Type:
		return cadence.NewInt64(int64(g.Random().Int()))
	case cadence.Int128Type:
		return cadence.NewInt128(g.Random().Int())
	case cadence.Int256Type:
		return cadence.NewInt256(g.Random().Int())
	case cadence.UIntType:
		return cadence.NewUInt(uint(g.Random().Uint64()))
	case cadence.UInt8Type:
		return cadence.NewUInt8(uint8(g.Random().Uint64()))
	case cadence.UInt16Type:
		return cadence.NewUInt16(uint16(g.Random().Uint64()))
	case cadence.UInt32Type:
		return cadence.NewUInt32(uint32(g.Random().Uint64()))
	case cadence.UInt64Type:
		return cadence.NewUInt64(g.Random().Uint64())
	case cadence.UInt128Type:
		return cadence.NewUInt128(uint(g.Random().Uint64()))
	case cadence.UInt256Type:
		return cadence.NewUInt256(uint(g.Random().Uint64()))
	case cadence.Word8Type:
		return cadence.NewWord8(uint8(g.Random().Uint64()))
	case cadence.Word16Type:
		return cadence.NewWord16(uint16(g.Random().Uint64()))
	case cadence.Word32Type:
		return cadence.NewWord32(uint32(g.Random().Uint64()))
	case cadence.Word64Type:
		return cadence.NewWord64(g.Random().Uint64())
	case cadence.Word128Type:
		return cadence.NewWord128(uint(g.Random().Uint64()))
	case cadence.Word256Type:
		return cadence.NewWord256(uint(g.Random().Uint64()))
	case cadence.Fix64Type:
		v, err := cadence.NewFix64(fmt.Sprintf("%.2f", g.Random().Float64()))
		require.NoError(t, err)
		return v
	case cadence.Fix128Type:
		v, err := cadence.NewFix128(fixedPoint.NewFix128(0, g.Random().Uint64()))
		require.NoError(t, err)
		return v
	case cadence.UFix64Type:
		v, err := cadence.NewUFix64(fmt.Sprintf("%.2f", g.Random().Float64()))
		require.NoError(t, err)
		return v
	case cadence.UFix128Type:
		v, err := cadence.NewUFix128(fixedPoint.NewUFix128(0, g.Random().Uint64()))
		require.NoError(t, err)
		return v
	case cadence.BoolType:
		return cadence.NewBool(g.Random().Bool())
	case cadence.StringType:
		v, err := cadence.NewString(g.Random().RandomString(16))
		require.NoError(t, err)
		return v
	case cadence.TheBytesType:
		return cadence.NewBytes(g.Random().RandomBytes(16))
	case cadence.CharacterType:
		v, err := cadence.NewCharacter(g.Random().RandomString(1))
		require.NoError(t, err)
		return v
	case cadence.AddressType:
		data := g.Random().RandomBytes(cadencecommon.AddressLength)
		address, err := cadencecommon.BytesToAddress(data)
		require.NoError(t, err)
		return cadence.NewAddress(address)
	case cadence.StoragePathType:
		v, err := cadence.NewPath(cadencecommon.PathDomainStorage, g.Random().RandomString(16))
		require.NoError(t, err)
		return v
	case cadence.PublicPathType:
		v, err := cadence.NewPath(cadencecommon.PathDomainPublic, g.Random().RandomString(16))
		require.NoError(t, err)
		return v
	default:
		t.Fatalf("unsupported type: %T", cadenceType)
		return nil
	}
}
