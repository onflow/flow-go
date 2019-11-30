package values_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	types2 "github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type encodeTest struct {
	name string
	typ  types2.Type
	val  values.Value
}

func TestEncodeVoid(t *testing.T) {
	testEncode(t, types2.Void{}, values.Void{})
}

func TestEncodeString(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyString",
			types2.String{},
			values.String(""),
		},
		{
			"SimpleString",
			types2.String{},
			values.String("abcdefg"),
		},
	}...)
}

func TestEncodeBool(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"True",
			types2.Bool{},
			values.Bool(true),
		},
		{
			"False",
			types2.Bool{},
			values.Bool(false),
		},
	}...)
}

func TestEncodeBytes(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"EmptyBytes",
			types2.Bytes{},
			values.Bytes{},
		},
		{
			"SimpleBytes",
			types2.Bytes{},
			values.Bytes{1, 2, 3, 4, 5},
		},
	}...)
}

func TestEncodeAddress(t *testing.T) {
	testEncode(t, types2.Address{}, values.BytesToAddress([]byte{1, 2, 3, 4, 5}))
}

func TestEncodeInt(t *testing.T) {
	x := big.NewInt(0).SetUint64(math.MaxUint64)
	x = x.Mul(x, big.NewInt(2))

	largerThanMaxUint64 := encodeTest{
		"LargerThanMaxUint64",
		types2.Int{},
		values.NewIntFromBig(x),
	}

	testAllEncode(t, []encodeTest{
		{
			"Negative",
			types2.Int{},
			values.NewInt(-42),
		},
		{
			"Zero",
			types2.Int{},
			values.NewInt(0),
		},
		{
			"Positive",
			types2.Int{},
			values.NewInt(42),
		},
		largerThanMaxUint64,
	}...)
}

func TestEncodeInt8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types2.Int8{},
			values.Int8(math.MinInt8),
		},
		{
			"Zero",
			types2.Int8{},
			values.Int8(0),
		},
		{
			"Max",
			types2.Int8{},
			values.Int8(math.MaxInt8),
		},
	}...)
}

func TestEncodeInt16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types2.Int16{},
			values.Int16(math.MinInt16),
		},
		{
			"Zero",
			types2.Int16{},
			values.Int16(0),
		},
		{
			"Max",
			types2.Int16{},
			values.Int16(math.MaxInt16),
		},
	}...)
}

func TestEncodeInt32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types2.Int32{},
			values.Int32(math.MinInt32),
		},
		{
			"Zero",
			types2.Int32{},
			values.Int32(0),
		},
		{
			"Max",
			types2.Int32{},
			values.Int32(math.MaxInt32),
		},
	}...)
}

func TestEncodeInt64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Min",
			types2.Int64{},
			values.Int64(math.MinInt64),
		},
		{
			"Zero",
			types2.Int64{},
			values.Int64(0),
		},
		{
			"Max",
			types2.Int64{},
			values.Int64(math.MaxInt64),
		},
	}...)
}

func TestEncodeUint8(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types2.UInt8{},
			values.Uint8(0),
		},
		{
			"Max",
			types2.UInt8{},
			values.Uint8(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint16(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types2.UInt16{},
			values.Uint16(0),
		},
		{
			"Max",
			types2.UInt16{},
			values.Uint16(math.MaxUint8),
		},
	}...)
}

func TestEncodeUint32(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types2.UInt32{},
			values.Uint32(0),
		},
		{
			"Max",
			types2.UInt32{},
			values.Uint32(math.MaxUint32),
		},
	}...)
}

func TestEncodeUint64(t *testing.T) {
	testAllEncode(t, []encodeTest{
		{
			"Zero",
			types2.UInt64{},
			values.Uint64(0),
		},
		{
			"Max",
			types2.UInt64{},
			values.Uint64(math.MaxUint64),
		},
	}...)
}

func TestEncodeVariableSizedArray(t *testing.T) {
	emptyArray := encodeTest{
		"EmptyArray",
		types2.VariableSizedArray{
			ElementType: types2.Int{},
		},
		values.VariableSizedArray{},
	}

	intArray := encodeTest{
		"IntArray",
		types2.VariableSizedArray{
			ElementType: types2.Int{},
		},
		values.VariableSizedArray{
			values.NewInt(1),
			values.NewInt(2),
			values.NewInt(3),
		},
	}

	compositeArray := encodeTest{
		"CompositeArray",
		types2.VariableSizedArray{
			ElementType: types2.Composite{
				Fields: map[string]*types2.Field{
					"a": {
						Type:       types2.String{},
						Identifier: "a",
					},
					"b": {
						Type:       types2.Int{},
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
			types2.ConstantSizedArray{
				Size:        0,
				ElementType: types2.Int{},
			},
			values.ConstantSizedArray{},
		},
		{
			"IntArray",
			types2.ConstantSizedArray{
				Size:        3,
				ElementType: types2.Int{},
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
		types2.Dictionary{
			KeyType:     types2.String{},
			ElementType: types2.Int{},
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
		types2.Dictionary{
			KeyType: types2.String{},
			ElementType: types2.Dictionary{
				KeyType:     types2.String{},
				ElementType: types2.Int{},
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
		types2.Dictionary{
			KeyType: types2.String{},
			ElementType: types2.Composite{
				Fields: map[string]*types2.Field{
					"a": {
						Type: types2.String{},
					},
					"b": {
						Type: types2.Int{},
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
		types2.Composite{
			Fields: map[string]*types2.Field{
				"a": {
					Type: types2.String{},
				},
				"b": {
					Type: types2.String{},
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
		types2.Composite{
			Fields: map[string]*types2.Field{
				"a": {
					Type: types2.String{},
				},
				"b": {
					Type: types2.Int{},
				},
				"c": {
					Type: types2.Bool{},
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
		types2.Composite{
			Fields: map[string]*types2.Field{
				"a": {
					Type: types2.VariableSizedArray{
						ElementType: types2.Int{},
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
		types2.Composite{
			Fields: map[string]*types2.Field{

				"a": {
					Type: types2.String{},
				},
				"b": {
					Type: types2.Composite{
						Fields: map[string]*types2.Field{
							"a": {
								Type: types2.Int{},
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
		types2.Event{
			Identifier: "Test",
			Fields: []*types2.Parameter{
				{
					Field: types2.Field{
						Identifier: "x",
						Type:       types2.Int{},
					},
				},
				{
					Field: types2.Field{
						Identifier: "y",
						Type:       types2.String{},
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
		types2.Event{
			Identifier: "Test",
			Fields: []*types2.Parameter{
				{
					Field: types2.Field{
						Identifier: "x",
						Type:       types2.String{},
					},
				},
				{
					Field: types2.Field{
						Identifier: "y",
						Type: types2.Composite{
							Fields: map[string]*types2.Field{
								"a": {
									Type: types2.String{},
								},
								"b": {
									Type: types2.Int{},
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

func testEncode(t *testing.T, typ types2.Type, val values.Value) {
	b1, err := values2.Encode(val)
	require.NoError(t, err)

	t.Logf("Encoded value: %x", b1)

	// encoding should be deterministic, repeat to confirm
	for i := 0; i < numTrials; i++ {
		b2, err := values2.Encode(val)
		require.NoError(t, err)
		assert.Equal(t, b1, b2)
	}

	val2, err := values2.Decode(typ, b1)
	require.NoError(t, err)

	assert.Equal(t, val, val2)
}
