package interpreter

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/sema"
	"example.com/cadence-initial/runtime/tests/utils"
)

type encodeDecodeTest struct {
	value        Value
	encoded      []byte
	invalid      bool
	deferred     bool
	deferrals    *EncodingDeferrals
	decodedValue Value
}

var testOwner = common.BytesToAddress([]byte{0x42})

func testEncodeDecode(t *testing.T, test encodeDecodeTest) {

	var encoded []byte
	var deferrals *EncodingDeferrals
	if test.value != nil {
		test.value.SetOwner(&testOwner)

		var err error
		encoded, deferrals, err = EncodeValue(test.value, nil, test.deferred)
		require.NoError(t, err)

		if test.encoded != nil {
			utils.AssertEqualWithDiff(t, test.encoded, encoded)
		}
	} else {
		encoded = test.encoded
	}

	decoded, err := DecodeValue(encoded, &testOwner, nil)
	if test.invalid {
		require.Error(t, err)
	} else {
		require.NoError(t, err)

		if !test.deferred || (test.deferred && test.decodedValue != nil) {
			expectedValue := test.value
			if test.decodedValue != nil {
				test.decodedValue.SetOwner(&testOwner)
				expectedValue = test.decodedValue
			}
			utils.AssertEqualWithDiff(t, expectedValue, decoded)
		}
	}

	if test.value != nil {
		if test.deferred {
			utils.AssertEqualWithDiff(t, test.deferrals, deferrals)
		} else {
			require.Empty(t, deferrals.Values)
			require.Empty(t, deferrals.Moves)
		}
	}
}

func TestEncodeDecodeNilValue(t *testing.T) {

	t.Parallel()

	testEncodeDecode(t,
		encodeDecodeTest{
			value: NilValue{},
			encoded: []byte{
				// null
				0xf6,
			},
		},
	)
}

func TestEncodeDecodeVoidValue(t *testing.T) {

	t.Parallel()

	testEncodeDecode(t,
		encodeDecodeTest{
			value: VoidValue{},
			encoded: []byte{
				// tag
				0xd8, cborTagVoidValue,
				// null
				0xf6,
			},
		},
	)
}

func TestEncodeDecodeBool(t *testing.T) {

	t.Parallel()

	t.Run("false", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: BoolValue(false),
				encoded: []byte{
					// false
					0xf4,
				},
			},
		)
	})

	t.Run("true", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: BoolValue(true),
				encoded: []byte{
					// true
					0xf5,
				},
			},
		)
	})
}

func TestEncodeDecodeString(t *testing.T) {

	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		expected := NewStringValue("")
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					//  UTF-8 string, 0 bytes follow
					0x60,
				},
			})
	})

	t.Run("non-empty", func(t *testing.T) {
		expected := NewStringValue("foo")
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// UTF-8 string, 3 bytes follow
					0x63,
					// f, o, o
					0x66, 0x6f, 0x6f,
				},
			},
		)
	})
}

func TestEncodeDecodeArray(t *testing.T) {

	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		expected := NewArrayValueUnownedNonCopying()
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// array, 0 items follow
					0x80,
				},
			})
	})

	t.Run("string and bool", func(t *testing.T) {
		expectedString := NewStringValue("test")
		expectedString.modified = false

		expected := NewArrayValueUnownedNonCopying(
			expectedString,
			BoolValue(true),
		)
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// array, 2 items follow
					0x82,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// true
					0xf5,
				},
			},
		)
	})
}

func TestEncodeDecodeDictionary(t *testing.T) {

	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		expected := NewDictionaryValueUnownedNonCopying()
		expected.modified = false
		expected.Keys.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// tag
					0xd8, cborTagDictionaryValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x00,
					// array, 0 items follow
					0x80,
					// key 1
					0x01,
					// map, 0 pairs of items follow
					0xa0,
				},
			},
		)
	})

	t.Run("non-empty", func(t *testing.T) {
		key1 := NewStringValue("test")
		value1 := NewArrayValueUnownedNonCopying()

		key2 := BoolValue(true)
		value2 := BoolValue(false)

		key3 := NewStringValue("foo")
		value3 := NewStringValue("bar")

		expected := NewDictionaryValueUnownedNonCopying(
			key1, value1,
			key2, value2,
			key3, value3,
		)

		expected.modified = false
		expected.Keys.modified = false

		key1.modified = false
		value1.modified = false

		key3.modified = false
		value3.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// tag
					0xd8, cborTagDictionaryValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// array, 3 items follow
					0x83,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// true
					0xf5,
					// UTF-8 string, length 3
					0x63,
					// f, o, o
					0x66, 0x6f, 0x6f,
					// key 1
					0x1,
					// map, 3 pairs of items follow
					0xa3,
					// UTF-8 string, length 3
					0x63,
					// f, o, o
					0x66, 0x6f, 0x6f,
					// UTF-8 string, length 3
					0x63,
					// b, a, r
					0x62, 0x61, 0x72,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// array, 0 items follow
					0x80,
					// UTF-8 string, length 4
					0x64,
					// t, r, u, e
					0x74, 0x72, 0x75, 0x65,
					// false
					0xf4,
				},
			},
		)
	})
}

func TestEncodeDecodeComposite(t *testing.T) {

	t.Parallel()

	t.Run("empty structure, string location", func(t *testing.T) {
		expected := NewCompositeValue(
			utils.TestLocation,
			"S.test.TestStruct",
			common.CompositeKindStructure,
			map[string]Value{},
			nil,
		)
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// tag
					0xd8, cborTagCompositeValue,
					// map, 4 pairs of items follow
					0xa4,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 17
					0x71,
					0x53, 0x2e, 0x74, 0x65,
					0x73, 0x74, 0x2e, 0x54,
					0x65, 0x73, 0x74, 0x53,
					0x74, 0x72, 0x75, 0x63,
					0x74,
					// key 2
					0x2,
					// positive integer 1
					0x1,
					// key 3
					0x3,
					// map, 0 pairs of items follow
					0xa0,
				},
			},
		)
	})

	t.Run("non-empty resource", func(t *testing.T) {
		stringValue := NewStringValue("test")
		stringValue.modified = false

		expected := NewCompositeValue(
			utils.TestLocation,
			"S.test.TestResource",
			common.CompositeKindResource,
			map[string]Value{
				"true":   BoolValue(true),
				"string": stringValue,
			},
			nil,
		)
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// tag
					0xd8, cborTagCompositeValue,
					// map, 4 pairs of items follow
					0xa4,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 19
					0x73,
					0x53, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54,
					0x65, 0x73, 0x74, 0x52, 0x65, 0x73, 0x6f, 0x75,
					0x72, 0x63, 0x65,
					// key 2
					0x2,
					// positive integer 2
					0x2,
					// key 3
					0x3,
					// map, 2 pairs of items follow
					0xa2,
					// UTF-8 string, length 4
					0x64,
					// t, r, u, e
					0x74, 0x72, 0x75, 0x65,
					// true
					0xf5,
					// UTF-8 string, length 6
					0x66,
					// s, t, r, i, n, g
					0x73, 0x74, 0x72, 0x69, 0x6e, 0x67,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
				},
			},
		)
	})

	t.Run("empty, address location", func(t *testing.T) {
		expected := NewCompositeValue(
			ast.AddressLocation{0x1},
			"A.0x1.TestStruct",
			common.CompositeKindStructure,
			map[string]Value{},
			nil,
		)
		expected.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: expected,
				encoded: []byte{
					// tag
					0xd8, cborTagCompositeValue,
					// map, 4 pairs of items follow
					0xa4,
					// key 0
					0x0,
					// tag
					0xd8, cborTagAddressLocation,
					// byte sequence, length 1
					0x41,
					// positive integer 1
					0x1,
					// key 1
					0x1,
					// UTF-8 string, length 16
					0x70,
					0x41, 0x2e, 0x30, 0x78, 0x31, 0x2e, 0x54, 0x65,
					0x73, 0x74, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74,
					// key 2
					0x2,
					// positive integer 1
					0x1,
					// key 3
					0x3,
					// map, 0 pairs of items follow
					0xa0,
				},
			},
		)
	})

	t.Run("empty, address location, address too long", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagCompositeValue,
					// map, 4 pairs of items follow
					0xa4,
					// key 0
					0x0,
					// tag
					0xd8, cborTagAddressLocation,
					// byte sequence, length 22
					0x56,
					// address
					0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
					0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
					0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
					// key 1
					0x1,
					// UTF-8 string, length 16
					0x70,
					0x41, 0x2e, 0x30, 0x78, 0x31, 0x2e, 0x54, 0x65,
					0x73, 0x74, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74,
					// key 2
					0x2,
					// positive integer 1
					0x1,
					// key 3
					0x3,
					// map, 0 pairs of items follow
					0xa0,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeIntValue(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewIntValueFromInt64(0),
				encoded: []byte{
					0xd8, cborTagIntValue,
					// positive bignum
					0xc2,
					// byte string, length 0
					0x40,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewIntValueFromInt64(42),
				encoded: []byte{
					0xd8, cborTagIntValue,
					// positive bignum
					0xc2,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewIntValueFromInt64(-42),
				encoded: []byte{
					0xd8, cborTagIntValue,
					// negative bignum
					0xc3,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("RFC", func(t *testing.T) {
		rfcValue, ok := new(big.Int).SetString("18446744073709551616", 10)
		require.True(t, ok)

		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewIntValueFromBigInt(rfcValue),
				encoded: []byte{
					// tag
					0xd8, cborTagIntValue,
					// positive bignum
					0xc2,
					// byte string, length 9
					0x49,
					0x01, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				},
			},
		)
	})
}

func TestEncodeDecodeInt8Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int8Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int8Value(-42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// negative integer 42
					0x38,
					0x29,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int8Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int8Value(math.MinInt8),
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// negative integer 0x7f
					0x38,
					0x7f,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// negative integer 0xf00
					0x38,
					0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int8Value(math.MaxInt8),
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// positive integer 0x7f00
					0x18,
					0x7f,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt8Value,
					// positive integer 0xff
					0x18,
					0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeInt16Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int16Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int16Value(-42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// negative integer 42
					0x38,
					0x29,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int16Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int16Value(math.MinInt16),
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// negative integer 0x7fff
					0x39,
					0x7f, 0xff,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// negative integer 0xffff
					0x39,
					0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int16Value(math.MaxInt16),
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// positive integer 0x7fff
					0x19,
					0x7f, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt16Value,
					// positive integer 0xffff
					0x19,
					0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeInt32Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int32Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int32Value(-42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// negative integer 42
					0x38,
					0x29,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int32Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int32Value(math.MinInt32),
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// negative integer 0x7fffffff
					0x3a,
					0x7f, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// negative integer 0xffffffff
					0x3a,
					0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int32Value(math.MaxInt32),
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// positive integer 0x7fffffff
					0x1a,
					0x7f, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt32Value,
					// positive integer 0xffffffff
					0x1a,
					0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeInt64Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int64Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int64Value(-42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// negative integer 42
					0x38,
					0x29,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int64Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int64Value(math.MinInt64),
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// negative integer: 0x7fffffffffffffff
					0x3b,
					0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// negative integer 0xffffffffffffffff
					0x3b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Int64Value(math.MaxInt64),
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// positive integer: 0x7fffffffffffffff
					0x1b,
					0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagInt64Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeInt128Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt128ValueFromInt64(0),
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 0
					0x40,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt128ValueFromInt64(42),
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt128ValueFromInt64(-42),
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// negative bignum
					0xc3,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt128ValueFromBigInt(sema.Int128TypeMinIntBig),
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// negative bignum
					0xc3,
					// byte string, length 16
					0x50,
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// negative bignum
					0xc3,
					// byte string, length 16
					0x50,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt128ValueFromBigInt(sema.Int128TypeMaxIntBig),
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 16
					0x50,
					0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 16
					0x50,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("RFC", func(t *testing.T) {
		rfcValue, ok := new(big.Int).SetString("18446744073709551616", 10)
		require.True(t, ok)

		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt128ValueFromBigInt(rfcValue),
				encoded: []byte{
					// tag
					0xd8, cborTagInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 9
					0x49,
					0x01, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				},
			},
		)
	})
}

func TestEncodeDecodeInt256Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt256ValueFromInt64(0),
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 0
					0x40,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt256ValueFromInt64(42),
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt256ValueFromInt64(-42),
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// negative bignum
					0xc3,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt256ValueFromBigInt(sema.Int256TypeMinIntBig),
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// negative bignum
					0xc3,
					// byte string, length 32
					0x58, 0x20,
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// negative bignum
					0xc3,
					// byte string, length 32
					0x58, 0x20,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt256ValueFromBigInt(sema.Int256TypeMaxIntBig),
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 32
					0x58, 0x20,
					0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 32
					0x58, 0x20,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("RFC", func(t *testing.T) {

		rfcValue, ok := new(big.Int).SetString("18446744073709551616", 10)
		require.True(t, ok)

		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewInt256ValueFromBigInt(rfcValue),
				encoded: []byte{
					// tag
					0xd8, cborTagInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 9
					0x49,
					0x01, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				},
			},
		)
	})
}

func TestEncodeDecodeUIntValue(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUIntValueFromUint64(0),
				encoded: []byte{
					0xd8, cborTagUIntValue,
					// positive bignum
					0xc2,
					// byte string, length 0
					0x40,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagUIntValue,
					// negative bignum
					0xc3,
					// byte string, length 1
					0x41,
					0x2a,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUIntValueFromUint64(42),
				encoded: []byte{
					0xd8, cborTagUIntValue,
					// positive bignum
					0xc2,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("RFC", func(t *testing.T) {

		rfcValue, ok := new(big.Int).SetString("18446744073709551616", 10)
		require.True(t, ok)

		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUIntValueFromBigInt(rfcValue),
				encoded: []byte{
					// tag
					0xd8, cborTagUIntValue,
					// positive bignum
					0xc2,
					// byte string, length 9
					0x49,
					0x01, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				},
			},
		)
	})
}

func TestEncodeDecodeUInt8Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt8Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt8Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt8Value,
					// negative integer 42
					0x38,
					0x29,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt8Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt8Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt8Value(math.MaxUint8),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt8Value,
					// positive integer 0xff
					0x18,
					0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt8Value,
					// positive integer 0xffff
					0x19,
					0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeUInt16Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt16Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt16Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt16Value,
					// negative integer 42
					0x38,
					0x29,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt16Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt16Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt16Value(math.MaxUint16),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt16Value,
					// positive integer 0xffff
					0x19,
					0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt16Value,
					// positive integer 0xffffffff
					0x1a,
					0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeUInt32Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt32Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt32Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt32Value,
					// negative integer 42
					0x38,
					0x29,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt32Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt32Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt32Value(math.MaxUint32),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt32Value,
					// positive integer 0xffffffff
					0x1a,
					0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt32Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeUInt64Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt64Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt64Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUInt64Value,
					// negative integer 42
					0x38,
					0x29,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt64Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt64Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UInt64Value(math.MaxUint64),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt64Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})
}

func TestEncodeDecodeUInt128Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt128ValueFromUint64(0),
				encoded: []byte{
					0xd8, cborTagUInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 0
					0x40,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt128ValueFromUint64(42),
				encoded: []byte{
					0xd8, cborTagUInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt128ValueFromBigInt(sema.UInt128TypeMaxIntBig),
				encoded: []byte{
					0xd8, cborTagUInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 16
					0x50,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagUInt128Value,
					// negative bignum
					0xc3,
					// byte string, length 1
					0x41,
					0x2a,
				},
				invalid: true,
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagUInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 17
					0x51,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("RFC", func(t *testing.T) {
		rfcValue, ok := new(big.Int).SetString("18446744073709551616", 10)
		require.True(t, ok)

		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt128ValueFromBigInt(rfcValue),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt128Value,
					// positive bignum
					0xc2,
					// byte string, length 9
					0x49,
					0x01, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				},
			},
		)
	})
}

func TestEncodeDecodeUInt256Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt256ValueFromUint64(0),
				encoded: []byte{
					0xd8, cborTagUInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 0
					0x40,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt256ValueFromUint64(42),
				encoded: []byte{
					0xd8, cborTagUInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 1
					0x41,
					0x2a,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagUInt256Value,
					// negative bignum
					0xc3,
					// byte string, length 1
					0x41,
					0x2a,
				},
				invalid: true,
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					0xd8, cborTagUInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 65
					0x58, 0x41,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("RFC", func(t *testing.T) {
		rfcValue, ok := new(big.Int).SetString("18446744073709551616", 10)
		require.True(t, ok)

		testEncodeDecode(t,
			encodeDecodeTest{
				value: NewUInt256ValueFromBigInt(rfcValue),
				encoded: []byte{
					// tag
					0xd8, cborTagUInt256Value,
					// positive bignum
					0xc2,
					// byte string, length 9
					0x49,
					0x01, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				},
			},
		)
	})
}

func TestEncodeDecodeWord8Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word8Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagWord8Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagWord8Value,
					// negative integer 42
					0x38,
					0x29,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word8Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagWord8Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word8Value(math.MaxUint8),
				encoded: []byte{
					// tag
					0xd8, cborTagWord8Value,
					// positive integer 0xff
					0x18,
					0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagWord8Value,
					// positive integer 0xffff
					0x19,
					0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeWord16Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word16Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagWord16Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word16Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagWord16Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word16Value(math.MaxUint16),
				encoded: []byte{
					// tag
					0xd8, cborTagWord16Value,
					// positive integer 0xffff
					0x19,
					0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagWord16Value,
					// positive integer 0xffffffff
					0x1a,
					0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeWord32Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word32Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagWord32Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word32Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagWord32Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word32Value(math.MaxUint32),
				encoded: []byte{
					// tag
					0xd8, cborTagWord32Value,
					// positive integer 0xffffffff
					0x1a,
					0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagWord32Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})
}

func TestEncodeDecodeWord64Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word64Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagWord64Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word64Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagWord64Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Word64Value(math.MaxUint64),
				encoded: []byte{
					// tag
					0xd8, cborTagWord64Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})
}

func TestEncodeDecodeSomeValue(t *testing.T) {

	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: &SomeValue{
					Value: NilValue{},
				},
				encoded: []byte{
					// tag
					0xd8, cborTagSomeValue,
					// null
					0xf6,
				},
			},
		)
	})

	t.Run("string", func(t *testing.T) {
		expectedString := NewStringValue("test")
		expectedString.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				value: &SomeValue{
					Value: expectedString,
				},
				encoded: []byte{
					// tag
					0xd8, cborTagSomeValue,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
				},
			},
		)
	})

	t.Run("bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: &SomeValue{
					Value: BoolValue(true),
				},
				encoded: []byte{
					// tag
					0xd8, cborTagSomeValue,
					// true
					0xf5,
				},
			},
		)
	})
}

func TestEncodeDecodeFix64Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Fix64Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Fix64Value(-42),
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// negative integer 42
					0x38,
					0x29,
				},
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Fix64Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Fix64Value(math.MinInt64),
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// negative integer: 0x7fffffffffffffff
					0x3b,
					0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run("<min", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// negative integer 0xffffffffffffffff
					0x3b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: Fix64Value(math.MaxInt64),
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// positive integer: 0x7fffffffffffffff
					0x1b,
					0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})

	t.Run(">max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagFix64Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
				invalid: true,
			},
		)
	})

}

func TestEncodeDecodeUFix64Value(t *testing.T) {

	t.Parallel()

	t.Run("zero", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UFix64Value(0),
				encoded: []byte{
					// tag
					0xd8, cborTagUFix64Value,
					// integer 0
					0x0,
				},
			},
		)
	})

	t.Run("negative", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagUFix64Value,
					// negative integer 42
					0x38,
					0x29,
				},
				invalid: true,
			},
		)
	})

	t.Run("positive", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UFix64Value(42),
				encoded: []byte{
					// tag
					0xd8, cborTagUFix64Value,
					// positive integer 42
					0x18,
					0x2a,
				},
			},
		)
	})

	t.Run("max", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: UFix64Value(math.MaxUint64),
				encoded: []byte{
					// tag
					0xd8, cborTagUFix64Value,
					// positive integer 0xffffffffffffffff
					0x1b,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				},
			},
		)
	})
}

func TestEncodeDecodeStorageReferenceValue(t *testing.T) {

	t.Parallel()

	t.Run("not-authorized", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: &StorageReferenceValue{
					Authorized:           false,
					TargetKey:            "test-key1",
					TargetStorageAddress: common.BytesToAddress([]byte{0x11}),
				},
				encoded: []byte{
					// tag
					0xd8, cborTagStorageReferenceValue,
					// map, 3 pairs of items follow
					0xa3,
					// key 0
					0x0,
					// false
					0xf4,
					// key 1
					0x1,
					// byte sequence, length 1
					0x41,
					// positive integer 0x11
					0x11,
					// key2
					0x2,
					// UTF-8 string, 9 bytes follow
					0x69,
					// t, e, s, t, -, k, e, y, 1
					0x74, 0x65, 0x73, 0x74, 0x2d, 0x6b, 0x65, 0x79, 0x31,
				},
			},
		)
	})

	t.Run("authorized", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: &StorageReferenceValue{
					Authorized:           true,
					TargetKey:            "test-key2",
					TargetStorageAddress: common.BytesToAddress([]byte{0x12}),
				},
				encoded: []byte{
					// tag
					0xd8, cborTagStorageReferenceValue,
					// map, 3 pairs of items follow
					0xa3,
					// key 0
					0x0,
					// true
					0xf5,
					// key 1
					0x1,
					// byte sequence, length 1
					0x41,
					// positive integer 0x12
					0x12,
					// key 2
					0x2,
					// UTF-8 string, 9 bytes follow
					0x69,
					// t, e, s, t, -, k, e, y, 2
					0x74, 0x65, 0x73, 0x74, 0x2d, 0x6b, 0x65, 0x79, 0x32,
				},
			},
		)
	})
}

func TestEncodeDecodeAddressValue(t *testing.T) {

	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: AddressValue{},
				encoded: []byte{
					// tag
					0xd8, cborTagAddressValue,
					// byte sequence, length 0
					0x40,
				},
			},
		)
	})

	t.Run("non-empty", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: AddressValue(common.BytesToAddress([]byte{0x42})),
				encoded: []byte{
					// tag
					0xd8, cborTagAddressValue,
					// byte sequence, length 1
					0x41,
					// address
					0x42,
				},
			},
		)
	})

	t.Run("with leading zeros", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: AddressValue(common.BytesToAddress([]byte{0x0, 0x42})),
				encoded: []byte{
					// tag
					0xd8, cborTagAddressValue,
					// byte sequence, length 1
					0x41,
					// address
					0x42,
				},
			},
		)
	})

	t.Run("with zeros in-between and at and", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: AddressValue(common.BytesToAddress([]byte{0x0, 0x42, 0x0, 0x43, 0x0})),
				encoded: []byte{
					// tag
					0xd8, cborTagAddressValue,
					// byte sequence, length 4
					0x44,
					// address
					0x42, 0x0, 0x43, 0x0,
				},
			},
		)
	})

	t.Run("too long", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				encoded: []byte{
					// tag
					0xd8, cborTagAddressValue,
					// byte sequence, length 22
					0x56,
					// address
					0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
					0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
					0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
				},
				invalid: true,
			},
		)
	})
}

var privatePathValue = PathValue{
	Domain:     common.PathDomainPrivate,
	Identifier: "foo",
}

var publicPathValue = PathValue{
	Domain:     common.PathDomainPublic,
	Identifier: "bar",
}

func TestEncodeDecodePathValue(t *testing.T) {

	t.Parallel()

	t.Run("private", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: privatePathValue,
				encoded: []byte{
					// tag
					0xd8, cborTagPathValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// positive integer 2
					0x2,
					// key 1
					0x1,
					// UTF-8 string, 3 bytes follow
					0x63,
					// f, o, o
					0x66, 0x6f, 0x6f,
				},
			},
		)
	})

	t.Run("public", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: publicPathValue,
				encoded: []byte{
					// tag
					0xd8, cborTagPathValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// positive integer 3
					0x3,
					// key 1
					0x1,
					// UTF-8 string, 3 bytes follow
					0x63,
					// b, a, r
					0x62, 0x61, 0x72,
				},
			},
		)
	})
}

func TestEncodeDecodeCapabilityValue(t *testing.T) {

	t.Parallel()

	t.Run("private path", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: CapabilityValue{
					Address: NewAddressValueFromBytes([]byte{0x2}),
					Path:    privatePathValue,
				},
				encoded: []byte{
					// tag
					0xd8, cborTagCapabilityValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag for address
					0xd8, cborTagAddressValue,
					// byte sequence, length 1
					0x41,
					// address
					0x02,
					// key 1
					0x1,
					// tag for address
					0xd8, cborTagPathValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// positive integer 2
					0x2,
					// key 1
					0x1,
					// UTF-8 string, length 3
					0x63,
					// f, o, o
					0x66, 0x6f, 0x6f,
				},
			},
		)
	})

	t.Run("public path", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: CapabilityValue{
					Address: NewAddressValueFromBytes([]byte{0x3}),
					Path:    publicPathValue,
				},
				encoded: []byte{
					// tag
					0xd8, cborTagCapabilityValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag for address
					0xd8, cborTagAddressValue,
					// byte sequence, length 1
					0x41,
					// address
					0x03,
					// key 1
					0x1,
					// tag for address
					0xd8, cborTagPathValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// positive integer 3
					0x3,
					// key 1
					0x1,
					// UTF-8 string, length 3
					0x63,
					// b, a, r
					0x62, 0x61, 0x72,
				},
			},
		)
	})
}

func TestEncodeDecodeLinkValue(t *testing.T) {

	t.Parallel()

	expectedLinkEncodingPrefix := []byte{
		// tag
		0xd8, cborTagLinkValue,
		// map, 2 pairs of items follow
		0xa2,
		// key 0
		0x0,
		0xd8, cborTagPathValue,
		// map, 2 pairs of items follow
		0xa2,
		// key 0
		0x0,
		// positive integer 3
		0x3,
		// key 1
		0x1,
		// UTF-8 string, length 3
		0x63,
		// b, a, r
		0x62, 0x61, 0x72,
		// key 1
		0x1,
	}

	t.Run("primitive, Bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type:       ConvertSemaToPrimitiveStaticType(&sema.BoolType{}),
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
				),
			},
		)
	})

	t.Run("optional, primitive, bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: OptionalStaticType{
						Type: PrimitiveStaticTypeBool,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagOptionalStaticType,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
				),
			},
		)
	})

	t.Run("composite, struct", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: CompositeStaticType{
						TypeID:   "S.test.SimpleStruct",
						Location: utils.TestLocation,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagCompositeStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 19
					0x73,
					// S.test.SimpleStruct
					0x53, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53,
					0x69, 0x6d, 0x70, 0x6c, 0x65, 0x53, 0x74, 0x72,
					0x75, 0x63, 0x74,
				),
			},
		)
	})

	t.Run("interface, struct", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: InterfaceStaticType{
						TypeID:   "S.test.SimpleInterface",
						Location: utils.TestLocation,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagInterfaceStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 22
					0x76,
					// S.test.SimpleInterface
					0x53, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53,
					0x69, 0x6d, 0x70, 0x6c, 0x65, 0x49, 0x6e, 0x74,
					0x65, 0x72, 0x66, 0x61, 0x63, 0x65,
				),
			},
		)
	})

	t.Run("variable-sized, bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: VariableSizedStaticType{
						Type: PrimitiveStaticTypeBool,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagVariableSizedStaticType,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
				),
			},
		)
	})

	t.Run("constant-sized, bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: ConstantSizedStaticType{
						Type: PrimitiveStaticTypeBool,
						Size: 42,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagConstantSizedStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// positive integer 42
					0x18, 0x2A,
					// key 1
					0x1,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
				),
			},
		)
	})

	t.Run("reference type, authorized, bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: ReferenceStaticType{
						Authorized: true,
						Type:       PrimitiveStaticTypeBool,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagReferenceStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// true
					0xf5,
					// key 1
					0x1,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
				),
			},
		)
	})

	t.Run("reference type, unauthorized, bool", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: ReferenceStaticType{
						Authorized: false,
						Type:       PrimitiveStaticTypeBool,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagReferenceStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// false
					0xf4,
					// key 1
					0x1,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
				),
			},
		)
	})

	t.Run("dictionary, bool, string", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: DictionaryStaticType{
						KeyType:   PrimitiveStaticTypeBool,
						ValueType: PrimitiveStaticTypeString,
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagDictionaryStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x6,
					// key 1
					0x1,
					// tag
					0xd8, cborTagPrimitiveStaticType,
					0x8,
				),
			},
		)
	})

	t.Run("restricted", func(t *testing.T) {
		testEncodeDecode(t,
			encodeDecodeTest{
				value: LinkValue{
					TargetPath: publicPathValue,
					Type: RestrictedStaticType{
						Type: CompositeStaticType{
							TypeID:   "S.test.S",
							Location: utils.TestLocation,
						},
						Restrictions: []InterfaceStaticType{
							{
								TypeID:   "S.test.I1",
								Location: utils.TestLocation,
							},
							{
								TypeID:   "S.test.I2",
								Location: utils.TestLocation,
							},
						},
					},
				},
				encoded: append(expectedLinkEncodingPrefix[:],
					// tag
					0xd8, cborTagRestrictedStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagCompositeStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 8
					0x68,
					// S.test.S
					0x53, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53,
					// key 1
					0x1,
					// array, length 2
					0x82,
					// tag
					0xd8, cborTagInterfaceStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 9
					0x69,
					// S.test.I1
					0x53, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x49, 0x31,
					// tag
					0xd8, cborTagInterfaceStaticType,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// tag
					0xd8, cborTagStringLocation,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// key 1
					0x1,
					// UTF-8 string, length 9
					0x69,
					// S.test.I2
					0x53, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x49, 0x32,
				),
			},
		)
	})
}

func TestEncodeDecodeDictionaryDeferred(t *testing.T) {

	t.Run("resource values", func(t *testing.T) {

		key1 := NewStringValue("test")
		key1.modified = false
		value1 := NewCompositeValue(
			utils.TestLocation,
			"S.test.R",
			common.CompositeKindResource,
			map[string]Value{},
			nil,
		)
		value1.modified = false

		key2 := BoolValue(true)
		value2 := NewCompositeValue(
			utils.TestLocation,
			"S.test.R2",
			common.CompositeKindResource,
			map[string]Value{},
			nil,
		)
		value2.modified = false

		expected := NewDictionaryValueUnownedNonCopying(
			key1, value1,
			key2, value2,
		)
		expected.modified = false
		expected.Keys.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				deferred: true,
				value:    expected,
				encoded: []byte{
					// tag
					0xd8, cborTagDictionaryValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// array, 2 items follow
					0x82,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// true
					0xf5,
					// key 1
					0x1,
					// map, 0 pairs of items follow
					0xa0,
				},
				deferrals: &EncodingDeferrals{
					Values: map[string]Value{
						"v\x1ftest": value1,
						"v\x1ftrue": value2,
					},
				},
				decodedValue: &DictionaryValue{
					Keys:          expected.Keys,
					Entries:       map[string]Value{},
					DeferredOwner: &testOwner,
					DeferredKeys: map[string]string{
						"test": "v\x1ftest",
						"true": "v\x1ftrue",
					},
				},
			},
		)
	})

	t.Run("non-resource values", func(t *testing.T) {

		key1 := NewStringValue("test")
		key1.modified = false
		value1 := NewStringValue("xyz")
		value1.modified = false

		key2 := BoolValue(true)
		value2 := BoolValue(false)

		expected := NewDictionaryValueUnownedNonCopying(
			key1, value1,
			key2, value2,
		)
		expected.modified = false
		expected.Keys.modified = false

		testEncodeDecode(t,
			encodeDecodeTest{
				deferred: true,
				value:    expected,
				encoded: []byte{
					// tag
					0xd8, cborTagDictionaryValue,
					// map, 2 pairs of items follow
					0xa2,
					// key 0
					0x0,
					// array, 2 items follow
					0x82,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// true
					0xf5,
					// key 1
					0x1,
					// map, 2 pairs of items follow
					0xa2,
					// UTF-8 string, length 4
					0x64,
					// t, e, s, t
					0x74, 0x65, 0x73, 0x74,
					// UTF-8 string, length 3
					0x63,
					// x, y, z
					0x78, 0x79, 0x7a,
					// UTF-8 string, length 4
					0x64,
					// t, r, u, e
					0x74, 0x72, 0x75, 0x65,
					// false
					0xf4,
				},
				deferrals: &EncodingDeferrals{Values: map[string]Value{}},
			},
		)
	})
}
