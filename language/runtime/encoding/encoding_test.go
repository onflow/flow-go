package encoding_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language"
	"github.com/dapperlabs/flow-go/language/runtime/encoding"
)

type encodeTest struct {
	name string
	typ  language.Type
	val  language.Value
}

func TestEncodeVoid(t *testing.T) {
	testEncode(t, language.VoidType{}, language.Void{})
}

func TestEncodeString(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyString",
			language.StringType{},
			language.NewString(""),
		},
		{
			"SimpleString",
			language.StringType{},
			language.NewString("abcdefg"),
		},
	}...)
}

func TestEncodeOptional(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Nil",
			language.OptionalType{Type: nil},
			language.NewOptional(nil),
		},
		{
			"SomeString",
			language.OptionalType{Type: language.StringType{}},
			language.NewOptional(language.NewString("abcdefg")),
		},
	}...)
}

func TestEncodeBool(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"True",
			language.BoolType{},
			language.NewBool(true),
		},
		{
			"False",
			language.BoolType{},
			language.NewBool(false),
		},
	}...)
}

func TestEncodeBytes(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyBytes",
			language.BytesType{},
			language.NewBytes([]byte{}),
		},
		{
			"SimpleBytes",
			language.BytesType{},
			language.NewBytes([]byte{1, 2, 3, 4, 5}),
		},
	}...)
}

func TestEncodeAddress(t *testing.T) {
	testEncode(t, language.AddressType{}, language.NewAddress([20]byte{1, 2, 3, 4, 5}))
}

func TestEncodeInt(t *testing.T) {
	x := big.NewInt(0).SetUint64(math.MaxUint64)
	x = x.Mul(x, big.NewInt(2))

	largerThanMaxUint64 := encodeTest{
		"LargerThanMaxUint64",
		language.IntType{},
		language.NewIntFromBig(x),
	}

	testAllEncode(t, []encodeTest{
		{
			"Negative",
			language.IntType{},
			language.NewInt(-42),
		},
		{
			"Zero",
			language.IntType{},
			language.NewInt(0),
		},
		{
			"Positive",
			language.IntType{},
			language.NewInt(42),
		},
		largerThanMaxUint64,
	}...)
}

func TestEncodeInt8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			language.Int8Type{},
			language.NewInt8(math.MinInt8),
		},
		{
			"Zero",
			language.Int8Type{},
			language.NewInt8(0),
		},
		{
			"Max",
			language.Int8Type{},
			language.NewInt8(math.MaxInt8),
		},
	}...)
}

func TestEncodeInt16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			language.Int16Type{},
			language.NewInt16(math.MinInt16),
		},
		{
			"Zero",
			language.Int16Type{},
			language.NewInt16(0),
		},
		{
			"Max",
			language.Int16Type{},
			language.NewInt16(math.MaxInt16),
		},
	}...)
}

func TestEncodeInt32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			language.Int32Type{},
			language.NewInt32(math.MinInt32),
		},
		{
			"Zero",
			language.Int32Type{},
			language.NewInt32(0),
		},
		{
			"Max",
			language.Int32Type{},
			language.NewInt32(math.MaxInt32),
		},
	}...)
}

func TestEncodeInt64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			language.Int64Type{},
			language.NewInt64(math.MinInt64),
		},
		{
			"Zero",
			language.Int64Type{},
			language.NewInt64(0),
		},
		{
			"Max",
			language.Int64Type{},
			language.NewInt64(math.MaxInt64),
		},
	}...)
}

func TestEncodeUint8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			language.UInt8Type{},
			language.NewUInt8(0),
		},
		{
			"Max",
			language.UInt8Type{},
			language.NewUInt8(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			language.UInt16Type{},
			language.NewUInt16(0),
		},
		{
			"Max",
			language.UInt16Type{},
			language.NewUInt16(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			language.UInt32Type{},
			language.NewUInt32(0),
		},
		{
			"Max",
			language.UInt32Type{},
			language.NewUInt32(math.MaxUint32),
		},
	}...)
}

func TestEncodeUint64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			language.UInt64Type{},
			language.NewUInt64(0),
		},
		{
			"Max",
			language.UInt64Type{},
			language.NewUInt64(math.MaxUint64),
		},
	}...)
}

func TestEncodeVariableSizedArray(t *testing.T) {
	emptyArray := encodeTest{
		"EmptyArray",
		language.VariableSizedArrayType{
			ElementType: language.IntType{},
		},
		language.NewVariableSizedArray([]language.Value{}),
	}

	intArray := encodeTest{
		"IntArray",
		language.VariableSizedArrayType{
			ElementType: language.IntType{},
		},
		language.NewVariableSizedArray([]language.Value{
			language.NewInt(1),
			language.NewInt(2),
			language.NewInt(3),
		}),
	}

	compositeArray := encodeTest{
		"CompositeArray",
		language.VariableSizedArrayType{
			ElementType: language.CompositeType{
				Fields: []language.Field{
					{
						Identifier: "a",
						Type:       language.StringType{},
					},
					{
						Identifier: "b",
						Type:       language.IntType{},
					},
				},
			},
		},
		language.NewVariableSizedArray([]language.Value{
			language.NewComposite([]language.Value{
				language.NewString("a"),
				language.NewInt(1),
			}),
			language.NewComposite([]language.Value{
				language.NewString("b"),
				language.NewInt(1),
			}),
			language.NewComposite([]language.Value{
				language.NewString("c"),
				language.NewInt(1),
			}),
		}),
	}

	testAllEncode(t,
		emptyArray,
		intArray,
		compositeArray,
	)
}

func TestEncodeConstantSizedArray(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyArray",
			language.ConstantSizedArrayType{
				Size:        0,
				ElementType: language.IntType{},
			},
			language.NewConstantSizedArray([]language.Value{}),
		},
		{
			"IntArray",
			language.ConstantSizedArrayType{
				Size:        3,
				ElementType: language.IntType{},
			},
			language.NewConstantSizedArray([]language.Value{
				language.NewInt(1),
				language.NewInt(2),
				language.NewInt(3),
			}),
		},
	}...)
}

func TestEncodeDictionary(t *testing.T) {
	simpleDict := encodeTest{
		"SimpleDict",
		language.DictionaryType{
			KeyType:     language.StringType{},
			ElementType: language.IntType{},
		},
		language.NewDictionary([]language.KeyValuePair{
			{
				language.NewString("a"),
				language.NewInt(1),
			},
			{
				language.NewString("b"),
				language.NewInt(2),
			},
			{
				language.NewString("c"),
				language.NewInt(3),
			},
		}),
	}

	nestedDict := encodeTest{
		"NestedDict",
		language.DictionaryType{
			KeyType: language.StringType{},
			ElementType: language.DictionaryType{
				KeyType:     language.StringType{},
				ElementType: language.IntType{},
			},
		},
		language.NewDictionary([]language.KeyValuePair{
			{
				language.NewString("a"),
				language.NewDictionary([]language.KeyValuePair{
					{
						language.NewString("1"),
						language.NewInt(1),
					},
				}),
			},
			{
				language.NewString("b"),
				language.NewDictionary([]language.KeyValuePair{
					{
						language.NewString("2"),
						language.NewInt(2),
					},
				}),
			},
			{
				language.NewString("c"),
				language.NewDictionary([]language.KeyValuePair{
					{
						language.NewString("3"),
						language.NewInt(3),
					},
				}),
			},
		}),
	}

	compositeDict := encodeTest{
		"CompositeDict",
		language.DictionaryType{
			KeyType: language.StringType{},
			ElementType: language.CompositeType{
				Fields: []language.Field{
					{
						Identifier: "a",
						Type:       language.StringType{},
					},
					{
						Identifier: "b",
						Type:       language.IntType{},
					},
				},
			},
		},
		language.NewDictionary([]language.KeyValuePair{
			{
				language.NewString("a"),
				language.NewComposite([]language.Value{
					language.NewString("a"),
					language.NewInt(1),
				}),
			},
			{
				language.NewString("b"),
				language.NewComposite([]language.Value{
					language.NewString("b"),
					language.NewInt(2),
				}),
			},
			{
				language.NewString("c"),
				language.NewComposite([]language.Value{
					language.NewString("c"),
					language.NewInt(3),
				}),
			},
		}),
	}

	testAllEncode(t,
		simpleDict,
		nestedDict,
		compositeDict,
	)
}

func TestEncodeComposite(t *testing.T) {
	simpleComp := encodeTest{
		"SimpleComposite",
		language.CompositeType{
			Fields: []language.Field{
				{
					Identifier: "a",
					Type:       language.StringType{},
				},
				{
					Identifier: "b",
					Type:       language.StringType{},
				},
			},
		},
		language.NewComposite([]language.Value{
			language.NewString("foo"),
			language.NewString("bar"),
		}),
	}

	multiTypeComp := encodeTest{
		"MultiTypeComposite",
		language.CompositeType{
			Fields: []language.Field{
				{
					Identifier: "a",
					Type:       language.StringType{},
				},
				{
					Identifier: "b",
					Type:       language.IntType{},
				},
				{
					Identifier: "c",
					Type:       language.BoolType{},
				},
			},
		},
		language.NewComposite([]language.Value{
			language.NewString("foo"),
			language.NewInt(42),
			language.NewBool(true),
		}),
	}

	arrayComp := encodeTest{
		"ArrayComposite",
		language.CompositeType{
			Fields: []language.Field{
				{
					Identifier: "a",
					Type: language.VariableSizedArrayType{
						ElementType: language.IntType{},
					},
				},
			},
		},
		language.NewComposite([]language.Value{
			language.NewVariableSizedArray([]language.Value{
				language.NewInt(1),
				language.NewInt(2),
				language.NewInt(3),
				language.NewInt(4),
				language.NewInt(5),
			}),
		}),
	}

	nestedComp := encodeTest{
		"NestedComposite",
		language.CompositeType{
			Fields: []language.Field{
				{
					Identifier: "a",
					Type:       language.StringType{},
				},
				{
					Identifier: "b",
					Type: language.CompositeType{
						Fields: []language.Field{
							{
								Identifier: "c",
								Type:       language.IntType{},
							},
						},
					},
				},
			},
		},
		language.NewComposite([]language.Value{
			language.NewString("foo"),
			language.NewComposite([]language.Value{
				language.NewInt(42),
			}),
		}),
	}

	testAllEncode(t,
		simpleComp,
		multiTypeComp,
		arrayComp,
		nestedComp,
	)
}

func TestEncodeEvent(t *testing.T) {
	simpleEvent := encodeTest{
		"SimpleEvent",
		language.EventType{
			CompositeType: language.CompositeType{
				Fields: []language.Field{
					{
						Identifier: "a",
						Type:       language.IntType{},
					},
					{
						Identifier: "b",
						Type:       language.StringType{},
					},
				},
			},
		},
		language.NewComposite(
			[]language.Value{
				language.NewInt(1),
				language.NewString("foo"),
			},
		),
	}

	compositeEvent := encodeTest{
		"CompositeEvent",
		language.EventType{
			CompositeType: language.CompositeType{
				Fields: []language.Field{
					{
						Identifier: "a",
						Type:       language.StringType{},
					},
					{
						Identifier: "b",
						Type: language.CompositeType{
							Fields: []language.Field{
								{
									Identifier: "c",
									Type:       language.StringType{},
								},
								{
									Identifier: "d",
									Type:       language.IntType{},
								},
							},
						},
					},
				},
			},
		},
		language.NewComposite(
			[]language.Value{
				language.NewString("foo"),
				language.NewComposite(
					[]language.Value{
						language.NewString("bar"),
						language.NewInt(42),
					},
				),
			},
		),
	}

	testAllEncode(t, simpleEvent, compositeEvent)
}

func testAllEncode(t *testing.T, tests ...encodeTest) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testEncode(t, test.typ, test.val)
		})
	}
}

const numTrials = 250

func testEncode(t *testing.T, typ language.Type, val language.Value) {
	b1, err := encoding.Encode(val)
	require.NoError(t, err)

	t.Logf("Encoded value: %x", b1)

	// encoding should be deterministic, repeat to confirm
	for i := 0; i < numTrials; i++ {
		b2, err := encoding.Encode(val)
		require.NoError(t, err)
		assert.Equal(t, b1, b2)
	}

	decodedVal, err := encoding.Decode(typ, b1)
	require.NoError(t, err)

	assert.Equal(t, val, decodedVal)
}
