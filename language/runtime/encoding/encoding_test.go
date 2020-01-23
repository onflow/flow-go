package encoding_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/encoding"
	"github.com/dapperlabs/flow-go/language/runtime/types"
	"github.com/dapperlabs/flow-go/language/runtime/values"
)

type encodeTest struct {
	name string
	typ  types.Type
	val  values.Value
}

func TestEncodeVoid(t *testing.T) {
	testEncode(t, types.Void{}, values.Void{})
}

func TestEncodeString(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyString",
			types.String{},
			values.NewString(""),
		},
		{
			"SimpleString",
			types.String{},
			values.NewString("abcdefg"),
		},
	}...)
}

func TestEncodeOptional(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Nil",
			types.Optional{Type: nil},
			values.NewOptional(nil),
		},
		{
			"SomeString",
			types.Optional{Type: types.String{}},
			values.NewOptional(values.NewString("abcdefg")),
		},
	}...)
}

func TestEncodeBool(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"True",
			types.Bool{},
			values.NewBool(true),
		},
		{
			"False",
			types.Bool{},
			values.NewBool(false),
		},
	}...)
}

func TestEncodeBytes(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyBytes",
			types.Bytes{},
			values.NewBytes([]byte{}),
		},
		{
			"SimpleBytes",
			types.Bytes{},
			values.NewBytes([]byte{1, 2, 3, 4, 5}),
		},
	}...)
}

func TestEncodeAddress(t *testing.T) {
	testEncode(t, types.Address{}, values.NewAddress([20]byte{1, 2, 3, 4, 5}))
}

func TestEncodeInt(t *testing.T) {
	x := big.NewInt(0).SetUint64(math.MaxUint64)
	x = x.Mul(x, big.NewInt(2))

	largerThanMaxUint64 := encodeTest{
		"LargerThanMaxUint64",
		types.Int{},
		values.NewIntFromBig(x),
	}

	testAllEncode(t, []encodeTest{
		{
			"Negative",
			types.Int{},
			values.NewInt(-42),
		},
		{
			"Zero",
			types.Int{},
			values.NewInt(0),
		},
		{
			"Positive",
			types.Int{},
			values.NewInt(42),
		},
		largerThanMaxUint64,
	}...)
}

func TestEncodeInt8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int8{},
			values.NewInt8(math.MinInt8),
		},
		{
			"Zero",
			types.Int8{},
			values.NewInt8(0),
		},
		{
			"Max",
			types.Int8{},
			values.NewInt8(math.MaxInt8),
		},
	}...)
}

func TestEncodeInt16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int16{},
			values.NewInt16(math.MinInt16),
		},
		{
			"Zero",
			types.Int16{},
			values.NewInt16(0),
		},
		{
			"Max",
			types.Int16{},
			values.NewInt16(math.MaxInt16),
		},
	}...)
}

func TestEncodeInt32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int32{},
			values.NewInt32(math.MinInt32),
		},
		{
			"Zero",
			types.Int32{},
			values.NewInt32(0),
		},
		{
			"Max",
			types.Int32{},
			values.NewInt32(math.MaxInt32),
		},
	}...)
}

func TestEncodeInt64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int64{},
			values.NewInt64(math.MinInt64),
		},
		{
			"Zero",
			types.Int64{},
			values.NewInt64(0),
		},
		{
			"Max",
			types.Int64{},
			values.NewInt64(math.MaxInt64),
		},
	}...)
}

func TestEncodeUint8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt8{},
			values.NewUInt8(0),
		},
		{
			"Max",
			types.UInt8{},
			values.NewUInt8(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt16{},
			values.NewUInt16(0),
		},
		{
			"Max",
			types.UInt16{},
			values.NewUInt16(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt32{},
			values.NewUInt32(0),
		},
		{
			"Max",
			types.UInt32{},
			values.NewUInt32(math.MaxUint32),
		},
	}...)
}

func TestEncodeUint64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt64{},
			values.NewUInt64(0),
		},
		{
			"Max",
			types.UInt64{},
			values.NewUInt64(math.MaxUint64),
		},
	}...)
}

func TestEncodeVariableSizedArray(t *testing.T) {
	emptyArray := encodeTest{
		"EmptyArray",
		types.VariableSizedArray{
			ElementType: types.Int{},
		},
		values.NewVariableSizedArray([]values.Value{}),
	}

	intArray := encodeTest{
		"IntArray",
		types.VariableSizedArray{
			ElementType: types.Int{},
		},
		values.NewVariableSizedArray([]values.Value{
			values.NewInt(1),
			values.NewInt(2),
			values.NewInt(3),
		}),
	}

	compositeArray := encodeTest{
		"CompositeArray",
		types.VariableSizedArray{
			ElementType: types.Composite{
				Fields: []types.Field{
					{
						Identifier: "a",
						Type:       types.String{},
					},
					{
						Identifier: "b",
						Type:       types.Int{},
					},
				},
			},
		},
		values.NewVariableSizedArray([]values.Value{
			values.NewComposite([]values.Value{
				values.NewString("a"),
				values.NewInt(1),
			}),
			values.NewComposite([]values.Value{
				values.NewString("b"),
				values.NewInt(1),
			}),
			values.NewComposite([]values.Value{
				values.NewString("c"),
				values.NewInt(1),
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
			types.ConstantSizedArray{
				Size:        0,
				ElementType: types.Int{},
			},
			values.NewConstantSizedArray([]values.Value{}),
		},
		{
			"IntArray",
			types.ConstantSizedArray{
				Size:        3,
				ElementType: types.Int{},
			},
			values.NewConstantSizedArray([]values.Value{
				values.NewInt(1),
				values.NewInt(2),
				values.NewInt(3),
			}),
		},
	}...)
}

func TestEncodeDictionary(t *testing.T) {
	simpleDict := encodeTest{
		"SimpleDict",
		types.Dictionary{
			KeyType:     types.String{},
			ElementType: types.Int{},
		},
		values.NewDictionary([]values.KeyValuePair{
			{
				values.NewString("a"),
				values.NewInt(1),
			},
			{
				values.NewString("b"),
				values.NewInt(2),
			},
			{
				values.NewString("c"),
				values.NewInt(3),
			},
		}),
	}

	nestedDict := encodeTest{
		"NestedDict",
		types.Dictionary{
			KeyType: types.String{},
			ElementType: types.Dictionary{
				KeyType:     types.String{},
				ElementType: types.Int{},
			},
		},
		values.NewDictionary([]values.KeyValuePair{
			{
				values.NewString("a"),
				values.NewDictionary([]values.KeyValuePair{
					{
						values.NewString("1"),
						values.NewInt(1),
					},
				}),
			},
			{
				values.NewString("b"),
				values.NewDictionary([]values.KeyValuePair{
					{
						values.NewString("2"),
						values.NewInt(2),
					},
				}),
			},
			{
				values.NewString("c"),
				values.NewDictionary([]values.KeyValuePair{
					{
						values.NewString("3"),
						values.NewInt(3),
					},
				}),
			},
		}),
	}

	compositeDict := encodeTest{
		"CompositeDict",
		types.Dictionary{
			KeyType: types.String{},
			ElementType: types.Composite{
				Fields: []types.Field{
					{
						Identifier: "a",
						Type:       types.String{},
					},
					{
						Identifier: "b",
						Type:       types.Int{},
					},
				},
			},
		},
		values.NewDictionary([]values.KeyValuePair{
			{
				values.NewString("a"),
				values.NewComposite([]values.Value{
					values.NewString("a"),
					values.NewInt(1),
				}),
			},
			{
				values.NewString("b"),
				values.NewComposite([]values.Value{
					values.NewString("b"),
					values.NewInt(2),
				}),
			},
			{
				values.NewString("c"),
				values.NewComposite([]values.Value{
					values.NewString("c"),
					values.NewInt(3),
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
		types.Composite{
			Fields: []types.Field{
				{
					Identifier: "a",
					Type:       types.String{},
				},
				{
					Identifier: "b",
					Type:       types.String{},
				},
			},
		},
		values.NewComposite([]values.Value{
			values.NewString("foo"),
			values.NewString("bar"),
		}),
	}

	multiTypeComp := encodeTest{
		"MultiTypeComposite",
		types.Composite{
			Fields: []types.Field{
				{
					Identifier: "a",
					Type:       types.String{},
				},
				{
					Identifier: "b",
					Type:       types.Int{},
				},
				{
					Identifier: "c",
					Type:       types.Bool{},
				},
			},
		},
		values.NewComposite([]values.Value{
			values.NewString("foo"),
			values.NewInt(42),
			values.NewBool(true),
		}),
	}

	arrayComp := encodeTest{
		"ArrayComposite",
		types.Composite{
			Fields: []types.Field{
				{
					Identifier: "a",
					Type: types.VariableSizedArray{
						ElementType: types.Int{},
					},
				},
			},
		},
		values.NewComposite([]values.Value{
			values.NewVariableSizedArray([]values.Value{
				values.NewInt(1),
				values.NewInt(2),
				values.NewInt(3),
				values.NewInt(4),
				values.NewInt(5),
			}),
		}),
	}

	nestedComp := encodeTest{
		"NestedComposite",
		types.Composite{
			Fields: []types.Field{
				{
					Identifier: "a",
					Type:       types.String{},
				},
				{
					Identifier: "b",
					Type: types.Composite{
						Fields: []types.Field{
							{
								Identifier: "c",
								Type:       types.Int{},
							},
						},
					},
				},
			},
		},
		values.NewComposite([]values.Value{
			values.NewString("foo"),
			values.NewComposite([]values.Value{
				values.NewInt(42),
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
		types.Event{
			Composite: types.Composite{
				Fields: []types.Field{
					{
						Identifier: "a",
						Type:       types.Int{},
					},
					{
						Identifier: "b",
						Type:       types.String{},
					},
				},
			},
		},
		values.NewComposite(
			[]values.Value{
				values.NewInt(1),
				values.NewString("foo"),
			},
		),
	}

	compositeEvent := encodeTest{
		"CompositeEvent",
		types.Event{
			Composite: types.Composite{
				Fields: []types.Field{
					{
						Identifier: "a",
						Type:       types.String{},
					},
					{
						Identifier: "b",
						Type: types.Composite{
							Fields: []types.Field{
								{
									Identifier: "c",
									Type:       types.String{},
								},
								{
									Identifier: "d",
									Type:       types.Int{},
								},
							},
						},
					},
				},
			},
		},
		values.NewComposite(
			[]values.Value{
				values.NewString("foo"),
				values.NewComposite(
					[]values.Value{
						values.NewString("bar"),
						values.NewInt(42),
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

func testEncode(t *testing.T, typ types.Type, val values.Value) {
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
