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
	testAllEncode(t, []encodeTest{
		{
			types.VariableSizedArray{
				ElementType: types.Int{},
			},
			values.VariableSizedArray{},
		},
		{
			types.VariableSizedArray{
				ElementType: types.Int{},
			},
			values.VariableSizedArray{
				values.Int(1),
				values.Int(2),
				values.Int(3),
			},
		},
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
	// TODO: add more cases
	testAllEncode(t, []encodeTest{
		{
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
		},
	})
}

func TestEncodeComposite(t *testing.T) {
	// TODO: add more cases
	testAllEncode(t, []encodeTest{
		{
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
		},
	})
}

func TestEncodeEvent(t *testing.T) {
	// TODO: add more cases
	testAllEncode(t, []encodeTest{
		{
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
		},
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
