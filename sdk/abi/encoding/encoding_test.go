package encoding_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
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
			values.String(""),
		},
		{
			"SimpleString",
			types.String{},
			values.String("abcdefg"),
		},
	}...)
}

func TestEncodeBool(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"True",
			types.Bool{},
			values.Bool(true),
		},
		{
			"False",
			types.Bool{},
			values.Bool(false),
		},
	}...)
}

func TestEncodeBytes(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyBytes",
			types.Bytes{},
			values.Bytes{},
		},
		{
			"SimpleBytes",
			types.Bytes{},
			values.Bytes{1, 2, 3, 4, 5},
		},
	}...)
}

func TestEncodeAddress(t *testing.T) {
	testEncode(t, types.Address{}, values.BytesToAddress([]byte{1, 2, 3, 4, 5}))
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
			values.Int8(math.MinInt8),
		},
		{
			"Zero",
			types.Int8{},
			values.Int8(0),
		},
		{
			"Max",
			types.Int8{},
			values.Int8(math.MaxInt8),
		},
	}...)
}

func TestEncodeInt16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int16{},
			values.Int16(math.MinInt16),
		},
		{
			"Zero",
			types.Int16{},
			values.Int16(0),
		},
		{
			"Max",
			types.Int16{},
			values.Int16(math.MaxInt16),
		},
	}...)
}

func TestEncodeInt32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int32{},
			values.Int32(math.MinInt32),
		},
		{
			"Zero",
			types.Int32{},
			values.Int32(0),
		},
		{
			"Max",
			types.Int32{},
			values.Int32(math.MaxInt32),
		},
	}...)
}

func TestEncodeInt64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types.Int64{},
			values.Int64(math.MinInt64),
		},
		{
			"Zero",
			types.Int64{},
			values.Int64(0),
		},
		{
			"Max",
			types.Int64{},
			values.Int64(math.MaxInt64),
		},
	}...)
}

func TestEncodeUint8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt8{},
			values.Uint8(0),
		},
		{
			"Max",
			types.UInt8{},
			values.Uint8(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt16{},
			values.Uint16(0),
		},
		{
			"Max",
			types.UInt16{},
			values.Uint16(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt32{},
			values.Uint32(0),
		},
		{
			"Max",
			types.UInt32{},
			values.Uint32(math.MaxUint32),
		},
	}...)
}

func TestEncodeUint64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types.UInt64{},
			values.Uint64(0),
		},
		{
			"Max",
			types.UInt64{},
			values.Uint64(math.MaxUint64),
		},
	}...)
}

func TestEncodeVariableSizedArray(t *testing.T) {
	emptyArray := encodeTest{
		"EmptyArray",
		types.VariableSizedArray{
			ElementType: types.Int{},
		},
		values.VariableSizedArray{},
	}

	intArray := encodeTest{
		"IntArray",
		types.VariableSizedArray{
			ElementType: types.Int{},
		},
		values.VariableSizedArray{
			values.NewInt(1),
			values.NewInt(2),
			values.NewInt(3),
		},
	}

	compositeArray := encodeTest{
		"CompositeArray",
		types.VariableSizedArray{
			ElementType: types.Composite{
				Fields: map[string]*types.Field{
					"a": {
						Type:       types.String{},
						Identifier: "a",
					},
					"b": {
						Type:       types.Int{},
						Identifier: "b",
					},
				},
			},
		},
		values.VariableSizedArray{
			values.Composite{
				Fields: []values.Value{
					values.String("a"),
					values.NewInt(1),
				},
			},
			values.Composite{
				Fields: []values.Value{
					values.String("b"),
					values.NewInt(1),
				},
			},
			values.Composite{
				Fields: []values.Value{
					values.String("c"),
					values.NewInt(1),
				},
			},
		},
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
			values.ConstantSizedArray{},
		},
		{
			"IntArray",
			types.ConstantSizedArray{
				Size:        3,
				ElementType: types.Int{},
			},
			values.ConstantSizedArray{
				values.NewInt(1),
				values.NewInt(2),
				values.NewInt(3),
			},
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
		values.Dictionary{
			values.KeyValuePair{
				values.String("a"),
				values.NewInt(1),
			},
			values.KeyValuePair{
				values.String("b"),
				values.NewInt(2),
			},
			values.KeyValuePair{
				values.String("c"),
				values.NewInt(3),
			},
		},
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
		values.Dictionary{
			values.KeyValuePair{
				values.String("a"),
				values.Dictionary{
					values.KeyValuePair{
						Key:   values.String("1"),
						Value: values.NewInt(1),
					},
				},
			},
			values.KeyValuePair{
				values.String("b"),
				values.Dictionary{
					values.KeyValuePair{
						Key:   values.String("2"),
						Value: values.NewInt(2),
					},
				},
			},
			values.KeyValuePair{
				Key: values.String("c"),
				Value: values.Dictionary{
					values.KeyValuePair{
						values.String("3"),
						values.NewInt(3),
					},
				},
			},
		},
	}

	compositeDict := encodeTest{
		"CompositeDict",
		types.Dictionary{
			KeyType: types.String{},
			ElementType: types.Composite{
				Fields: map[string]*types.Field{
					"a": {
						Type: types.String{},
					},
					"b": {
						Type: types.Int{},
					},
				},
			},
		},
		values.Dictionary{
			values.KeyValuePair{
				values.String("a"),
				values.Composite{
					Fields: []values.Value{
						values.String("a"),
						values.NewInt(1),
					},
				},
			},
			values.KeyValuePair{
				values.String("b"),
				values.Composite{
					Fields: []values.Value{
						values.String("b"),
						values.NewInt(2),
					},
				},
			},
			values.KeyValuePair{
				values.String("c"),
				values.Composite{
					Fields: []values.Value{
						values.String("c"),
						values.NewInt(3),
					},
				},
			},
		},
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
			Fields: map[string]*types.Field{
				"a": {
					Type: types.String{},
				},
				"b": {
					Type: types.String{},
				},
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
		"MultiTypeComposite",
		types.Composite{
			Fields: map[string]*types.Field{
				"a": {
					Type: types.String{},
				},
				"b": {
					Type: types.Int{},
				},
				"c": {
					Type: types.Bool{},
				},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.String("foo"),
				values.NewInt(42),
				values.Bool(true),
			},
		},
	}

	arrayComp := encodeTest{
		"ArrayComposite",
		types.Composite{
			Fields: map[string]*types.Field{
				"a": {
					Type: types.VariableSizedArray{
						ElementType: types.Int{},
					},
				},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.VariableSizedArray{
					values.NewInt(1),
					values.NewInt(2),
					values.NewInt(3),
					values.NewInt(4),
					values.NewInt(5),
				},
			},
		},
	}

	nestedComp := encodeTest{
		"NestedComposite",
		types.Composite{
			Fields: map[string]*types.Field{

				"a": {
					Type: types.String{},
				},
				"b": {
					Type: types.Composite{
						Fields: map[string]*types.Field{
							"a": {
								Type: types.Int{},
							},
						},
					},
				},
			},
		},
		values.Composite{
			Fields: []values.Value{
				values.String("foo"),
				values.Composite{
					Fields: []values.Value{
						values.NewInt(42),
					},
				},
			},
		},
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
			Identifier: "Test",
			Fields: []*types.Parameter{
				{
					Field: types.Field{
						Identifier: "x",
						Type:       types.Int{},
					},
				},
				{
					Field: types.Field{
						Identifier: "y",
						Type:       types.String{},
					},
				},
			},
		},
		values.Event{
			Identifier: "Test",
			Fields: []values.Value{
				values.NewInt(1),
				values.String("foo"),
			},
		},
	}

	compositeEvent := encodeTest{
		"CompositeEvent",
		types.Event{
			Identifier: "Test",
			Fields: []*types.Parameter{
				{
					Field: types.Field{
						Identifier: "x",
						Type:       types.String{},
					},
				},
				{
					Field: types.Field{
						Identifier: "y",
						Type: types.Composite{
							Fields: map[string]*types.Field{
								"a": {
									Type: types.String{},
								},
								"b": {
									Type: types.Int{},
								},
							},
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
						values.NewInt(42),
					},
				},
			},
		},
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
