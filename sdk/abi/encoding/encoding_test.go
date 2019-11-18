package encoding_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type encodeTest struct {
	typ types.Type
	val values.Value
}

func TestEncodeVoid(t *testing.T) {
	testEncode(t, types.Void{}, values.Void{})
}

func TestEncodeString(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.String{},
			values.String(""),
		},
		{
			types.String{},
			values.String("abcdefg"),
		},
	})
}

func TestEncodeBool(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Bool{},
			values.Bool(true),
		},
		{
			types.Bool{},
			values.Bool(false),
		},
	})
}

func TestEncodeBytes(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Bytes{},
			values.Bytes{},
		},
		{
			types.Bytes{},
			values.Bytes{1, 2, 3, 4, 5},
		},
	})
}

func TestEncodeAddress(t *testing.T) {
	testEncode(t, types.Address{}, values.BytesToAddress([]byte{1, 2, 3, 4, 5}))
}

func TestEncodeInt(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Int{},
			values.Int(-42),
		},
		{
			types.Int{},
			values.Int(0),
		},
		{
			types.Int{},
			values.Int(42),
		},
	})
}

func TestEncodeInt8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Int8{},
			values.Int8(math.MinInt8),
		},
		{
			types.Int8{},
			values.Int8(0),
		},
		{
			types.Int8{},
			values.Int8(math.MaxInt8),
		},
	})
}

func TestEncodeInt16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Int16{},
			values.Int16(math.MinInt16),
		},
		{
			types.Int16{},
			values.Int16(0),
		},
		{
			types.Int16{},
			values.Int16(math.MaxInt16),
		},
	})
}

func TestEncodeInt32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Int32{},
			values.Int32(math.MinInt32),
		},
		{
			types.Int32{},
			values.Int32(0),
		},
		{
			types.Int32{},
			values.Int32(math.MaxInt32),
		},
	})
}

func TestEncodeInt64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Int64{},
			values.Int64(math.MinInt64),
		},
		{
			types.Int64{},
			values.Int64(0),
		},
		{
			types.Int64{},
			values.Int64(math.MaxInt64),
		},
	})
}

func TestEncodeUint8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Uint8{},
			values.Uint8(0),
		},
		{
			types.Uint8{},
			values.Uint8(math.MaxUint8),
		},
	})
}

func TestEncodeUint16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Uint16{},
			values.Uint16(0),
		},
		{
			types.Uint16{},
			values.Uint16(math.MaxUint8),
		},
	})
}

func TestEncodeUint32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Uint32{},
			values.Uint32(0),
		},
		{
			types.Uint32{},
			values.Uint32(math.MaxUint32),
		},
	})
}

func TestEncodeUint64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.Uint64{},
			values.Uint64(0),
		},
		{
			types.Uint64{},
			values.Uint64(math.MaxUint64),
		},
	})
}

func TestEncodeVariableSizedArray(t *testing.T) {
	emptyArray := encodeTest{
		types.VariableSizedArray{
			ElementType: types.Int{},
		},
		values.VariableSizedArray{},
	}

	intArray := encodeTest{
		types.VariableSizedArray{
			ElementType: types.Int{},
		},
		values.VariableSizedArray{
			values.Int(1),
			values.Int(2),
			values.Int(3),
		},
	}

	compositeArray := encodeTest{
		types.VariableSizedArray{
			ElementType: types.Composite{
				FieldTypes: []types.Type{
					types.String{},
					types.Int{},
				},
			},
		},
		values.VariableSizedArray{
			values.Composite{
				Fields: []values.Value{
					values.String("a"),
					values.Int(1),
				},
			},
			values.Composite{
				Fields: []values.Value{
					values.String("b"),
					values.Int(1),
				},
			},
			values.Composite{
				Fields: []values.Value{
					values.String("c"),
					values.Int(1),
				},
			},
		},
	}

	testAllEncode(t, []encodeTest{
		emptyArray,
		intArray,
		compositeArray,
	})
}

func TestEncodeConstantSizedArray(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			types.ConstantSizedArray{
				Size:        0,
				ElementType: types.Int{},
			},
			values.ConstantSizedArray{},
		},
		{
			types.ConstantSizedArray{
				Size:        3,
				ElementType: types.Int{},
			},
			values.ConstantSizedArray{
				values.Int(1),
				values.Int(2),
				values.Int(3),
			},
		},
	})
}

func TestEncodeDictionary(t *testing.T) {
	simpleDict := encodeTest{
		types.Dictionary{
			KeyType:     types.String{},
			ElementType: types.Int{},
		},
		values.Dictionary{
			values.KeyValuePair{
				values.String("a"),
				values.Int(1),
			},
			values.KeyValuePair{
				values.String("b"),
				values.Int(2),
			},
			values.KeyValuePair{
				values.String("c"),
				values.Int(3),
			},
		},
	}

	nestedDict := encodeTest{
		types.Dictionary{
			KeyType: types.String{},
			ElementType: types.Dictionary{
				KeyType:     types.String{},
				ElementType: types.Int{},
			},
		},
		values.Dictionary{
			values.KeyValuePair{
				values.String("a"),
				values.Dictionary{
					values.KeyValuePair{
						values.String("1"),
						values.Int(1),
					},
				},
			},
			values.KeyValuePair{
				values.String("b"),
				values.Dictionary{
					values.KeyValuePair{
						values.String("2"),
						values.Int(2),
					},
				},
			},
			values.KeyValuePair{
				values.String("c"),
				values.Dictionary{
					values.KeyValuePair{
						values.String("3"),
						values.Int(3),
					},
				},
			},
		},
	}

	compositeDict := encodeTest{
		types.Dictionary{
			KeyType: types.String{},
			ElementType: types.Composite{
				FieldTypes: []types.Type{
					types.String{},
					types.Int{},
				},
			},
		},
		values.Dictionary{
			values.KeyValuePair{
				values.String("a"),
				values.Composite{
					Fields: []values.Value{
						values.String("a"),
						values.Int(1),
					},
				},
			},
			values.KeyValuePair{
				values.String("b"),
				values.Composite{
					Fields: []values.Value{
						values.String("b"),
						values.Int(2),
					},
				},
			},
			values.KeyValuePair{
				values.String("c"),
				values.Composite{
					Fields: []values.Value{
						values.String("c"),
						values.Int(3),
					},
				},
			},
		},
	}

	testAllEncode(t, []encodeTest{
		simpleDict,
		nestedDict,
		compositeDict,
	})
}

func TestEncodeComposite(t *testing.T) {
	simpleComp := encodeTest{
		types.Composite{
			FieldTypes: []types.Type{
				types.String{},
				types.String{},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.String("foo"),
				values.String("bar"),
			},
		},
	}

	multiTypeComp := encodeTest{
		types.Composite{
			FieldTypes: []types.Type{
				types.String{},
				types.Int{},
				types.Bool{},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.String("foo"),
				values.Int(42),
				values.Bool(true),
			},
		},
	}

	arrayComp := encodeTest{
		types.Composite{
			FieldTypes: []types.Type{
				types.VariableSizedArray{
					ElementType: types.Int{},
				},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.VariableSizedArray{
					values.Int(1),
					values.Int(2),
					values.Int(3),
					values.Int(4),
					values.Int(5),
				},
			},
		},
	}

	nestedComp := encodeTest{
		types.Composite{
			FieldTypes: []types.Type{
				types.String{},
				types.Composite{
					FieldTypes: []types.Type{
						types.Int{},
					},
				},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.String("foo"),
				values.Composite{
					Fields: []values.Value{
						values.Int(42),
					},
				},
			},
		},
	}

	testAllEncode(t, []encodeTest{
		simpleComp,
		multiTypeComp,
		arrayComp,
		nestedComp,
	})
}

func TestEncodeEvent(t *testing.T) {
	simpleEvent := encodeTest{
		types.Event{
			Identifier: "Test",
			FieldTypes: []types.EventField{
				{
					Identifier: "x",
					Type:       types.Int{},
				},
				{
					Identifier: "y",
					Type:       types.String{},
				},
			},
		},
		values.Event{
			Identifier: "Test",
			Fields: []values.Value{
				values.Int(1),
				values.String("foo"),
			},
		},
	}

	compositeEvent := encodeTest{
		types.Event{
			Identifier: "Test",
			FieldTypes: []types.EventField{
				{
					Identifier: "x",
					Type:       types.String{},
				},
				{
					Identifier: "y",
					Type: types.Composite{
						FieldTypes: []types.Type{
							types.String{},
							types.Int{},
						},
					},
				},
			},
		},
		values.Event{
			Identifier: "Test",
			Fields: []values.Value{
				values.String("foo"),
				values.Composite{
					Fields: []values.Value{
						values.String("bar"),
						values.Int(42),
					},
				},
			},
		},
	}

	testAllEncode(t, []encodeTest{
		simpleEvent,
		compositeEvent,
	})
}

func testAllEncode(t *testing.T, tests []encodeTest) {
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			testEncode(t, test.typ, test.val)
		})
	}
}

const numTrials = 50

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

	val2, err := encoding.Decode(typ, b1)
	require.NoError(t, err)

	assert.Equal(t, val, val2)
}
